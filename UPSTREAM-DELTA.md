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
- **Change:** Added `Subject` and `DisplayName` fields to the `User` struct.
- **Reason:** Downstream handlers need the authenticated caller's identity for
  ownership recording and presentation. Upstream's `User` exposed only
  `Permissions`, so no handler could see *who* the caller was, regardless of auth
  method. `Subject` is read-only identity (used for ownership); `DisplayName` is
  presentation-only and must never be authorized against.
- **Reapply after bump:** Re-add the two fields to `User`. Purely additive; named
  struct literals elsewhere are unaffected.

### `pkg/registry/auth/jwt.go`
- **Change:** (1) Added `AuthMethodDisplayName` field to `JWTClaims`
  (`json:"auth_method_name,omitempty"`). (2) `jwtSession.Principal()` now populates
  `User.Subject` from `claims.AuthMethodSubject` and `User.DisplayName` from
  `claims.AuthMethodDisplayName`.
- **Reason:** Carries the caller's stable subject and a display-name snapshot on
  the registry JWT so they are available at artifact-create time via the session.
  Additive claim; existing tokens without the claim decode with an empty
  display name.
- **Reapply after bump:** Re-add the claim field and the two assignments in
  `Principal()`.

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
  fallback when the configured claim is absent or empty.
- **Reason:** Verifies role-based permission minting and preservation of the static
  fallback behavior.
- **Reapply after bump:** Reapply the focused OIDC exchange tests.

### `charts/agentregistry/values.yaml` and `charts/agentregistry/templates/configmap.yaml`
- **Change:** Added Helm values and ConfigMap environment-variable templating for
  OIDC role claim mapping and role-scoped resource patterns.
- **Reason:** Makes role-based OIDC permission configuration available to Helm
  deployments.
- **Reapply after bump:** Re-add the six role-mapping values and corresponding
  `AGENT_REGISTRY_OIDC_*` ConfigMap entries.

### `pkg/registry/auth/jwt.go`
- **Change:** (1) Added a new `PermissionActionAdmin PermissionAction = "admin"`
  sentinel action. `GenerateTokenResponse`'s namespace-denylist bypass check
  now requires `perm.Action == PermissionActionAdmin && perm.ResourcePattern
  == "*"` instead of a bare `perm.ResourcePattern == "*"` on any action.
- **AR-1:** Added `PermissionActionEditOwn PermissionAction = "edit_own"` and
  exposed the existing resource-pattern matcher through the package-level
  `HasPermission` helper so transactional owner checks reuse authorization
  matching without changing the admin sentinel semantics.
- **Reason:** Upstream's denylist bypass treated *any* permission with
  `ResourcePattern: "*"` as admin, regardless of its `Action`. A caller with
  only `{Action: publish, ResourcePattern: "*"}` (a legitimate "publish
  anywhere" grant, useful in deployments with no publisher-derived namespace)
  silently bypassed the namespace denylist meant to block abusive publishers.
  The new sentinel makes "unbounded bypass" an explicit, distinct grant from
  "unbounded publish/read/edit/delete/deploy".
- **Reapply after bump:** Re-add the `PermissionActionAdmin` constant and
  change the `hasGlobalPermissions` predicate in `GenerateTokenResponse` to
  match on `Action == PermissionActionAdmin` in addition to the wildcard
  pattern.

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

### `pkg/models/server_response.go`, `pkg/models/agent.go`, `pkg/models/skill.go`, `pkg/models/prompt.go`
- **Change:** Adds Ownership *OwnershipMeta extension field so artifact responses carry the registering subject and display-name snapshot. Type is defined in the new file pkg/models/ownership.go; these files gain one field each. To reapply after a version bump: re-add the field to the struct.

### Server response type migration

### `internal/registry/database/postgres.go`
- **Change:** Server query and mutation methods now return fork-local `models.ServerResponse` values. Semantic search scores are assigned to `ServerResponse.Meta.Semantic` directly instead of round-tripping through publisher-provided metadata.
- **Reason:** Makes the database layer consistent with the agent, skill, and prompt response paths while preserving the public response JSON.
- **Reapply after bump:** Change the eight server response return types and constructors to `models.ServerResponse`; assign semantic scores to `Meta.Semantic` and keep the upstream server JSON unchanged.

### `pkg/registry/database/database.go`
- **Change:** Updated the database interface signatures for the eight server response methods to return `models.ServerResponse`.
- **Reason:** Keeps the interface aligned with the PostgreSQL implementation.
- **Reapply after bump:** Replace the eight upstream server response return types with the fork-local model type.

### `internal/registry/service/registry_service.go`
- **Change:** Propagated `models.ServerResponse` through the server service methods and transaction callbacks.
- **Reason:** Carries fork-local response metadata from the database to API consumers without converting back to the upstream response.
- **Reapply after bump:** Update the server service method and transaction callback return types to `models.ServerResponse`.

### `internal/registry/service/service.go`
- **Change:** Updated the server methods on `RegistryService` to return fork-local response models.
- **Reason:** Exposes the migrated response type at the service boundary.
- **Reapply after bump:** Update the six server response signatures in the interface to use `models.ServerResponse`.

### `internal/registry/api/handlers/v0/servers.go`
- **Change:** Retained `normalizeServerResponse` as a nil-safe pass-through for `models.ServerResponse`; removed semantic score extraction from publisher-provided metadata.
- **Reason:** The database now supplies fork-local metadata directly, so handler conversion is no longer needed while existing handler call sites remain stable.
- **Reapply after bump:** Change the normalizer input to the fork-local response and return its value unchanged, preserving the nil guard.

### AR-1 — `internal/registry/api/handlers/v0/auth/oidc_roles.go`
- **Change:** Added `edit_own` to the user permission bundle and updated the
  bundle documentation.
- **Reason:** Users need a distinct permission for owner-scoped edits without
  receiving curator-level `edit`.
- **Reapply after bump:** Add `PermissionActionEditOwn` to the user actions and
  update the bundle documentation.

### AR-1 — `internal/registry/api/handlers/v0/edit.go`
- **Change:** Removed the admin-only wording from the server edit endpoint
  description.
- **Reason:** Unreviewed owner edits are now supported.
- **Reapply after bump:** Update the endpoint description to describe general
  server updates.

### AR-1 — `internal/registry/database/postgres.go`
- **Change:** Server update and status mutations accept either `edit` or
  `edit_own` at the database authorization gate.
- **Reason:** The database layer provides only the coarse edit-authority gate;
  ownership and review narrowing belongs to the service transaction.
- **Reapply after bump:** Use the same shared coarse edit check for both server
  mutation methods.

### AR-1 — `internal/registry/service/registry_service.go`
- **Change:** Added transactional owner-scoped server update authorization,
  comparing stable subjects and rejecting unowned or reviewed artifacts for
  `edit_own`; added the `isArtifactReviewed` AR-2 stub. The
  `IsSystemSession` branch preserves the existing internal-system authorization
  bypass and does not grant new user authority.
- **Reason:** The ownership decision must use the row fetched in the same
  transaction as the write. Display names are never authorization inputs.
- **Reapply after bump:** Add the narrowing check after the transactional
  current-server fetch and retain the review predicate insertion point.

### AR-1 — `internal/registry/service/ownership_test.go`
- **Change:** Added PostgreSQL-backed coverage for owner edits, cross-subject
  refusal, unowned artifacts, curator override, same-display-name subjects,
  and display-name changes.
- **Reason:** Verifies subject-only owner authorization and the curator
  permission path.
- **Reapply after bump:** Reapply the focused owner-scoped update tests.

### AR-1 — `internal/registry/api/handlers/v0/auth/oidc_roles_test.go`
- **Change:** Updated role-bundle expectations for the user-granted
  `edit_own` permission.
- **Reason:** Keeps role-mapping tests aligned with the new user bundle.
- **Reapply after bump:** Reapply the `edit_own` expectations to role-bundle
  tests.

### AR-1 — `internal/registry/api/handlers/v0/auth/oidc_test.go`
- **Change:** Updated mapped-user token exchange expectations for `edit_own`.
- **Reason:** Verifies the new permission is carried into signed user tokens.
- **Reapply after bump:** Reapply the `edit_own` expectation to the mapped-role
  exchange test.

### AR-1 — `internal/registry/database/postgres_test.go`
- **Change:** Authenticated setup contexts are used for server and deployment
  fixture creation.
- **Reason:** Authentication is mandatory after the upstream `PublicActions`
  regression fix; these fixtures are not testing anonymous access.
- **Reapply after bump:** Use `database.WithTestSession` for setup mutations.

### AR-1 — `internal/registry/api/handlers/v0/edit_test.go`
- **Change:** Authenticated setup contexts are used for all server fixture
  creation.
- **Reason:** The endpoint tests exercise authenticated edit behavior and must
  not rely on the removed sessionless public actions.
- **Reapply after bump:** Use the existing `database.WithTestSession` helper
  for setup creates.

### AR-1 — `internal/registry/service/registry_service_test.go`
- **Change:** Server update fixtures now include a deliberately non-existent,
  structurally valid fixture repository URL, disable repository reachability,
  and use the shared authenticated test session for setup.
- **Reason:** Structural repository validation remains required, while tests
  must not make live network requests; setup mutations also require auth.
- **Reapply after bump:** Preserve the fixture-only repository and
  `WithTestSession` setup context.

### `internal/registry/api/handlers/v0/scoring.go`
- **Change:** Updated the server scoring persistence seam to accept `models.ServerResponse`.
- **Reason:** The scoring endpoint consumes the migrated service response type.
- **Reapply after bump:** Use `*models.ServerResponse` for the server argument passed to scoring persistence.

### `internal/mcp/registryserver/server.go`
- **Change:** Server discovery MCP tools now return `models.ServerListResponse` so fork-local response metadata is retained.
- **Reason:** Avoids converting the migrated service response back to the upstream type and preserves semantic metadata in the serialized output.
- **Reapply after bump:** Use the fork-local server list/response models for the two server discovery tools and `fetchSingleServer`.

### `internal/registry/database/postgres_test.go`
- **Change:** Updated the pagination result accumulator to the migrated server response type.
- **Reason:** Keeps database tests aligned with the database interface without changing assertions.
- **Reapply after bump:** Change the accumulator type to `[]*models.ServerResponse`.

### `internal/registry/service/registry_service_test.go`
- **Change:** Updated server response callback and mock database types to the migrated model.
- **Reason:** Keeps service tests aligned with the service and database interfaces; test expectations are unchanged.
- **Reapply after bump:** Update server response callback, mock, and result types to `models.ServerResponse`.

### `internal/registry/service/testing/fake_registry.go`
- **Change:** Kept existing upstream-shaped test setup hooks and added conversion at the fake service boundary to return `models.ServerResponse`.
- **Reason:** Preserves existing test fixtures while allowing the fake to implement the migrated service interface.
- **Reapply after bump:** Keep the fixture compatibility conversion if the service response boundary is regenerated.

### `internal/registry/importer/importer_test.go`
- **Change:** Explicitly converts the migrated database response to the upstream response shape used by the importer fixture.
- **Reason:** The importer fixture tests the upstream export payload and does not consume fork-local response metadata.
- **Reapply after bump:** Preserve the explicit `Server`/official-meta conversion at the upstream fixture boundary.

### Server ownership capture

### `pkg/registry/auth/auth.go`
- **Change:** Added `User.AuthMethod` alongside the existing subject and display-name identity fields.
- **Reason:** The service must explicitly distinguish anonymous authentication from other methods without inspecting request context in the database layer.
- **Reapply after bump:** Re-add the typed `AuthMethod Method` field to `User`.

### `pkg/registry/auth/jwt.go`
- **Change:** `jwtSession.Principal()` now copies `JWTClaims.AuthMethod` into `User.AuthMethod`.
- **Reason:** Ownership resolution needs the authenticated method to omit anonymous ownership and persist the method string for non-anonymous creators.
- **Reapply after bump:** Re-add the `AuthMethod` assignment in `Principal()`.

### `internal/registry/service/registry_service.go`
- **Change:** Resolves ownership once per server create operation and passes `models.OwnershipInput` explicitly to the database create method.
- **Reason:** Keeps authentication resolution in the service layer while leaving the database context-free; request metadata cannot override the authenticated identity.
- **Reapply after bump:** Resolve ownership before opening the transaction, pass it through `createServerInTransaction`, and append it to `Database.CreateServer`.

### `pkg/registry/database/database.go`
- **Change:** Added `models.OwnershipInput` to the `CreateServer` database interface method.
- **Reason:** Makes ownership data flow explicit at the persistence boundary.
- **Reapply after bump:** Add the ownership value object as the final `CreateServer` parameter.

### `internal/registry/database/postgres.go`
- **Change:** Server inserts persist nullable subject, display-name, and auth-method columns. All five server read paths scan nullable ownership columns and populate `ServerResponseMeta.Ownership`; anonymous and empty-subject inputs write all three columns as `NULL`.
- **Reason:** Captures authenticated ownership without allowing request payload metadata or display names to influence identity, and omits ownership for legacy/unowned rows.
- **Reapply after bump:** Add the three nullable columns and `sql.NullString` scan destinations to each server read query, pass `OwnershipInput` to the insert, and preserve named-column-only update statements.

### `internal/registry/database/ownership.go`
- **Change:** Added fork-local helpers for constructing ownership metadata from nullable database columns or create input.
- **Reason:** Keeps repeated null handling and omission semantics consistent across server response constructors.
- **Reapply after bump:** Re-add the two ownership metadata conversion helpers.

### `internal/registry/database/postgres_test.go`
- **Change:** Updated direct database create calls for the explicit ownership argument.
- **Reason:** Keeps the upstream database tests aligned with the new interface.
- **Reapply after bump:** Supply an empty `models.OwnershipInput` in existing ownership-neutral fixtures.

### `pkg/models/ownership.go`
- **Change:** Added `OwnershipInput{Subject, DisplayName, AuthMethod}` for explicit service-to-database ownership flow.
- **Reason:** Gives tasks 9–11 one stable value-object parameter to copy without adding bare string arguments.
- **Reapply after bump:** Re-add the three plain string fields to the ownership input type.

### Agent ownership capture

### `internal/registry/service/registry_service.go`
- **Change:** Resolves ownership once per agent create operation and passes `models.OwnershipInput` through the transaction to the database create method.
- **Reason:** Keeps authentication resolution in the service layer and prevents request payload metadata from overriding the authenticated creator.
- **Reapply after bump:** Resolve ownership before opening the transaction, pass it through `createAgentInTransaction`, and append it to `Database.CreateAgent`.

### `pkg/registry/database/database.go`
- **Change:** Added `models.OwnershipInput` to the `CreateAgent` database interface method.
- **Reason:** Makes agent ownership data flow explicit at the persistence boundary.
- **Reapply after bump:** Add the ownership value object as the final `CreateAgent` parameter.

### `internal/registry/database/postgres.go`
- **Change:** Agent inserts persist nullable subject, display-name, and auth-method columns. All five agent read paths scan nullable ownership columns and populate `AgentResponseMeta.Ownership`; named-column-only update and status-update statements remain unchanged.
- **Reason:** Captures authenticated ownership without trusting request payload data, preserves ownership through updates, and omits ownership for legacy/unowned rows.
- **Reapply after bump:** Add the three nullable columns and `sql.NullString` scan destinations to each agent read query, pass `OwnershipInput` to the insert, and use `ownershipMetaFromInput` for the create response.

### `internal/registry/service/ownership_test.go`
- **Change:** Added agent service coverage for authenticated ownership capture, ignoring ownership-shaped request data, and distinct subjects sharing a display name.
- **Reason:** Verifies agent ownership is resolved from the authenticated principal rather than the request body and remains subject-based.
- **Reapply after bump:** Reapply the focused agent ownership service tests.

### `internal/registry/database/ownership_test.go`
- **Change:** Added agent database coverage for ownership surviving update/status-update operations, all five read paths, and omission of the ownership JSON key for NULL ownership columns.
- **Reason:** Verifies persistence, read-path consistency, update safety, and JSON omission semantics.
- **Reapply after bump:** Reapply the focused agent ownership database tests.

### Skill ownership capture

### `internal/registry/service/registry_service.go`
- **Change:** Resolves ownership once per skill create operation and passes `models.OwnershipInput` through the transaction to the database create method.
- **Reason:** Keeps authentication resolution in the service layer and prevents request payload metadata from overriding the authenticated creator.
- **Reapply after bump:** Resolve ownership before opening the transaction, pass it through `createSkillInTransaction`, and append it to `Database.CreateSkill`.

### `pkg/registry/database/database.go`
- **Change:** Added `models.OwnershipInput` to the `CreateSkill` database interface method.
- **Reason:** Makes skill ownership data flow explicit at the persistence boundary.
- **Reapply after bump:** Add the ownership value object as the final `CreateSkill` parameter.

### `internal/registry/database/postgres.go`
- **Change:** Skill inserts persist nullable subject, display-name, and auth-method columns. All five skill read paths scan nullable ownership columns and populate `SkillResponseMeta.Ownership`; named-column-only update and status-update statements remain unchanged.
- **Reason:** Captures authenticated ownership without trusting request payload data, preserves ownership through updates, and omits ownership for legacy/unowned rows.
- **Reapply after bump:** Add the three nullable columns and unconditional `sql.NullString` scan destinations to each skill read query, pass `OwnershipInput` to the insert, and use `ownershipMetaFromInput` for the create response.

### `internal/registry/service/ownership_test.go`
- **Change:** Added skill service coverage for authenticated ownership capture, ignoring ownership-shaped request data, anonymous omission, and distinct subjects sharing a display name.
- **Reason:** Verifies skill ownership is resolved from the authenticated principal rather than the request body and remains subject-based.
- **Reapply after bump:** Reapply the focused skill ownership service tests.

### `internal/registry/database/ownership_test.go`
- **Change:** Added skill database coverage for ownership surviving update/status-update operations, all five read paths, and omission of the ownership JSON key for NULL ownership columns.
- **Reason:** Verifies persistence, read-path consistency, update safety, and JSON omission semantics.
- **Reapply after bump:** Reapply the focused skill ownership database tests.

### Prompt ownership capture

### `internal/registry/service/registry_service.go`
- **Change:** Resolves ownership once per prompt create operation and passes `models.OwnershipInput` through the transaction to the database create method.
- **Reason:** Keeps authentication resolution in the service layer and prevents request payload metadata from overriding the authenticated creator.
- **Reapply after bump:** Resolve ownership before opening the transaction, pass it through `createPromptInTransaction`, and append it to `Database.CreatePrompt`.

### `pkg/registry/database/database.go`
- **Change:** Added `models.OwnershipInput` to the `CreatePrompt` database interface method.
- **Reason:** Makes prompt ownership data flow explicit at the persistence boundary.
- **Reapply after bump:** Add the ownership value object as the final `CreatePrompt` parameter.

### `internal/registry/database/postgres.go`
- **Change:** Prompt inserts persist nullable subject, display-name, and auth-method columns. All five prompt read paths scan nullable ownership columns and populate `PromptResponseMeta.Ownership`; named-column-only latest-version updates remain unchanged.
- **Reason:** Captures authenticated ownership without trusting request payload data, preserves ownership through latest-version promotion, and omits ownership for legacy/unowned rows.
- **Reapply after bump:** Add the three nullable columns and unconditional `sql.NullString` scan destinations to each prompt read query, pass `OwnershipInput` to the insert, and use `ownershipMetaFromInput` for the create response.

### `internal/registry/service/ownership_test.go`
- **Change:** Added prompt service coverage for authenticated ownership capture, ignoring ownership-shaped request data, anonymous omission, and ownership surviving latest-version promotion.
- **Reason:** Verifies prompt ownership is resolved from the authenticated principal rather than the request body and remains intact across the prompt mutation that exists.
- **Reapply after bump:** Reapply the focused prompt ownership service tests.

### `internal/registry/database/ownership_test.go`
- **Change:** Added prompt database coverage for ownership on all five read paths, preservation through latest-version promotion, and omission of the ownership JSON key for NULL ownership columns.
- **Reason:** Verifies prompt persistence, read-path consistency, the only prompt mutation affecting rows, and JSON omission semantics.
- **Reapply after bump:** Reapply the focused prompt ownership database tests.

### AR-1 — `ui/components/server-detail.tsx`
- **Change:** Scheme-allowlisted artifact-supplied website, repository, remote, and icon URLs before rendering them as links or images; rejected link values remain visible as text.
- **Reason:** Closes the AR-1 criterion on URL scheme allowlisting. The pre-existing form validation was bypassable when artifacts were published directly through the API.
- **Reapply after bump:** Retain the `getSafeHttpUrl` validation at every server detail URL and icon rendering site.
