---
name: go-backend
description: Go backend conventions for PromptTree — errors, context, SQL with pgx, transactions, tests against a real database. Use for changes in Go services.
when: any task that edits .go files
roles: [tech_lead, junior]
projects: [go]
version: "1"
---

# Go backend

## Must
- `gofmt`, `go vet ./...`, `go test ./...`, `go build ./...` pass — the orchestrator runs exactly these.
- Errors are returned, wrapped with context (`fmt.Errorf("load user: %w", err)`), and checked with `errors.Is/As`. No `panic` for expected failures.
- Every function that does I/O takes `ctx context.Context` first and respects cancellation.
- SQL: parameters only (`$1`), never string concatenation. Multi-step changes run in one transaction; `defer tx.Rollback(ctx)` right after `Begin`.
- Lock rows you are about to change (`SELECT … FOR UPDATE`) when concurrent requests can race.

## Structure
- Follow the package boundaries of the repository (and `architecture.yaml` if present). HTTP handlers stay thin: decode, call the service, map errors.
- Domain errors are package-level variables (`ErrNotFound`, `ErrInvalid`); handlers map them to HTTP codes in one place.

## Tests
- Table-driven tests for pure logic; database tests use the project's `testdb` helper and a throwaway database.
- Test the race or the failure you fixed, not only the happy path.

## Do not
- Add modules to `go.mod` unless the task explicitly asks — that needs the owner's approval.
- Log secrets, tokens or full request bodies.
