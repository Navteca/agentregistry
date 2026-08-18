# Upstream Delta

This fork tracks upstream Solo.io AgentRegistry (0.4.0). To keep version bumps
cheap, **new behavior lives in new files**; existing upstream files are modified
only where a seam is unavoidable. Every modified upstream file is recorded here
with the reason and how to reapply the change after an upstream version bump.

Conventions:
- Additive only (new fields, new columns, new functions). No renames, no removed
  functionality, no altered semantics of existing fields.
- Upstream functionality is hidden behind configuration, never deleted.

---

## AR-1 — Role-aware OIDC permissions & artifact ownership

### `pkg/registry/auth/auth.go`
- **Change:** Added `Subject`, `DisplayName`, and typed `AuthMethod` fields to the
  `User` struct.
- **Reason:** Downstream handlers need the authenticated caller's identity for
  ownership recording and presentation. Upstream's `User` exposed only
  `Permissions`, so no handler could see *who* the caller was, regardless of auth
  method. `Subject` is read-only identity (used for ownership); `DisplayName` is
  presentation-only and must never be authorized against. `AuthMethod` lets the
  service distinguish anonymous authentication without inspecting request context
  in the database layer.
- **Reapply after bump:** Re-add the three fields to `User`: `Subject`,
  `DisplayName`, and `AuthMethod Method`. Purely additive; named struct literals
  elsewhere are unaffected.

### `pkg/registry/auth/jwt.go`
- **Change:** Added the `AuthMethodDisplayName` JWT claim
  (`json:"auth_method_name,omitempty"`); `jwtSession.Principal()` now populates
  `User.Subject` and `User.DisplayName` from the authenticated claims and copies
  `JWTClaims.AuthMethod` into `User.AuthMethod`. Added the
  `PermissionActionAdmin` sentinel and changed the global-permission denylist
  bypass to require `Action == PermissionActionAdmin` together with
  `ResourcePattern == "*"`. Added `PermissionActionEditOwn` and exposed the
  resource-pattern matcher through the package-level `HasPermission` helper so
  transactional owner checks reuse authorization matching without changing admin
  sentinel semantics.
- **Reason:** Carries the caller's stable subject, display-name snapshot, and
  authentication method on the registry JWT so they are available at artifact
  creation time via the session and so anonymous ownership can be omitted.
  Existing tokens without the display-name claim decode with an empty display
  name. The explicit admin sentinel prevents any non-admin wildcard permission
  (including legitimate publish/read/edit/delete/deploy grants) from silently
  bypassing the namespace denylist or all authorization checks. `edit_own` and
  `HasPermission` provide subject-scoped owner authorization without treating
  display names as identity.
- **Reapply after bump:** Re-add `AuthMethodDisplayName` and the identity
  assignments in `Principal()`, including `AuthMethod`. Re-add
  `PermissionActionAdmin`, `PermissionActionEditOwn`, and `HasPermission`;
  require both the admin action and wildcard pattern in `GenerateTokenResponse`'s
  global-permission predicate.

### `internal/registry/api/handlers/v0/auth/oidc.go`
- **Change:** Cache validated OIDC role-mapping configuration when constructing the
  handler and resolve verified OIDC claims into role-scoped permission bundles,
  retaining the existing static permission bundle as the unmatched-role fallback.
  `buildPermissions` also returns the resolved display name, falling back to the
  authenticated subject when the configured display-name claim is absent or empty;
  `ExchangeToken` stores that value in `AuthMethodDisplayName`.
- **Reason:** The registry must mint permissions based on the caller's mapped OIDC
  role while rejecting malformed role-map configuration at startup rather than
  silently falling back to an unbounded static bundle. The display-name snapshot
  must also reach the registry JWT for downstream ownership capture, including
  callers using the static permission fallback.
- **Reapply after bump:** Re-add the cached role-mapping configuration and the
  resolver call in `buildPermissions`, return the display name through
  `buildPermissions`, apply the subject fallback, and set
  `AuthMethodDisplayName` in the minted `JWTClaims`.

### `internal/registry/api/handlers/v0/auth/oidc_test.go`
- **Change:** Added token-exchange coverage for mapped, unknown, missing, and
  malformed OIDC role claims, including display-name propagation and subject
  fallback when the configured claim is absent or empty. Updated mapped-user
  token exchange expectations for the user-granted `edit_own` permission.
- **Reason:** Verifies role-based permission minting, preservation of the static
  fallback behavior, and that the new permission is carried into signed user
  tokens.
- **Reapply after bump:** Reapply the focused OIDC exchange tests and the
  `edit_own` expectation for the mapped-role exchange test.

### `charts/agentregistry/values.yaml`
- **Change:** Added Helm values for OIDC role claim mapping and role-scoped
  resource patterns.
- **Reason:** Makes role-based OIDC permission configuration available to Helm
  deployments.
- **Reapply after bump:** Re-add the six role-mapping values.

### `charts/agentregistry/templates/configmap.yaml`
- **Change:** Added ConfigMap environment-variable templating for OIDC role claim
  mapping and role-scoped resource patterns.
- **Reason:** Makes role-based OIDC permission configuration available to Helm
  deployments.
- **Reapply after bump:** Re-add the corresponding `AGENT_REGISTRY_OIDC_*`
  ConfigMap entries.

### `charts/agentregistry/templates/deployment.yaml`
- **Change:** Added an additional conditional `hostAliases` rendering block
  immediately before the deployment containers.
- **Reason:** Keeps chart-provided host aliases available at the pod-spec
  insertion point used by this fork's deployment template.
- **Reapply after bump:** Re-add the conditional `hostAliases` block immediately
  before `containers`.

### `Makefile`
- **Change:** Scoped Podman `--tls-verify=false` to local or loopback registries
  instead of applying it to every push. The Kind install target now disables
  anonymous authentication. Added opt-in local Keycloak targets for install,
  reset, and deletion, plus the Keycloak namespace setting.
- **Reason:** Prevents insecure TLS bypasses for remote registries, keeps local
  Kind setup aligned with mandatory authentication, and provides a reproducible
  local OIDC role-mapping workflow without making Keycloak part of the default
  setup.
- **Reapply after bump:** Re-add `PODMAN_TLS_VERIFY_FLAG` and use it for Podman
  image pushes, set `config.enableAnonymousAuth` to `"false"` in the Kind
  install, and restore `KEYCLOAK_NAMESPACE` plus the `install-keycloak`,
  `keycloak-reset`, and `delete-keycloak` targets.

### `internal/registry/config/config.go`
- **Change:** Added optional OIDC role-mapping configuration fields:
  `OIDCRoleClaimPath`, `OIDCDisplayNameClaimPath`, `OIDCRoleMap`,
  `OIDCUserPatterns`, `OIDCCuratorPatterns`, and `OIDCAdminPatterns`, including
  environment names, empty defaults, and documentation. An empty role-claim
  path disables role mapping and falls back to the static permission bundles;
  the display-name claim is presentation-only and never used for authorization.
- **Reason:** Makes role-scoped OIDC permissions and display-name propagation
  configurable while preserving the existing static-permission behavior by
  default.
- **Reapply after bump:** Re-add the six fields and their `AGENT_REGISTRY_OIDC_*`
  tags/defaults, including the documented empty-claim-path fallback.

### `internal/registry/config/config_test.go`
- **Change:** Added coverage that all six OIDC role-mapping environment variables
  default to empty and therefore leave role mapping disabled.
- **Reason:** Verifies the safe static-permission fallback and the documented
  defaults.
- **Reapply after bump:** Reapply `TestNewConfig_OIDCRolePatternsDefaultToEmpty`.

### `pkg/registry/auth/authz.go`
- **Change:** `PublicAuthzProvider.IsRegistryAdmin` now requires
  `permission.Action == PermissionActionAdmin && permission.ResourcePattern ==
  "*"`, instead of matching on `ResourcePattern == "*"` alone.
- **Reason:** `IsRegistryAdmin` gates `Check()`'s only bypass path, which
  currently skips every per-action `jwtManager.Check` call across ~50 call
  sites in `internal/registry/database/postgres.go`, as well as any future
  authorization check added inside `Check()` (e.g. a namespace denylist at
  request time). Under the old any-action-with-"*" rule, a deliberately scoped
  grant like `{Action: read, ResourcePattern: "*"}` ("read everything") or
  `{Action: delete, ResourcePattern: "*"}` ("delete everything") silently
  became a full, unbounded bypass of authorization for *all* actions - not
  just the one granted. This matters in this fork because artifact names have
  no publisher-derived namespace, so `"*"` is the only pattern that can
  express "all artifacts" for a curator/admin role's read/edit/delete/publish
  grants, and doing so must not implicitly grant everything else too.
- **Reapply after bump:** Re-apply the same two-part predicate
  (`Action == PermissionActionAdmin && ResourcePattern == "*"`) to
  `IsRegistryAdmin`.

### `pkg/registry/auth/authz_test.go`
- **Change:** Added coverage for the admin-only wildcard bypass, including
  rejection of nil sessions, non-admin wildcard permissions, scoped admin
  permissions, and mixed non-admin wildcards; also verifies that `PublicActions`
  remains empty and every action requires authentication for a nil session.
- **Reason:** Locks down action-aware admin authorization and the mandatory
  authentication behavior that prevents wildcard non-admin grants from becoming
  global bypasses.
- **Reapply after bump:** Reapply the focused `PublicAuthzProvider` and
  `PublicActions` authorization tests.

### `pkg/registry/auth/jwt_test.go`
- **Change:** Updated blocked-namespace fixtures to use the explicit admin
  sentinel, added regression coverage proving wildcard publish permissions no
  longer bypass the denylist, and added coverage that authenticated principals
  expose subject/display-name identity while absent display names remain empty.
- **Reason:** Verifies the action-aware global-permission predicate and the JWT
  identity propagation required for ownership capture.
- **Reapply after bump:** Reapply the blocked-namespace admin-sentinel and
  wildcard-publish regression tests, plus the principal identity propagation
  and empty-display-name tests.

### `internal/registry/database/testutil.go`
- **Change:** The test-only `testSession.Principal()` fixture now grants
  `{Action: PermissionActionAdmin, ResourcePattern: "*"}` instead of
  `{Action: PermissionActionEdit, ResourcePattern: "*"}`.
- **Reason:** This fixture exists to give integration tests a fully permissive
  session across every action (read/publish/edit/delete/deploy). Under the
  old semantics, `{Edit, "*"}` accidentally achieved that via the
  action-agnostic bypass; under the new action-aware predicate it would only
  grant `edit`. Using the `Admin` sentinel keeps the fixture's actual
  behavior (full bypass) explicit and correct.
- **Reapply after bump:** Re-apply the same fixture change if this file is
  regenerated from upstream.

### `pkg/models/server_response.go`
- **Change:** Added `Ownership *OwnershipMeta` and
  `Capabilities *CapabilitiesMeta` extension fields to server response metadata.
  `CapabilitiesMeta` is defined in fork-local `pkg/models/capabilities.go`.
- **Reason:** Lets responses expose registry-managed ownership metadata — the
  registering subject and a display-name snapshot — and caller-specific
  permitted actions, without changing the upstream artifact JSON payload.
- **Reapply after bump:** Re-add both metadata fields and preserve their
  `aregistry.ai/ownership` and `aregistry.ai/capabilities` JSON keys.

### `pkg/models/agent.go`
- **Change:** Added `Ownership *OwnershipMeta` and
  `Capabilities *CapabilitiesMeta` extension fields to agent response metadata.
  `CapabilitiesMeta` is defined in fork-local `pkg/models/capabilities.go`.
- **Reason:** Lets responses expose registry-managed ownership metadata — the
  registering subject and a display-name snapshot — and caller-specific
  permitted actions, without changing the upstream artifact JSON payload.
- **Reapply after bump:** Re-add both metadata fields and preserve their
  `aregistry.ai/ownership` and `aregistry.ai/capabilities` JSON keys.

### `pkg/models/skill.go`
- **Change:** Added `Ownership *OwnershipMeta` and
  `Capabilities *CapabilitiesMeta` extension fields to skill response metadata.
  `CapabilitiesMeta` is defined in fork-local `pkg/models/capabilities.go`.
- **Reason:** Lets responses expose registry-managed ownership metadata — the
  registering subject and a display-name snapshot — and caller-specific
  permitted actions, without changing the upstream artifact JSON payload.
- **Reapply after bump:** Re-add both metadata fields and preserve their
  `aregistry.ai/ownership` and `aregistry.ai/capabilities` JSON keys.

### `pkg/models/prompt.go`
- **Change:** Added `Ownership *OwnershipMeta` and
  `Capabilities *CapabilitiesMeta` extension fields to prompt response metadata.
  `CapabilitiesMeta` is defined in fork-local `pkg/models/capabilities.go`.
- **Reason:** Lets responses expose registry-managed ownership metadata — the
  registering subject and a display-name snapshot — and caller-specific
  permitted actions, without changing the upstream artifact JSON payload.
- **Reapply after bump:** Re-add both metadata fields and preserve their
  `aregistry.ai/ownership` and `aregistry.ai/capabilities` JSON keys.

### `internal/registry/database/postgres.go`
- **Change:** Server query and mutation methods now return fork-local
  `models.ServerResponse` values, with semantic search scores assigned directly
  to `ServerResponse.Meta.Semantic` instead of round-tripping through
  publisher-provided metadata. Server update and status mutations accept either
  `edit` or `edit_own` at the coarse database authorization gate. Server,
  agent, skill, and prompt inserts persist nullable subject, display-name, and
  auth-method columns; all five read paths for each artifact scan nullable
  ownership columns and populate the corresponding response metadata. Anonymous
  and empty-subject server inputs write all three ownership columns as `NULL`;
  named-column-only update, status-update, and latest-version statements remain
  unchanged.
- **Reason:** Keeps database responses consistent with fork-local response
  models and preserves public JSON while retaining semantic metadata. The
  database supplies only the coarse edit-authority gate; owner and review
  narrowing belongs to the service transaction. Ownership is captured without
  trusting request payload metadata or display names, preserved through the
  supported artifact mutations, and omitted for legacy or unowned rows.
- **Reapply after bump:** Update all eight server response return types and
  constructors to `models.ServerResponse`; assign semantic scores to
  `Meta.Semantic`. Allow both `edit` and `edit_own` at the shared mutation gate.
  Add nullable ownership columns and `sql.NullString` scan destinations to all
  five server, agent, skill, and prompt read paths; pass `OwnershipInput` to
  each create method; preserve the named-column-only mutation statements.

### `pkg/registry/database/database.go`
- **Change:** Updated the eight server response method signatures to return
  `models.ServerResponse` and added `models.OwnershipInput` as the final
  parameter to `CreateServer`, `CreateAgent`, `CreateSkill`, and `CreatePrompt`.
- **Reason:** Keeps the interface aligned with the PostgreSQL implementation and
  makes ownership data flow explicit at the persistence boundary.
- **Reapply after bump:** Replace the eight upstream server response return types
  with the fork-local model type and append `models.OwnershipInput` to all four
  artifact create methods.

### `internal/registry/service/registry_service.go`
- **Change:** Propagated `models.ServerResponse` through server service methods
  and transaction callbacks. Added transactional owner-scoped server update
  authorization that compares stable subjects and rejects unowned or reviewed
  artifacts for `edit_own`; the `IsSystemSession` branch retains the internal
  system bypass, and the `isArtifactReviewed` AR-2 stub remains the review
  predicate insertion point. Resolves ownership once per server, agent, skill,
  and prompt create operation and passes `models.OwnershipInput` through each
  transaction to the database. The service constructor now requires and stores
  the existing by-value `auth.Authorizer` from the composition root.
- **Reason:** Carries fork-local response metadata to API consumers without
  converting back to upstream types. Owner authorization must use the row
  fetched in the same transaction as the write; display names are never
  authorization inputs. Resolving ownership in the service keeps authentication
  context out of the database and prevents request metadata from overriding the
  authenticated creator. Capability flags reuse the existing update
  authorization predicate and the configured authorizer's delete check without
  changing enforcement. Missing sessions produce all-false capabilities, and a
  nil authorizer provider cannot fail open for deletion.
- **Additional AR-1B change:** Added shared delete-capability computation and
  per-type capability annotation to all agent, skill, and prompt list, latest,
  version-specific, and all-version reads. Annotates successful server update
  responses after the transaction commits, carrying forward the fetched
  ownership metadata so owner-scoped update capabilities match a subsequent
  read.
- **Reapply after bump:** Update server response and transaction callback types
  to `models.ServerResponse`. Add the owner narrowing check after the
  transactional current-server fetch and retain the review predicate insertion
  point. Resolve ownership before each create transaction, pass it through the
  corresponding create callback, append it to all four database create calls,
  and require the composition-root `auth.Authorizer` in the service
  constructor. Annotate every server read response with caller-specific
  `can_update` and `can_delete` capabilities after database retrieval. Apply
  capability annotation as a service-layer post-pass to list, latest,
  version-specific, and all-version server reads.

### `internal/registry/registry_app.go`
- **Change:** Passes the existing composition-root `auth.Authorizer` into the
  registry service.
- **Reason:** Service-layer capability computation must use the same authorizer
  and provider configuration as database mutation enforcement.
- **Reapply after bump:** Pass the constructed `auth.Authorizer` to the
  registry service constructor.

### `internal/registry/service/service.go`
- **Change:** Updated the server methods on `RegistryService` to return fork-local response models.
- **Reason:** Exposes the migrated response type at the service boundary.
- **Reapply after bump:** Update the six server response signatures in the interface to use `models.ServerResponse`.

### `internal/registry/api/handlers/v0/servers.go`
- **Change:** Retained `normalizeServerResponse` as a nil-safe pass-through for `models.ServerResponse`; removed semantic score extraction from publisher-provided metadata.
- **Reason:** The database now supplies fork-local metadata directly, so handler conversion is no longer needed while existing handler call sites remain stable.
- **Reapply after bump:** Change the normalizer input to the fork-local response and return its value unchanged, preserving the nil guard.

### AR-1 — `internal/registry/api/handlers/v0/auth/oidc_roles.go`
- **Change:** Added fork-local `edit_own` support to the user permission bundle
  and updated the bundle documentation. This new file is deliberately structured
  to mirror `github_at.go` so it can be upstreamed later.
- **Reason:** Users need a distinct permission for owner-scoped edits without
  receiving curator-level `edit`, while keeping the fork-local role-bundle
  implementation easy to upstream.

### AR-1 — `internal/registry/api/handlers/v0/edit.go`
- **Change:** Removed the admin-only wording from the server edit endpoint
  description.
- **Reason:** Unreviewed owner edits are now supported.
- **Reapply after bump:** Update the endpoint description to describe general
  server updates.

### AR-1 — `internal/registry/api/handlers/v0/edit_test.go`
- **Change:** Authenticated setup contexts are used for all server fixture
  creation.
- **Reason:** The endpoint tests exercise authenticated edit behavior and must
  not rely on the removed sessionless public actions.
- **Reapply after bump:** Use the existing `database.WithTestSession` helper
  for setup creates.

### `internal/registry/api/handlers/v0/scoring.go`
- **Change:** Updated the server scoring persistence seam to accept `models.ServerResponse`.
- **Reason:** The scoring endpoint consumes the migrated service response type.
- **Reapply after bump:** Use `*models.ServerResponse` for the server argument passed to scoring persistence.

### `internal/mcp/registryserver/server.go`
- **Change:** Server discovery MCP tools now return `models.ServerListResponse` so fork-local response metadata is retained.
- **Reason:** Avoids converting the migrated service response back to the upstream type and preserves semantic metadata in the serialized output.
- **Reapply after bump:** Use the fork-local server list/response models for the two server discovery tools and `fetchSingleServer`.

### `internal/registry/service/testing/fake_registry.go`
- **Change:** Kept existing upstream-shaped test setup hooks and added conversion at the fake service boundary to return `models.ServerResponse`.
- **Reason:** Preserves existing test fixtures while allowing the fake to implement the migrated service interface.
- **Reapply after bump:** Keep the fixture compatibility conversion if the service response boundary is regenerated.

### `internal/registry/importer/importer_test.go`
- **Change:** Explicitly converts the migrated database response to the upstream response shape used by the importer fixture. Service construction now passes an explicit permissive test authorizer.
- **Reason:** The importer fixture tests the upstream export payload and does not consume fork-local response metadata. The required service constructor authorizer keeps test wiring explicit.
- **Reapply after bump:** Preserve the explicit `Server`/official-meta conversion at the upstream fixture boundary and pass an explicit test authorizer when constructing the service.

### `internal/cli/export.go`
- **Change:** Passes the existing command authorizer into the registry service constructor.
- **Reason:** The service constructor now requires its authorizer explicitly.
- **Reapply after bump:** Pass the existing command authorizer to `NewRegistryService`.

### `internal/cli/import.go`
- **Change:** Passes the existing command authorizer into the registry service constructor.
- **Reason:** The service constructor now requires its authorizer explicitly.
- **Reapply after bump:** Pass the existing command authorizer to `NewRegistryService`.

### `internal/mcp/registryserver/server_integration_test.go`
- **Change:** Passes an explicit permissive test authorizer into the registry service constructor.
- **Reason:** The service constructor now requires its authorizer explicitly.
- **Reapply after bump:** Preserve explicit test authorization wiring.

### `internal/registry/service/registry_service_test.go`
- **Change:** Added a shared permissive test-authorizer fixture and passed it to every registry service construction.
- **Reason:** Keeps service test construction explicit after making the authorizer constructor parameter required.
- **Reapply after bump:** Preserve the explicit authorizer fixture at every service construction.

### `internal/registry/service/server_capabilities_test.go`
- **Change:** Added service-layer coverage for caller-specific server update and
  delete capabilities across every server read path, including missing-session
  and nil-authorizer cases.
- **Reason:** Proves capability flags reflect the existing authorization rules
  without exercising HTTP handlers or weakening enforcement.
- **Reapply after bump:** Preserve the real-JWT authorization fixtures and
  permission-removal cases when reapplying server capability computation.

### `internal/registry/service/artifact_capabilities_test.go`
- **Change:** Added real-JWT service coverage for delete capabilities across
  every agent, skill, and prompt read path, including owner-scoped edit
  permissions and sessionless all-false behavior. Added coverage that server
  update responses match the capabilities of a subsequent read.
- **Reason:** Proves the four artifact types expose consistent caller-specific
  capability metadata without changing server-side authorization enforcement.
- **Reapply after bump:** Preserve the real capability authorizer, ownership
  contexts, all read-path cases, and post-update/read comparison.

### `internal/registry/service/ownership_test.go`
- **Change:** Passes the shared service test authorizer into every registry service construction.
- **Reason:** Keeps service test construction explicit after making the authorizer constructor parameter required.
- **Reapply after bump:** Preserve explicit authorizer wiring.

### `internal/registry/api/cors_test.go`
- **Change:** Passes an explicit permissive test authorizer into each registry service construction.
- **Reason:** Keeps API test construction explicit after making the authorizer constructor parameter required.
- **Reapply after bump:** Preserve explicit authorizer wiring.

### `internal/registry/api/handlers/v0/edit_test.go`
- **Change:** Passes an explicit permissive test authorizer into each registry service construction.
- **Reason:** Keeps handler test construction explicit after making the authorizer constructor parameter required.
- **Reapply after bump:** Preserve explicit authorizer wiring.

### `internal/registry/api/handlers/v0/servers_test.go`
- **Change:** Passes an explicit permissive test authorizer into every registry service construction.
- **Reason:** Keeps handler test construction explicit after making the authorizer constructor parameter required.
- **Reapply after bump:** Preserve explicit authorizer wiring.

### `internal/registry/api/handlers/v0/telemetry_test.go`
- **Change:** Passes an explicit permissive test authorizer into the registry service construction.
- **Reason:** Keeps handler test construction explicit after making the authorizer constructor parameter required.
- **Reapply after bump:** Preserve explicit authorizer wiring.

### Interlude — pre-existing test fixture repairs
- **Files:** `internal/registry/service/registry_service_test.go`,
  `internal/registry/api/handlers/v0/servers_test.go`,
  `internal/registry/api/handlers/v0/edit_test.go`,
  `internal/registry/api/handlers/v0/telemetry_test.go`,
  `internal/registry/importer/importer_test.go`, and
  `internal/mcp/registryserver/server_integration_test.go`.
- **Change:** Completed the three pre-existing fixture defect variants caused
  by fixtures written against a populated `PublicActions` map, after commit
  `0f6e5d2` emptied it: server fixtures now provide the deliberately
  non-existent `https://github.com/fixture-owner/fixture-repository`
  repository; server-creation setup calls use `database.WithTestSession`; and
  hand-built edit tokens include the explicit `read` permission required before
  the edit. Protected HTTP requests and service reads in the handler fixtures
  also carry the test session, and edit request bodies include the required
  repository fixture. The three unsafe `assert` checks were changed to fatal guards:
  `assert.Len` to `require.Len` in the local-file and HTTP-file importer tests,
  and `assert.Equal` to `require.Equal` for the edit status response.
- **Reason:** The defects produced missing-repository errors (which masked the
  other failures), unauthenticated setup errors, and a forbidden response from
  the preliminary read when the edit token lacked `read`. The non-fatal
  assertions allowed invalid responses to be indexed or dereferenced and abort
  the package. The initial fixture repairs did not change reachability
  validation settings.
- **Reapply after bump:** Preserve the offline repository fixture, authenticated
  setup and request contexts, explicit read-plus-edit test token permissions,
  and fatal response-length/status guards when reapplying the AR-1 test fixes.
- **Observation:** `resolveOwnership` in
  `internal/registry/service/ownership.go:10` excludes only `auth.MethodNone`,
  so a session with a zero-value auth method passes through it. The
  `ownership.Subject != ""` guard in `internal/registry/database/postgres.go:506`
  is what prevents a bad ownership row; that guard is correctly placed, while
  the service-layer method check is narrower than it reads.
- **Follow-up:** Disabled repository reachability validation in
  `TestListServersSemanticSearch` because its fixture URL is intentionally
  unreachable and repository-URL validation issues an outbound request to a
  publisher-supplied URL at registration time. That pre-existing SSRF surface
  remains on the findings list and is not addressed here.

### AR-1 — `ui/components/server-detail.tsx`
- **Change:** Scheme-allowlisted artifact-supplied website, repository, remote, and icon URLs before rendering them as links or images; rejected link values remain visible as text. The server detail quick-info pills also show registered-by ownership and optional last-modified metadata.
- **Reason:** Closes the AR-1 criterion on URL scheme allowlisting. The pre-existing form validation was bypassable when artifacts were published directly through the API. Ownership and modification metadata are displayed from typed response metadata without using presentation names as identity.
- **Reapply after bump:** Retain the `getSafeHttpUrl` validation at every server detail URL and icon rendering site, and render ownership from `aregistry.ai/ownership` plus `official.updatedAt` in the quick-info pills.

### AR-1 — `ui/components/server-card.tsx`
- **Change:** Scheme-allowlisted the artifact-supplied icon `src`, repository
  URL, and website URL with `getSafeHttpUrl`; unsafe images and actions are not
  rendered. The card also shows registered-by metadata from
  `aregistry.ai/ownership`, preferring `displayName` and falling back to
  `subject`, with `Unknown` for unowned artifacts.
- **Reason:** Closes the AR-1 URL scheme bypass in catalog cards, including
  `window.open` call sites that could execute a `javascript:` URL when artifacts
  were published directly through the API. Ownership is presentation metadata,
  not an identity input. Last-modified remains on the detail view because adding
  it to dense catalog rows would make their metadata unscannable; this is a
  deliberate deviation from the AR-1 ticket wording.
- **Reapply after bump:** Retain `getSafeHttpUrl` at the icon, repository, and
  website rendering/action sites. Render registered-by after published date
  using the `displayName`/`subject` fallback and `Unknown` placeholder; keep
  last-modified exclusive to the detail view.

## AR-1B — Current principal endpoint

### `internal/registry/api/handlers/v0/auth/main.go`
- **Change:** Registers the authenticated current-principal endpoint alongside
  the existing authentication endpoints.
- **Reason:** Exposes the caller's subject, display name, and authentication
  method to the frontend without returning permissions or deriving a role.
- **Reapply after bump:** Register `RegisterCurrentPrincipalEndpoint` with the
  existing authentication route registrations.

### `openapi.yaml`
- **Change:** Regenerated the API specification to include `GET /v0/auth/me`
  and its identity response schema.
- **Reason:** Keeps the checked-in API contract synchronized with the endpoint.
- **Reapply after bump:** Regenerate the OpenAPI specification after registering
  the current-principal endpoint.

### `ui/lib/api/index.ts`
- **Change:** Regenerated the TypeScript client barrel exports to include the
  current-principal operation and response types.
- **Reason:** Keeps the checked-in frontend client synchronized with the API
  contract.
- **Reapply after bump:** Run `make gen-client` after regenerating the OpenAPI
  specification.

### `ui/lib/api/sdk.gen.ts`
- **Change:** Regenerated the TypeScript client operation for
  `GET /v0/auth/me`.
- **Reason:** Keeps the checked-in frontend client synchronized with the API
  contract.
- **Reapply after bump:** Run `make gen-client` after regenerating the OpenAPI
  specification.

### `ui/lib/api/types.gen.ts`
- **Change:** Regenerated the TypeScript response and operation types for
  `GET /v0/auth/me`.
- **Reason:** Keeps the checked-in frontend client synchronized with the API
  contract.
- **Reapply after bump:** Run `make gen-client` after regenerating the OpenAPI
  specification.

### AR-1 — `ui/components/agent-card.tsx`
- **Change:** Scheme-allowlisted the artifact-supplied repository `href` with
  `getSafeHttpUrl`; rejected URLs remain visible as text without an anchor. The
  card also shows registered-by metadata from `aregistry.ai/ownership`,
  preferring `displayName` and falling back to `subject`, with `Unknown` for
  unowned artifacts.
- **Reason:** Closes the AR-1 URL scheme bypass when artifacts are published
  directly through the API. Ownership is presentation metadata, not an identity
  input. Last-modified remains on the detail view because adding it to dense
  catalog rows would make their metadata unscannable; this is a deliberate
  deviation from the AR-1 ticket wording.
- **Reapply after bump:** Retain `getSafeHttpUrl` at the repository link and
  keep rejected URLs text-only. Render registered-by after published date using
  the `displayName`/`subject` fallback and `Unknown` placeholder; keep
  last-modified exclusive to the detail view.

### AR-1 — `ui/components/skill-card.tsx`
- **Change:** Scheme-allowlisted artifact-supplied repository and website URLs
  passed to `window.open` with `getSafeHttpUrl`; rejected actions are not
  rendered and do not open a blank tab. The card also shows registered-by
  metadata from `aregistry.ai/ownership`, preferring `displayName` and falling
  back to `subject`, with `Unknown` for unowned artifacts.
- **Reason:** Closes the AR-1 URL scheme bypass in catalog actions, including the
  pre-existing `window.open(url, '_blank')` behavior that could execute a
  `javascript:` URL when artifacts were published directly through the API.
  Ownership is presentation metadata, not an identity input. Last-modified
  remains on the detail view because adding it to dense catalog rows would make
  their metadata unscannable; this is a deliberate deviation from the AR-1
  ticket wording.
- **Reapply after bump:** Retain `getSafeHttpUrl` at both `window.open` sites,
  including the no-op behavior for rejected URLs. Render registered-by after
  published date using the `displayName`/`subject` fallback and `Unknown`
  placeholder; keep last-modified exclusive to the detail view.

### AR-1 — `ui/components/prompt-card.tsx`
- **Change:** The card shows registered-by metadata from
  `aregistry.ai/ownership`, preferring `displayName` and falling back to
  `subject`, with `Unknown` for unowned artifacts. No URL allowlisting was
  added because prompt cards have no artifact-supplied URLs. Updated-at is not
  shown on cards.
- **Reason:** Adds ownership context without treating the presentation
  `displayName` as identity. Last-modified remains on the detail view because
  adding it to dense catalog rows would make their metadata unscannable; this is
  a deliberate deviation from the AR-1 ticket wording.
- **Reapply after bump:** Render registered-by after published date using the
  `displayName`/`subject` fallback and `Unknown` placeholder; keep
  last-modified exclusive to the detail view and do not add URL handling unless
  prompt cards gain artifact-supplied URLs.

## AR-1B Task 6 — Conditional catalog controls

### `ui/lib/capabilities.ts`
- **Change:** Added the exported strict `capabilityFlags` mapper, which turns
  the optional response capability block into the three existing catalog-card
  control flags and hides controls unless each field is exactly `true`.
- **Reason:** Centralizes the response-to-control mapping so page and component
  tests exercise the same rule without duplicating it.
- **Reapply after bump:** Preserve the strict `=== true` mapping for update,
  delete, and deploy.

### `ui/app/page.tsx`
- **Change:** Server card Edit, Remove, and Deploy props, plus agent card
  Deploy props, now use the shared strict `capabilityFlags` mapper.
- **Reason:** Makes catalog controls reflect backend-provided caller
  capabilities without deriving permissions client-side. Existing deploy
  artifact-content checks and disabled-state messages remain in the cards.
- **Reapply after bump:** Gate only the existing server and agent card props
  with `capabilities?.can_update === true`,
  `capabilities?.can_delete === true`, and
  `capabilities?.can_deploy === true`; do not wire dormant skill controls.

### `ui/components/__tests__/server-card.test.tsx`
- **Change:** Added coverage for all capability combinations, absent metadata,
  permitted deploy without an OCI package, and denied deploy without an OCI
  package.
- **Reason:** Verifies permission absence hides Deploy while permitted but
  content-ineligible artifacts retain the existing disabled Deploy affordance.
- **Reapply after bump:** Preserve the explicit capability and OCI
  permission/content state assertions.

### `ui/components/__tests__/agent-card.test.tsx`
- **Change:** Added coverage that agent Deploy renders only when
  `can_deploy` is true.
- **Reason:** Verifies agent deployment controls use the response capability.
- **Reapply after bump:** Preserve the true/false deploy capability cases.

### `ui/app/__tests__/page-edit-flow.test.tsx`
- **Change:** Updated the page card mock to honor control props and added
  coverage for forwarding all server capability flags and hiding controls when
  the capability block is absent. The test uses the shared mapper for expected
  flags.
- **Reason:** Verifies page-level wiring, including the required unknown-means
  hidden behavior.
- **Reapply after bump:** Keep the mock prop behavior and page wiring tests
  synchronized with catalog capability gating.

### `ui/lib/__tests__/capabilities.test.ts`
- **Change:** Added direct coverage for all-true, all-false, mixed, absent,
  empty, and partially populated capability metadata.
- **Reason:** Locks down strict explicit-true behavior at the single mapping
  seam.
- **Reapply after bump:** Preserve the direct mapper cases, especially missing
  fields.

## AR-1B Task 5B — Deploy capability

### `pkg/models/capabilities.go`
- **Change:** Added the required `CanDeploy` field to `CapabilitiesMeta`,
  serialized as `can_deploy`.
- **Reason:** Exposes the caller's existing deploy authorization as a
  response capability without changing deployment enforcement.
- **Reapply after bump:** Add `CanDeploy bool \`json:"can_deploy"\`` alongside
  `CanUpdate` and `CanDelete`.

### `pkg/models/capabilities_test.go`
- **Change:** Extended capability JSON coverage to include `can_deploy`.
- **Reason:** Verifies the new required response field and its wire format.
- **Reapply after bump:** Preserve the `can_deploy` serialization assertion.

### `internal/registry/service/registry_service.go`
- **Change:** Generalized delete capability authorization into `canPerform`,
  which retains the no-session and nil-authorizer fail-closed guards and accepts
  an authorization action. Server and agent responses now compute `CanDeploy`
  with `PermissionActionDeploy`; skills and prompts explicitly set it to false
  because they have no deployment endpoint.
- **Reason:** Keeps capability computation aligned with the existing deployment
  enforcement in the database while keeping artifact deployability
  preconditions separate from permission.
- **Reapply after bump:** Replace delete-only capability checks with the shared
  action predicate, compute deploy for server and agent artifact names, and
  preserve explicit false values for skills and prompts.

### `internal/registry/service/server_capabilities_test.go`
- **Change:** Added real-JWT coverage for deploy-authorized and
  non-deploy-authorized server principals, including no-session and
  nil-authorizer fail-closed cases.
- **Reason:** Verifies server `CanDeploy` matches `PermissionActionDeploy`.
- **Reapply after bump:** Preserve deploy and fail-closed capability cases.

### `internal/registry/service/artifact_capabilities_test.go`
- **Change:** Added real-JWT deploy capability coverage across all agent,
  skill, and prompt read paths, asserting deploy is available only for agents
  with the permission and remains false for skills and prompts.
- **Reason:** Verifies per-type behavior and the response consistency of every
  read path.
- **Reapply after bump:** Preserve deploy permission, no-deploy permission,
  and per-artifact-type assertions.

### `openapi.yaml`
- **Change:** Regenerated the schema so `CapabilitiesMeta` includes required
  `can_deploy`.
- **Reason:** Keeps the checked-in API contract synchronized with the model.
- **Reapply after bump:** Regenerate the OpenAPI specification after adding
  `CanDeploy`.

### `ui/lib/api/types.gen.ts`
- **Change:** Regenerated the TypeScript client type so `CapabilitiesMeta`
  exposes required `can_deploy: boolean`.
- **Reason:** Keeps frontend consumers synchronized with the API response shape.
- **Reapply after bump:** Run `make gen-client` after regenerating OpenAPI.

## AR-2 Task 1 — Typed reviews migration

### `internal/registry/database/migrations/013_reviews_table.sql`
- **Change:** Added the fork-owned append-only `reviews` table for typed,
  outcome-bearing reviews attached to a specific artifact version. Added a
  surrogate primary key for deterministic current-review ordering and indexes
  for reviewer history and current-review/artifact-state resolution.
- **Reason:** AR-2 needs a single cross-artifact review store. The migration is
  additive and intentionally has no foreign keys because artifact identity spans
  four existing tables; orphaned reviews after deletion are accepted. Review
  types and outcomes remain plain strings and are validated by configuration and
  the API rather than duplicated as schema constraints. Reviewer identity fields
  remain `TEXT`, matching the ownership columns added by AR-1.
- **Reapply after bump:** Preserve migration version `013` in the fork's base
  migration namespace. Migrations `011`, `012`, and `013` are fork-owned; the
  upstream `origin/main` tree has no `internal/registry/database/migrations/`
  directory. Keep the artifact-type structural check, append-only timestamp,
  reviewer identity snapshots, and the two query-supporting indexes. If
  upstream later adds migrations in this namespace, choose the next unused base
  version without renumbering the fork-owned migrations.

## AR-2 Task 2 — Configured review vocabulary

### `.env.example`
- **Change:** Documented the review type and outcome environment variables and
  their shipped defaults, including the configured failure outcome.
- **Reason:** Makes the deployment configuration surface discoverable without
  changing the defaults or validation behavior.
- **Reapply after bump:** Preserve the `REVIEW_TYPES`, `REVIEW_OUTCOMES`, and
  `REVIEW_FAILURE_OUTCOME` examples alongside the other application settings.

### `internal/registry/config/config.go`
- **Change:** Added environment-configured review type and outcome lists with
  defaults of `security,scientific` and `pass,fail`. Added in-place
  normalization during validation, an explicit configured failure outcome, and
  the single `ReviewConfig()` accessor with ordered list and membership methods.
- **Reason:** Makes the review vocabulary deployment-configurable while keeping
  downstream consumers off the raw configuration slices. The service already
  receives `*config.Config` at construction, so the accessor is available to
  certification and API logic.
- **Reapply after bump:** Preserve the `REVIEW_TYPES`, `REVIEW_OUTCOMES`, and
  `REVIEW_FAILURE_OUTCOME` fields, defaults, and `ReviewConfig()` accessor.

### `internal/registry/config/validate.go`
- **Change:** Added startup validation for non-empty, unique review types and
  outcomes. Entries are trimmed and must match the ASCII identifier pattern
  `[A-Za-z][A-Za-z0-9_-]*`; the configured failure outcome must be one of the
  configured outcomes.
- **Reason:** Rejects malformed review vocabulary during the existing
  fail-fast configuration validation path rather than at request time.
  The pattern deliberately excludes dots and slashes so configured values stay safe to use as JSON keys and URL segments, which rules out namespaced names like nasa.export-control.
- **Reapply after bump:** Preserve first-error validation behavior, the
  identifier allowlist, and failure-outcome membership validation.

### `internal/registry/config/config_test.go`
- **Change:** Added coverage for default and configured ordering, accessor
  membership, explicit failure-outcome selection, in-place normalization, empty
  lists, duplicates, whitespace-only values, and malformed entries.
- **Reason:** Locks validation to startup configuration behavior and ensures
  invalid vocabularies cannot reach downstream review logic.
- **Reapply after bump:** Preserve the default, ordering, membership, and
  invalid-input cases.

## AR-2 Task 3 — Create-review endpoint and permission

### `pkg/registry/auth/jwt.go`
- **Change:** Added `PermissionActionReview` with the existing permission
  vocabulary.
- **Reason:** Review creation needs a distinct authorization action rather than
  reusing edit or publish permissions.
- **Reapply after bump:** Add the `review` action constant alongside the other
  permission actions.

### `internal/registry/api/handlers/v0/auth/oidc_roles.go`
- **Change:** Added review permission to curator and admin role bundles while
  leaving the user bundle unchanged.
- **Reason:** Curators and admins may review; users may not. This follows the
  AR-1 role-bundle source of truth.
- **Reapply after bump:** Preserve review in curator/admin actions and absent
  from the user action list.

### `internal/registry/api/handlers/v0/auth/oidc_roles_test.go`
- **Change:** Updated curator/admin role-bundle expectations for the review
  action.
- **Reason:** Keeps role registration tests aligned with the new permission.
- **Reapply after bump:** Preserve explicit curator/admin review assertions and
  the user bundle's absence of review.

### `pkg/models/review.go`
- **Change:** Added the review response model containing the database identity,
  artifact identity, review content, reviewer snapshot, and creation timestamp.
- **Reason:** Provides one typed shape for service, database, and API responses.
- **Reapply after bump:** Preserve the JSON field names and append-only review
  fields.

### `pkg/registry/database/database.go`
- **Change:** Added `CreateReview` to the database interface.
- **Reason:** Keeps review persistence behind the repository boundary.
- **Reapply after bump:** Retain the transactional append-only insert contract.

### `internal/registry/database/postgres.go`
- **Change:** Implemented `CreateReview` as an insert returning the complete
  review row, with review permission enforcement and no artifact update.
- **Reason:** Persists reviewer identity supplied by the service and leaves
  artifact `updated_at` untouched.
- **Reapply after bump:** Preserve the authorization check, `INSERT ... RETURNING`
  shape, and absence of artifact mutation.

### `internal/registry/service/service.go`
- **Change:** Added `CreateReview` to the registry service interface.
- **Reason:** Exposes review creation to API transport without exposing database
  details.
- **Reapply after bump:** Preserve the service-level review creation contract.

### `internal/registry/service/reviews.go`
- **Change:** Added service orchestration for configured-value validation,
  `HasPermission`-backed review authorization, artifact-version existence
  checks, token-derived reviewer identity, and transactional persistence.
- **Reason:** Keeps business rules and server-assigned identity out of the
  handler and prevents typo-created orphan reviews. The service validation is
  authoritative for all callers, including non-HTTP callers; the handler repeats
  it only to return an earlier clean 400 response.
- **Reapply after bump:** Preserve permission-before-write ordering, all four
  artifact existence checks, and identity derivation from the authenticated
  session.

### `internal/registry/service/testing/fake_registry.go`
- **Change:** Added a configurable fake hook for `CreateReview`.
- **Reason:** Allows endpoint tests to isolate boundary validation and identity
  handling without a database.
- **Reapply after bump:** Keep the fake implementation synchronized with the
  service interface.

### `internal/registry/service/reviews_test.go`
- **Change:** Added real-PostgreSQL and real-JWT-authorizer coverage for
  identity snapshots, independent curators, append-only revisions, permission
  refusal, configured-value checks, artifact existence, and unchanged
  `updated_at`.
- **Reason:** Verifies review persistence and authorization without router or
  IdP dependencies.
- **Reapply after bump:** Preserve the real-subject and timestamp assertions.

### `internal/registry/api/handlers/v0/reviews.go`
- **Change:** Added `POST /v0/reviews/{artifactType}/{artifactName}/versions/{version}`.
  The handler validates configured type/outcome values before calling the
  service and accepts but ignores client-supplied reviewer identity fields.
- **Reason:** The handler-side validation is a transport-level fast path; the
  service remains authoritative. A reviews namespace supports all four artifact types without
  duplicating four artifact-specific routes; artifact names retain the existing
  URL-encoded path convention.
- **Reapply after bump:** Preserve the POST-only route, boundary validation,
  error mapping, and token-owned identity behavior.

### `internal/registry/api/handlers/v0/reviews_test.go`
- **Change:** Added endpoint tests for boundary rejection, ignored payload
  identity fields, and user permission refusal.
- **Reason:** Locks down transport-level status codes and the no-write boundary.
- **Reapply after bump:** Preserve the invalid-config 400 and forbidden 403
  cases.

### `internal/registry/api/router/v0.go`
- **Change:** Registered the reviews endpoint in the v0 route set.
- **Reason:** Wires the new API surface into the application router.
- **Reapply after bump:** Preserve `RegisterReviewsEndpoint` registration.

### `openapi.yaml`
- **Change:** Regenerated the API contract for the create-review operation and
  review schemas.
- **Reason:** Keeps the checked-in OpenAPI specification synchronized.
- **Reapply after bump:** Regenerate after API changes.

### `ui/lib/api/index.ts`
### `ui/lib/api/sdk.gen.ts`
### `ui/lib/api/types.gen.ts`
- **Change:** Regenerated the TypeScript client exports, operation, and review
  types.
- **Reason:** Keeps generated frontend API consumers synchronized with OpenAPI.
- **Reapply after bump:** Run `make gen-client`.

## AR-2 Task 4 — Current-review resolution and certification

### `pkg/models/review.go`
- **Change:** Added per-type and overall derived review-state models with
  stable pending, pass, and fail statuses plus the raw per-type outcome.
- **Reason:** Gives later artifact responses and review endpoints a shared
  representation of current reviews and certification state.
- **Reapply after bump:** Preserve the derived-state fields and status values.

### `pkg/registry/database/database.go`
- **Change:** Added `ListReviews` for one artifact version.
- **Reason:** Supplies the service resolver with review rows while keeping
  database access behind the repository interface.
- **Reapply after bump:** Preserve the artifact-scoped list operation.

### `internal/registry/database/postgres.go`
- **Change:** Implemented deterministic artifact-scoped review retrieval ordered
  by `created_at DESC, id DESC`.
- **Reason:** Uses the review index to fetch source rows; Go applies the shared
  staleness and reviewer-resolution rules so unit tests can exercise them
  without HTTP or identity-provider dependencies.
- **Reapply after bump:** Preserve artifact `updated_at` as a separate caller
  input and do not perform staleness or certification logic in SQL.

### `internal/registry/service/service.go`
- **Change:** Added `GetReviewState` to the service interface.
- **Reason:** Exposes derived state internally for later certification consumers
  without adding an HTTP surface in this task.
- **Reapply after bump:** Preserve the artifact-version state evaluation method.

### `internal/registry/service/reviews.go`
- **Change:** Added one reusable current-review resolver and certification
  evaluator. It excludes only rows with `created_at < updated_at`, uses `id DESC`
  for equal timestamps, groups by artifact/review type/reviewer subject, maps
  the configured failure outcome to stable fail status, and derives certified,
  rejected, pending, and per-type states from the current config.
- **Reason:** Centralizes staleness and current-review semantics for all future
  consumers. Artifact `updated_at` is fetched from the existing artifact record
  in the same transaction; review rows are fetched separately.
- **Reapply after bump:** Preserve `ResolveCurrentReviews` as the single
  resolution seam, literal less-than staleness, equal-timestamp current
  behavior, configured failure-outcome mapping, and current-config certification.

### `internal/registry/service/testing/fake_registry.go`
- **Change:** Added a fake `GetReviewState` hook and state field.
- **Reason:** Keeps service consumers and tests compatible with the expanded
  internal interface.
- **Reapply after bump:** Keep the fake synchronized with `RegistryService`.

### `internal/registry/service/reviews_test.go`
- **Change:** Added pure resolver coverage plus real-PostgreSQL sequence tests
  for certification, rejection, pending types, revisions, config changes,
  custom failure outcomes, equal timestamps, stale passes after edits, and the
  reopen-and-recertify hole.
- **Reason:** Verifies derived state through actual review/edit sequences rather
  than constructed end states.
- **Reapply after bump:** Preserve both sequence tests, custom outcome mapping,
  and the staleness falsification coverage. The edit sequences rely on
  PostgreSQL microsecond timestamps and require no sleep.

## AR-2 Task 5 — Certified edit freeze and review capability

### `internal/registry/service/registry_service.go`
- **Change:** Replaced `isArtifactReviewed` with an artifact-version review-row
  lookup and placed the certified freeze before admin and system early returns.
  System sessions are explicitly exempt for the real builtin-seed and
  seed-import writes, which wrap `CreateServer`/`UpdateServer` in
  `auth.WithSystemContext`. Added `CanReview` capability computation through
  the existing `canPerform` predicate. Server list and all-version capability
  annotation now uses one batched review query instead of one query per item.
  Missing official metadata fails closed with an invalid-input error. Review
  authorization uses separate fetch-and-delegate methods with a required,
  artifact-identity-checked snapshot; no variadic optional review rows remain.
- **Reason:** Enforces AR-1 owner edit blocking after any review and freezes
  certified content for real callers, including admin sentinels, while retaining
  the explicit system-session exemption. `CanUpdate` now reflects certification
  automatically because it uses the same authorization path.
- **Reapply after bump:** Preserve freeze-before-bypass ordering, the
  all-review-row owner predicate, explicit system write-path exemption, batched
  list loading, fail-closed metadata handling, required snapshot matching, and
  review capability action check.

### `pkg/registry/database/database.go`
- **Change:** Added `ReviewArtifact` keys and a batched review-list repository
  method.
- **Reason:** Lets catalog and all-version capability annotations load review
  rows in one database query rather than introducing an N+1 query.
- **Reapply after bump:** Preserve the batch artifact key contract and
  `ListReviewsForArtifacts` method.

### `internal/registry/database/postgres.go`
- **Change:** Implemented one parameterized tuple query for batched review
  retrieval, retaining per-artifact read authorization checks.
- **Reason:** Avoids database round trips per catalog item while keeping review
  visibility aligned with artifact read permissions.
- **Reapply after bump:** Preserve the single batched query and deterministic
  `created_at DESC, id DESC` ordering.

### `internal/registry/service/server_capabilities_test.go`
- **Change:** Updated missing-metadata capability coverage to assert fail-closed
  `can_update`.
- **Reason:** A response without official metadata cannot safely establish
  review state or edit integrity.
- **Reapply after bump:** Preserve the false capability assertion.

### `pkg/models/capabilities.go`
- **Change:** Added the serialized `can_review` capability field.
- **Reason:** Exposes review permission consistently with the AR-1B capability
  model.
- **Reapply after bump:** Add required `CanReview bool` alongside the other
  capability fields.

### `pkg/models/capabilities_test.go`
- **Change:** Updated capability JSON coverage for `can_review`.
- **Reason:** Locks down the new wire field.
- **Reapply after bump:** Preserve the serialized false-value assertion.

### `internal/registry/service/reviews_test.go`
- **Change:** Added real-subject tests for owner review blocking, stale-review
  blocking, certified curator/admin freeze, rejected and pending edit
  reopening, review capabilities, certified `can_update` behavior, and
  mismatched review-snapshot rejection.
- **Reason:** Verifies both stacked edit rules and capability enforcement through
  the real service authorization path.
- **Reapply after bump:** Preserve the rule-1, rule-2, sentinel-ordering, and
  capability cases.

### `openapi.yaml`
- **Change:** Regenerated capability schemas to include `can_review`.
- **Reason:** Keeps the API contract synchronized with the capability model.
- **Reapply after bump:** Regenerate after capability changes.

### `ui/lib/api/index.ts`
### `ui/lib/api/sdk.gen.ts`
### `ui/lib/api/types.gen.ts`
- **Change:** Regenerated TypeScript exports and capability types for
  `can_review`.
- **Reason:** Keeps generated UI API types synchronized. No UI rendering logic
  was changed; existing `can_update` behavior now reflects the server freeze.
- **Reapply after bump:** Run `make gen-client`.

## AR-2 Task 6 — Review reads and artifact review summaries

### `internal/registry/api/handlers/v0/reviews.go`
- **Change:** Added a public-read GET operation beside the existing review
  creation route. It returns every raw review row with current/stale markers and
  maps missing artifact versions to 404.
- **Reason:** Findings remain restricted to the dedicated review resource while
  artifact reads expose only the derived summary.
- **Reapply after bump:** Preserve artifact read authorization, deterministic
  row retrieval, and the separate GET resource.

### `internal/registry/api/handlers/v0/reviews_test.go`
- **Change:** Added endpoint coverage for complete rows, markers, and missing
  artifact errors.
- **Reason:** Locks down the review read contract independently from creation.
- **Reapply after bump:** Preserve the GET route and error mapping assertions.

### `internal/registry/service/service.go`
- **Change:** Added the service operation for artifact-scoped review reads.
- **Reason:** Keeps review retrieval behind service authorization and database
  boundaries.
- **Reapply after bump:** Preserve the non-review-permission read path.

### `internal/registry/service/reviews.go`
- **Change:** Added review-row current/stale derivation and a sanitized summary
  projection built from `ResolveReviewState`.
- **Reason:** Callers receive enough context to distinguish stale and current
  rows, while artifact metadata cannot leak notes, subjects, or auth methods.
- **Reapply after bump:** Reuse the existing resolver and retain configured
  pending entries for every review type.

### `internal/registry/service/registry_service.go`
- **Change:** Added review summaries to server, agent, skill, and prompt
  create, read, list, and all-version responses. Catalog and version paths load
  all review rows through one batched query per result set.
- **Reason:** Makes the summary consistent across every artifact family without
  introducing an N+1 review query.
- **Reapply after bump:** Preserve identity-bearing snapshots, one-query batch
  loading, and summary-only artifact metadata.

### `internal/registry/service/testing/fake_registry.go`
- **Change:** Added the fake service hook for full review reads.
- **Reason:** Keeps endpoint tests compatible with the expanded service
  interface.
- **Reapply after bump:** Keep the fake synchronized with `RegistryService`.

### `pkg/models/review.go`
- **Change:** Added optional current/stale row markers and sanitized
  `ReviewSummary`/`ReviewTypeSummary` models.
- **Reason:** Separates the findings-bearing review resource from the safe
  artifact metadata projection.
- **Reapply after bump:** Preserve the marker fields and exclude sensitive row
  fields from summaries.

### `pkg/models/server_response.go`
### `pkg/models/agent.go`
### `pkg/models/skill.go`
### `pkg/models/prompt.go`
- **Change:** Added pointer-plus-`omitempty` `aregistry.ai/review` metadata.
- **Reason:** Exposes the same derived summary shape for all four artifact
  response families.
- **Reapply after bump:** Preserve the metadata key and pointer semantics.

### `internal/registry/service/reviews_test.go`
- **Change:** Added service coverage for read-only review access, empty and
  missing artifact behavior, all overall statuses, all artifact types, and
  serialized summary redaction.
- **Reason:** Verifies resolution reuse, no-findings behavior, and the
  no-sensitive-fields contract at the JSON boundary.
- **Reapply after bump:** Preserve the real-authorizer integration sequences
  and serialization assertions.

### `openapi.yaml`
- **Change:** Regenerated the review GET operation, row marker fields, summary
  schemas, and four artifact metadata schemas.
- **Reason:** Keeps the checked-in API contract synchronized.
- **Reapply after bump:** Run `make gen-client`.

### `ui/lib/api/index.ts`
### `ui/lib/api/sdk.gen.ts`
### `ui/lib/api/types.gen.ts`
- **Change:** Regenerated TypeScript exports, the list-reviews operation, review
  markers, and review summary types.
- **Reason:** Keeps generated frontend API consumers synchronized without
  changing UI behavior.
- **Reapply after bump:** Run `make gen-client`.

## AR-2 Task 6B — Frontend review vocabulary

### `internal/registry/api/handlers/v0/frontend_config.go`
- **Change:** Added configured review types and outcomes to the unauthenticated
  frontend configuration response, sourced through `ReviewConfig()`.
- **Reason:** The browser needs deployment-wide review vocabulary to populate
  submission controls. This does not regress AR-1B's unauthenticated endpoint
  rule: review vocabulary is identical for every caller, like the existing
  Keycloak configuration, and contains no identity-dependent data.
- **Reapply after bump:** Preserve the `review_types` and `review_outcomes`
  fields, their configured order, and the `ReviewConfig()` accessor.

### `internal/registry/api/handlers/v0/frontend_config_test.go`
- **Change:** Added coverage for configured ordering, non-default vocabulary,
  unauthenticated access, and the exact serialized public field set.
- **Reason:** Prevents hardcoded defaults and future identity-dependent fields
  from entering the public bootstrap response.
- **Reapply after bump:** Preserve the non-default configuration and exact JSON
  field-set assertions.

### `openapi.yaml`
- **Change:** Regenerated the frontend configuration schema and endpoint
  description with review vocabulary fields.
- **Reason:** Keeps the checked-in API contract synchronized.
- **Reapply after bump:** Run `make gen-client`.

### `ui/lib/api/sdk.gen.ts`
### `ui/lib/api/types.gen.ts`
- **Change:** Regenerated the TypeScript frontend configuration type with
  `review_types` and `review_outcomes`, plus the generated operation
  documentation from the updated OpenAPI description.
- **Reason:** Makes the deployment vocabulary available to subsequent UI work
  through the generated client without changing UI behavior in this task.
- **Reapply after bump:** Run `make gen-client`.

## AR-2 Task 7 — Detail-view reviews

### `ui/lib/capabilities.ts`
- **Change:** Added the strict `showReview` capability flag mapped only from
  `can_review === true`.
- **Reason:** Keeps review-form visibility based solely on the server-provided
  capability block rather than client-side permission derivation.
- **Reapply after bump:** Preserve the explicit-true `can_review` mapping.

### `ui/lib/__tests__/capabilities.test.ts`
- **Change:** Extended capability mapping coverage for review visibility,
  including absent and false values.
- **Reason:** Locks the form-gating contract to the artifact capability flag.
- **Reapply after bump:** Preserve the explicit-true and absent-capability cases.

### `ui/components/review-section.tsx`
- **Change:** Added shared detail-view review status, findings, stale-row
  presentation, vocabulary-driven submission form, and independent loading and
  error handling for review/config fetches.
- **Reason:** Gives all four artifact details consistent review UX while keeping
  findings readable as escaped React text and review reads available to any
  signed-in viewer.
- **Reapply after bump:** Preserve selected-version endpoint paths, configured
  type/outcome inputs, `can_review` gating, summary-authoritative per-type
  statuses, distinct Passed/Failed/Pending versus Certified/Rejected labels,
  stale distinction, and visible failure states.

### `ui/components/__tests__/review-section.test.tsx`
- **Change:** Added coverage for certified/rejected/pending statuses, pending
  unreviewed types, no-review placeholder, capability gating, findings,
  current/stale distinction, escaped markup, submission errors, and failed
  findings fetches.
- **Reason:** Verifies the detail-view acceptance criteria at rendered output.
- **Reapply after bump:** Preserve the rendered-output escaping assertion and
  non-permission/error cases.

### `ui/components/server-detail.tsx`
### `ui/components/skill-detail.tsx`
### `ui/components/agent-detail.tsx`
### `ui/components/prompt-detail.tsx`
- **Change:** Placed the shared review section after quick-info pills and
  before tabs for each selected artifact version, with a submission refresh
  callback.
- **Reason:** Keeps status and findings visible across tabs and ensures review
  submissions trigger the catalog refresh path.
- **Reapply after bump:** Preserve the artifact type/name/version wiring and
  placement in all four detail components.

### `ui/app/page.tsx`
- **Change:** Passed the existing list `fetchData` callback into all detail
  review sections so successful submissions refresh artifact summaries.
- **Reason:** Reuses the established page refresh path without moving
  selected-version review fetching into the catalog page.
- **Reapply after bump:** Preserve the callback wiring for all four artifact
  detail views.

### `ui/app/__tests__/page-edit-flow.test.tsx`
- **Change:** Updated capability flag expectations for the new `showReview`
  field.
- **Reason:** Keeps the existing page-level capability mapping test aligned
  with the centralized review flag.
- **Reapply after bump:** Preserve the explicit false expectation when the
  fixture omits `can_review`.

## AR-2 Task 8 — Superseded review markers

### `pkg/models/review.go`
- **Change:** Added the optional `is_superseded` review-row field alongside
  `is_current` and `is_stale`.
- **Reason:** Exposes the resolver's independent revision result without
  requiring clients to reconstruct review resolution.
- **Reapply after bump:** Preserve the pointer boolean type, JSON name, and
  `omitempty` behavior.

### `internal/registry/service/reviews.go`
- **Change:** Populated `IsSuperseded` from the latest row per artifact,
  review type, and reviewer subject while leaving current/stale resolution
  unchanged.
- **Reason:** Distinguishes revised rows from rows that are only stale,
  including rows that are both superseded and stale.
- **Reapply after bump:** Preserve created-at/ID ordering and independent
  current, stale, and superseded calculations.

### `internal/registry/service/reviews_test.go`
- **Change:** Added real-JWT-authorizer integration coverage for single
  reviews, revisions, independent reviewers, stale revisions, unrevised
  stale rows, and latest stale rows.
- **Reason:** Locks the row-level resolution semantics and prevents deriving
  supersession from current or stale flags.
- **Reapply after bump:** Preserve real subject identities and both-flag edge
  coverage.

### `openapi.yaml`
### `ui/lib/api/types.gen.ts`
- **Change:** Regenerated the API contract and TypeScript client for the
  optional `is_superseded` review-row field.
- **Reason:** Keeps generated consumers synchronized with the review endpoint.
- **Reapply after bump:** Run `make gen-client`.
