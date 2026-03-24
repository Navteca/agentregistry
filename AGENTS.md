# AGENTS.md - Development Guidelines

AgentRegistry is a centralized registry for curating, discovering, deploying, and managing agentic infrastructure (MCP servers, agents, skills).

**Stack:** Go 1.25+ · PostgreSQL/pgvector (pgx) · Cobra CLI · Huma (OpenAPI) · Next.js 14 (App Router) · Tailwind CSS

---

## Build & Run Commands

```bash
make build-cli          # Build bin/arctl (CLI)
make build-server       # Build bin/arctl-server (server binary)
make build-ui           # Build Next.js UI and embed into internal/registry/api/ui/dist/
make build              # build-ui + build-cli
make dev-ui             # Run Next.js dev server
make run                # Start full local env (docker-compose + build-cli)
make down               # Stop local env
```

## Test Commands

```bash
make test-unit          # Unit tests only  (-tags=unit)
make test               # Integration tests (-tags=integration, needs DB)
make e2e                # End-to-end tests  (-tags=e2e, needs full env)
make test-coverage      # Unit+integration tests with -cover
make test-coverage-report  # Generates coverage.html

# Run a single test / package directly:
go test -tags=unit -run TestFunctionName ./internal/path/to/package/...
go test -tags=unit -run TestFunctionName -v ./...

# Run all unit tests with pretty output (same as make test-unit):
go tool gotestsum --format testdox -- -tags=unit -timeout 5m ./...
```

Tests use build tags to separate concerns:
- `//go:build unit` — fast, no external dependencies
- `//go:build integration` — requires a running PostgreSQL instance
- `//go:build e2e` — requires the full stack

## Lint & Format

```bash
make lint               # Run golangci-lint --fix
go tool golangci-lint run          # Lint without auto-fix
go tool golangci-lint run ./pkg/...  # Lint a specific subtree
make lint-ui            # Run eslint on ui/
make mod-tidy           # go mod tidy
```

Linter config: `.golangci.yaml`. Enabled linters include `staticcheck`, `govet`, `unused`, `misspell`, `nestif`, `modernize`, `testifylint`, `depguard`. **`sort` package is forbidden — use `slices` instead.**

Formatters enforced: `gofmt` + `goimports`. Run `goimports -w .` or let `make lint` auto-fix.

---

## Directory Structure

```
cmd/            # Entry points only — delegate immediately to internal/ or pkg/
pkg/            # Public, reusable packages (models, printer, registry/database interfaces)
internal/
  registry/
    api/        # HTTP handlers (transport only — parse, call service, map errors)
    database/   # ONLY layer that touches the DB directly
    service/    # Business logic; receives DB interface via constructor
    platforms/  # Platform adapters (local/, kubernetes/); shared utils in utils/
  cli/          # Cobra command implementations
  mcp/          # MCP protocol handling
  daemon/       # Daemon/docker-compose orchestration
ui/             # Next.js frontend
```

---

## Critical Architecture Rules

**Database access** — only `internal/registry/database/` and the authorizer may query the DB directly. Services receive a repository interface via constructor injection.

**Platform ownership** — `internal/registry/platforms/<platform>/` owns all platform-specific logic. `platforms/utils/` is for narrowly shared helpers only. HTTP handlers in `api/handlers/` must not own deployment behavior.

**Composition root** — `internal/registry/registry_app.go` wires concrete adapters explicitly. Do not hide factory/registration logic inside handler packages.

---

## Go Code Style

**Imports** — use `goimports` grouping: stdlib / external / internal. Apply required aliases from `.golangci.yaml`:
```go
import (
    "context"
    "fmt"

    corev1 "k8s.io/api/core/v1"           // k8s aliases are mandatory
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    "github.com/agentregistry-dev/agentregistry/internal/registry/service"
    "github.com/agentregistry-dev/agentregistry/pkg/models"
)
```

**Naming** — `PascalCase` exports, `camelCase` locals, `kebab-case` CLI flags. Interface names are noun-based (`AgentRepository`, not `IAgentRepository`). Receiver names are short (1-2 chars), consistent per type.

**Error handling** — always wrap with context; use lowercase messages (they get wrapped again):
```go
return nil, fmt.Errorf("getting agent %s: %w", id, err)
```
Define sentinel errors for cases callers check: `var ErrNotFound = errors.New("not found")`. Check with `errors.Is`/`errors.As`.

**Logging** — use `log/slog` with structured key-value pairs:
```go
slog.Info("agent created", "agent_id", agent.ID, "name", agent.Name)
slog.Error("failed to deploy", "error", err, "agent", name)
```

**Interfaces** — define interfaces for every significant dependency to enable mocking. Accept interfaces, return concrete types from constructors.

**Dependency injection** — manual constructor injection only; no DI frameworks:
```go
func NewAgentService(repo AgentRepository) *AgentService {
    return &AgentService{repo: repo}
}
```

**File size** — keep files under 500 lines. Split on responsibility boundaries.

---

## Testing Patterns

Use `github.com/stretchr/testify/assert` and `require` (already a dependency). Prefer table-driven tests:
```go
//go:build unit

func TestFoo(t *testing.T) {
    tests := []struct{ name, input string; wantErr bool }{
        {"valid", "ok", false},
        {"empty", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Foo(tt.input)
            require.Equal(t, tt.wantErr, err != nil)
        })
    }
}
```

Write mock structs implementing the target interface; place them in a `testing/` sub-package (see `internal/registry/service/testing/`).

---

## CLI & API Conventions

- CLI commands use `cobra.Command.RunE` (return errors, don't `os.Exit`).
- Use `pkg/printer` for all user-facing output (`PrintSuccess`, `PrintInfo`, `PrintError`, `NewTablePrinter`).
- API handlers use Huma: define typed Input/Output structs with struct tags (`path:`, `query:`, `body:`), register with `huma.Get/Post/...`. Huma generates OpenAPI automatically.
- After changing the API, regenerate the spec: `make gen-openapi` and the TS client: `make gen-client`.

---

## Quick Reference

| Task | Command |
|---|---|
| Build CLI | `make build-cli` |
| Build server | `make build-server` |
| Unit tests | `make test-unit` |
| Integration tests | `make test` |
| Single test | `go test -tags=unit -run TestName ./path/...` |
| Lint (with fix) | `make lint` |
| UI lint | `make lint-ui` |
| Build UI | `make build-ui` |
| Dev UI | `make dev-ui` |
| Verify (CI check) | `make verify` |
| Docker up | `make docker-compose-up` |
| Docker down | `make docker-compose-down` |
| Daemon Start | `make daemon-start` |
| Daemon Stop | `make daemon-stop` |

---

## Related Documentation

- [README.md](./README.md) - Project overview and quick start
- [DEVELOPMENT.md](./DEVELOPMENT.md) - Architecture details
- [CONTRIBUTING.md](./CONTRIBUTING.md) - Contribution guidelines
