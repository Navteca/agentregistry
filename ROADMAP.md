# Roadmap — Navteca Agentregistry

Roadmap of upcoming initiatives for Navteca's fork of [agentregistry](https://github.com/agentregistry-dev/agentregistry) (formerly Solo.io).

| # | Name + brief description | Now/Next/Later | Priority |
|---|---|---|---|
| 1 | **Sync `develop` with upstream (agentregistry-dev)** — `develop` is ~33 commits behind `main`, which already has upstream merged up to PR #494 (v1alpha1 API refactor, declarative CLI rework, provider→runtime rename). This is technical debt that grows every week and makes the next merge more painful. | Now | High |
| 2 | **Real per-user/group RBAC on top of Keycloak** — Keycloak login already works (exchanges the ID token for our own JWT), but today every authenticated user gets the same permission set, defined via static environment variables — permissions are not differentiated by Keycloak role or group. Missing: mapping Keycloak group/role claims to distinct per-user permissions. | Now → Next | High |
| 3 | **Complete MCP server Scoring** — the integration already exists (calls an external `mcp-scoring` microservice that doesn't live in this repo, with deterministic rules + LLM judge) but only runs once, at server creation. Missing: on-demand re-scoring, score history, bulk scoring of the existing/imported catalog. | Next | High |
| 4 | **Native role management (Admin / Publisher / Viewer)** — there is currently no concept of a "role" at all: only a flat permission list (`read/publish/edit/delete/deploy`) with no roles table or per-user permission persistence. Requires a new DB schema, service layer, and admin UI. Depends on / builds on item 2. | Next → Later | High |
| 5 | **MCP Server registration and approval flow (CMS-style)** — today publishing is fully automatic: anyone with `publish` permission adds a server and it goes live in the catalog immediately, with no draft, "pending" state, or review queue. This is a fully new feature: new statuses (`pending/approved/rejected`), new permission actions, moderation UI. | Later | Medium |
| 6 | **Deployment manifest customization** — today the core doesn't generate raw YAML: it translates the internal model into kagent/kmcp Custom Resources, and the kagent operator handles the actual deployment. Two sub-paths: (a) quick win — improve parity/error handling between the local adapter (docker-compose) and the k8s adapter (there's already a half-finished branch, `fix/k8s-kagent-missing-crd-error`); (b) larger effort — support generating manifests outside of kagent/kmcp (plain Helm/Kustomize or another GitOps target), which would be a brand-new platform adapter. | Later | Medium |

## Open notes and dependencies

- **Item 3 (scoring):** if Navteca owns the external `mcp-scoring` service, its roadmap likely lives in a separate repo — worth splitting out or at least flagging as a cross-repo dependency.
- **Minor cleanup not listed above:** there are 7 remote branches (`feature/edit-server`, `feature/mcp-scoring`, `feature/remove-server`, `feature/unsoloio`, `fix/builtin-seed-invalid-repository-url`, `fix/k8s-kagent-missing-crd-error`, `chore/backfill-pr-trace-edit-server`) that appear to already be merged in spirit into `develop` — candidates for deletion/cleanup.
- Item 1 is prioritized first because the longer it's postponed, the more conflict-prone the merge becomes — it's the hard dependency that (indirectly) blocks everything else from moving forward without extra friction.
