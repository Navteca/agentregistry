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
