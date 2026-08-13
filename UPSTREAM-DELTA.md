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
- **Reason:** The registry must mint permissions based on the caller's mapped OIDC
  role while rejecting malformed role-map configuration at startup rather than
  silently falling back to an unbounded static bundle.
- **Reapply after bump:** Re-add the cached role-mapping configuration and the
  resolver call in `buildPermissions`.

### `internal/registry/api/handlers/v0/auth/oidc_test.go`
- **Change:** Added token-exchange coverage for mapped, unknown, missing, and
  malformed OIDC role claims.
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

---

## AR-2 — Action-aware IsRegistryAdmin bypass

### `pkg/registry/auth/jwt.go`
- **Change:** Added a new `PermissionActionAdmin PermissionAction = "admin"`
  sentinel action. `GenerateTokenResponse`'s namespace-denylist bypass check
  now requires `perm.Action == PermissionActionAdmin && perm.ResourcePattern
  == "*"` instead of a bare `perm.ResourcePattern == "*"` on any action.
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

### Known related issue, explicitly NOT changed here
`pkg/registry/auth/jwt.go`'s `Check` still has no denylist enforcement at
request time (only at token-mint time, in `GenerateTokenResponse`). This is
unrelated to the `IsRegistryAdmin` bypass fixed above and is out of scope -
noted here so it isn't mistaken for having been addressed.

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
