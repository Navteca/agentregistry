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
