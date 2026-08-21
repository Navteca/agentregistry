## Part 0 — Architecture contract

**Every ticket in this epic is bound by this section. It is not background; it is a
requirement. Reviewers should reject work that violates it even if the acceptance
criteria pass.**

### 0.1 Why this matters

We maintain a fork of upstream AgentRegistry. Luis has already reported that the
existing Keycloak integration required modifying core, and that pulling upstream
branches in is painful as a result. Upstream may never accept an authorization
contribution at all (it currently ships without authn/authz, which may be deliberate
product positioning). We therefore optimize for **cleanly mergeable**, not for
*accepted upstream* — if a PR lands upstream, good; if it does not, we must still be
able to track upstream releases without a multi-day merge each time.

### 0.2 The decoupling rules

1. **New behavior lives in new files.** Authorization, identity resolution, and review
   logic go in their own packages/modules. Upstream files are touched only where an
   integration seam is unavoidable.
2. **Each ticket carries an expected upstream footprint.** This is an estimate and a
   checkpoint, **not an acceptance criterion** — do not contort the design to hit a
   number. Its purpose is to make scope creep visible: if the actual number of modified
   upstream files exceeds the estimate, say so in the PR description and explain why. A
   well-justified overrun is fine; a silent one is not. (This exists mainly as an agent
   guardrail — "minimize modifications" is unfalsifiable, and a coding agent will touch
   twenty files and sincerely report that it minimized.)
3. **Seams, not surgery.** Prefer middleware, decorators, interface implementations, and
   registration hooks over inline edits to handler bodies. The ideal upstream diff is a
   single line that registers our component.
4. **Additive persistence only.** New columns are nullable or defaulted; new concepts get
   new tables rather than reshaping existing ones. No destructive migrations, no renamed
   columns, no altered semantics of existing fields. An upstream schema change must be
   able to land without colliding with ours.
5. **Nothing is deleted from upstream.** Functionality we do not want in the NASA
   deployment is hidden through configuration. Removal is a last resort and requires an
   explicit note in the merge log.
6. **Maintain `UPSTREAM-DELTA.md`** at the repo root: every modified upstream file, the
   reason, and how to reapply after a version bump. This file is the deliverable that
   makes the next upstream merge cheap. Each ticket updates it.
7. **No deployment-specific identifiers in code.** No NASA, Goddard, Marshall, GeneLab,
   ECHO, SMCE, or Keycloak strings in any core path. These appear only in configuration
   files and deployment manifests.
8. **Be generous with schema and API contract shape; be stingy with runtime
   configurability.** Configuration machinery can be added later cheaply. A data model
   that external consumers have already integrated against cannot. When in doubt about
   whether something is premature generality, ask which of the two it is: shape, or
   machinery. Build the shape, defer the machinery.

### 0.3 The identity and authorization contract

> **Revised against the codebase, 12 Aug 2026.** An earlier draft of this section
> specified a `PrincipalResolver` and `Authorizer` to be built. Reading AgentRegistry 0.4.0
> showed that upstream already has the authorization primitive. This section now describes
> what exists and where our work attaches. See §0.3.1 for the original design and why it
> was dropped.

**How authentication actually works today.** Authentication is a **token exchange**, not a
pass-through:

```
  Keycloak ID token
        │  verified in internal/registry/api/handlers/v0/auth/oidc.go
        │  (go-oidc: issuer pinned via oidc.NewProvider, aud checked against
        │   clientID, RS256 enforced, verify errors return 401)
        ▼
  oidcClaims { Subject, Issuer, extra claims }   ← short-lived, scoped to oidc.go
        │
        │  buildPermissions(_ *OIDCClaims) → []auth.Permission
        │  ◄── THIS IS WHERE OUR WORK GOES
        ▼
  JWTClaims { AuthMethod, AuthMethodSubject, Permissions[], Issuer,
              Subject, Audience, ExpiresAt, NotBefore, IssuedAt, ID }
        │  signed EdDSA in pkg/registry/auth/jwt.go
        ▼
  registry token rides every subsequent API request
        │
        ▼
  HasPermission / isResourceMatch  → allow / deny
```

The Keycloak ID token is verified and then **discarded**. Only the registry-minted EdDSA
token reaches the request path. Keycloak is configured as the OIDC issuer
(`OIDCIssuer`/`OIDCClientID`); there is no Keycloak-specific Go handler, and the
Keycloak-related additions Luis made are config, a public frontend-config endpoint, route
wiring, and a hardening of `PublicActions`.

**The existing authorization primitive.**

```go
type Permission struct {
    Action          PermissionAction  // publish, edit, delete, deploy, read
    ResourcePattern string
}
```

`isResourceMatch` supports exactly three forms: `*` (all), a trailing-`*` prefix glob, and
exact string match. **There is no variable interpolation** — a pattern cannot express
"resources created by this subject."

Enforcement is action equality plus pattern match against the permission list embedded in
the token at issuance. This is the same decision shape the original design proposed, arrived
at independently by upstream. **We do not build a second one.**

**Where permissions come from today.** Each auth method has its own builder. Three of them
derive permissions from proven identity; one does not:

| Method | Builder | Behavior |
|---|---|---|
| DNS/HTTP domain signature | `BuildPermissions` (common.go) | `publish` on `<reverse-domain>/*`, from the proven domain |
| GitHub access token | `buildPermissions` (github_at.go) | `publish` on `io.github.<user>/*` and each org — per caller |
| GitHub OIDC (Actions) | `buildPermissions` (github_oidc.go) | `publish` on `io.github.<repo_owner>/*`, from a validated claim |
| **Generic OIDC (our path)** | `buildPermissions` (oidc.go) | **Ignores claims entirely** (`_ *OIDCClaims`); returns static operator-configured env-var patterns — identical for every caller |
| Anonymous (`none`) | `GetAnonymousToken` (none.go) | Fully hardcoded, dev/test only, gated by `EnableAnonymousAuth` |

The generic OIDC builder is the only one that discards its claims. Its discarded parameter
is the seam.

**Our design, restated against reality.**

1. `buildPermissions` in `oidc.go` reads roles from the claims it is already handed.
2. Configured mapping turns external role strings into the internal vocabulary — exactly
   `user`, `curator`, `admin`.
3. Each internal role expands to a bundle of `[]auth.Permission`.
4. Callers with no recognized role receive the existing env-var patterns
   (`OIDC_PUBLISH_PERMISSIONS` and siblings), which become the `default_role` bundle. That
   mechanism already exists; it simply is not role-aware.
5. Structure the builder to match `github_at.go` — dynamic, derived from validated
   identity. That is the shape upstream already accepts, and our strongest upstreamability
   argument in the epic.

**Claim extraction stays dumb.** A dotted-path lookup into the decoded claims
(`realm_access.roles`) — not an expression language, not JSONPath, not a plugin system.

**No provider concepts leak.** The builder reads configured claim paths; it does not know
the issuer is Keycloak. Role names, claim paths, and bundles are configuration.

Illustrative configuration (keys settled during implementation; the requirement is that
none of it is compiled in):

```yaml
oidc:
  roles_claim: "realm_access.roles"   # dotted path; no code default
  name_claim:  "preferred_username"   # display only; falls back to subject
  role_map:                           # external string -> internal role
    registry-admin:   admin
    registry-curator: curator
    registry-user:    user
  default_role: user                  # falls through to existing env-var patterns
```

**What does not follow from the permission model.** Because `ResourcePattern` has no
interpolation, **owner-scoped update is not expressible as a permission.** Patterns match
resource names; ownership is a property of the record. Owner-scoped update therefore
remains an in-handler check against the stored owner (AR-1).

Resolved. OIDC users do not receive an owner-namespaced pattern. buildPermissions returns configured static patterns, identical regardless of subject — unlike the GitHub builders, which derive io.github.<user>/* from proven identity. Nor could a prefix pattern cover the catalog: server names are constrained to namespace/name, but agent, skill, and prompt names forbid slashes entirely, so no prefix exists for three of the four types. Ownership is therefore an in-handler check against the stored owner for all four.

### 0.3.1 Superseded design (retained for rationale)

The original §0.3 specified a `PrincipalResolver` interface producing a generic
`Principal{Subject, Name, Roles[], Groups[], Attributes{}}`, and an `Authorizer` interface
with a static RBAC table, both to be written by us and registered as middleware.

It is recorded here because the *reasoning* still holds and shaped the rest of the epic:
authorization decisions belong behind one interface; provider concepts must not leak into
core; claim extraction is configuration. Upstream simply satisfies those requirements
already, under different names — `Permission` and `HasPermission` are the `Authorizer`, and
`buildPermissions` is the resolver.

Building the proposed layer alongside upstream's would have produced two authorization
systems in one codebase and a fork that could never be merged upstream. **Do not
reintroduce it.** If a future requirement genuinely cannot be expressed in
`Permission`/`ResourcePattern`, extend that model or add a check beside it — do not
replace it.

### 0.4 Cross-cutting security constraints

These apply across the epic and to work not yet scoped. They are recorded here rather than
in a single ticket because the code that violates them will most likely be written by
someone reading a *later* ticket.

**Registered URLs are attacker-controlled input.** An MCP server registration is, at its
core, a URL supplied by a user. Nothing in this epic fetches those URLs — but Ramon's
automated scoring and best-practice checks explicitly will, and a health check or metadata
probe is an obvious near-term addition. The moment any component fetches a registered URL,
it becomes a server-side request forgery vector originating inside SMCE (OWASP A10). The
canonical target is the cloud instance metadata endpoint, which yields IAM credentials.

Any future work that fetches a registered URL must: allowlist schemes to `http`/`https`;
resolve DNS and reject loopback, link-local, and private address ranges *after* resolution
(rejecting only the hostname is bypassable via DNS rebinding and redirects); refuse to
follow redirects, or re-validate each hop; enforce timeouts and response size limits; and
run through a controlled egress path rather than direct outbound from the application.
**File this as a ticket when scoring work is scoped — do not let it be discovered during
implementation.**

**Never trust the registry's own catalog as safe content.** Registered servers are
self-declared by default. Any consumer — agentgateway, the research platform, an agent
invoking a tool — is consuming third-party content. This is the substance behind AR-3: the
answer to "do unreviewed servers reach the research platform" is a security decision, not
a product preference.

**Dependency and supply-chain hygiene.** We merge upstream code on an ongoing basis
(OWASP A06/A08). Dependency scanning runs on merge, and an upstream version bump includes
reviewing its dependency delta rather than only its source diff.

**Secrets.** No credential, client secret, realm name, or token appears in the repository,
in `UPSTREAM-DELTA.md`, or in log output. Tokens and raw claim payloads are never logged,
including at debug level.

### 0.6 Integration patterns: where our code attaches

> **Revised against the codebase, 12 Aug 2026.** The earlier version of this section
> specified a middleware/in-handler split for authorization we would write. Upstream
> already owns the enforcement path, so most of that split no longer applies. The framework
> boundary rule survives and still matters.

**Enforcement already exists and is already in-handler.** `HasPermission` /
`isResourceMatch` run against the permission list embedded in the registry token. We do not
add authorization middleware, and we do not wrap the router. Adding a second enforcement
path would create exactly the drift §0.2 exists to prevent.

**Our code attaches at two points, and only two:**

1. **Token issuance** — `buildPermissions` in `oidc.go`. Roles are read from claims and
   expanded into permission bundles. This runs once per login, not per request. All
   role-to-permission logic lives here.
2. **In-handler ownership checks** — for rules the permission model cannot express (owner
   may update own unreviewed artifact), a check after the artifact loads, against the
   stored owner. One call per affected handler.

**Default-deny is likely already handled.** `PublicActions` in `pkg/registry/auth/authz.go`
was cleared and hardened as part of the Keycloak work; most operations now require a valid
session. **Verify whether it is an allowlist of public actions** — if so, AR-1's default-deny
requirement is already satisfied and the route-enumeration test becomes a regression guard
on `PublicActions` rather than new middleware.

**The framework boundary still applies.** Role-mapping and bundle-expansion logic should sit
in plain functions that take claims and return `[]auth.Permission`, importing no HTTP
framework — testable without a request, a router, or an IdP. The handler is a thin caller.
This is also how `github_at.go` is structured, which is the shape we are matching.

**Unauthenticated by design.** `/v0/config/frontend` has no caller identity. Nothing
identity-dependent may be served from it — in particular, AR-1B capability flags must not
go there.

**Deployment check.** `EnableAnonymousAuth` grants read/publish/edit/delete/deploy on
`io.modelcontextprotocol.anonymous/*` to any caller with no authentication at all. It is
dev/test only by design. **Confirm it is off in the SMCE deployment** (AR-7) — a one-line
check, and a bad thing to discover later.