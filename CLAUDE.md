Don't modify upstream files beyond what the task requires; document any you do in UPSTREAM-DELTA.md
Prefer seams over surgery — new behavior in new files, hooks over inline edits
Additive migrations only
Hide upstream functionality via config; don't delete it
Keep implementations minimal; no configurability or abstractions no requirement asks for

Run golangci-lint run and gofmt/goimports before proposing any change; the repo config is authoritative. Do not add linters, enable additional rules, or tighten existing settings — upstream's config is inherited, and diverging creates merge friction. Tests use testify (require/assert) per testifylint. Note nestif (avoid deep conditional nesting) and depguard (use slices, not sort).

Database tests require PostgreSQL on port 5432; they skip and report ok without it. ~19 tests in internal/registry/database and internal/registry/service fail against a live database due to fixtures that predate 0f6e5d2 (which emptied PublicActions, making auth mandatory): setup calls using context.Background() fail unauthenticated, and server fixtures without a repository fail validation. Use database.WithTestSession and a structurally valid non-existent repository URL.