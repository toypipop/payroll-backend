# AGENTS.md

Agent playbook for `payroll-backend`.
This repository is a Go (Gin + GORM + MySQL) backend for payroll, attendance, and reporting.
Follow these conventions when editing code, adding tests, or running local checks.

## Rule Sources

- Checked `.cursor/rules/`: not present.
- Checked `.cursorrules`: not present.
- Checked `.github/copilot-instructions.md`: not present.
- Therefore, this file is the canonical agent guidance in this repo.

## Project Layout

- `main.go`: app entrypoint, DB connection, auto-migration, HTTP server.
- `seed/main.go`: seed entrypoint (migrations + master/test data).
- `routes/routes.go`: all route registration and middleware wiring.
- `handlers/*`: HTTP handlers grouped by domain.
- `middleware/*`: auth, permission checks, delete guards.
- `models/*`: GORM models, enums, request/response structs.
- `repository/*`: query scopes and cross-entity helpers.
- `config/database.go`: DB bootstrap and global `config.DB`.

## Environment Assumptions

- Go version from `go.mod`: `1.25.5`.
- Database: MySQL on `127.0.0.1:3306`.
- Default DB name in code: `PayrollDB`.
- App listens on port `8000`.
- Frontend CORS allowlist includes `http://localhost:5173`.

## Build, Run, Lint, and Test Commands

Run from repository root.

### Dependency and compile checks

- `go mod tidy` - clean and sync module dependencies.
- `go build ./...` - compile all packages.
- `go run .` - run API server from `main.go`.
- `go run ./seed` - run seed script (requires reachable MySQL).

### Formatting and static analysis

- `gofmt -w .` - format all Go files.
- `go fmt ./...` - package-level formatting pass.
- `go vet ./...` - static checks (shadowing, printf, misuse patterns).

### Tests

- Current state: `go test ./...` reports `[no test files]` for all packages.
- Standard full suite command: `go test ./...`.
- Verbose suite: `go test -v ./...`.
- Race detector: `go test -race ./...` (use when adding concurrency-sensitive code).

### Run a single test (important)

- Single test in one package:
- `go test ./handlers/auth -run '^TestLogin$' -v`
- Single subtest:
- `go test ./handlers/auth -run '^TestLogin$/valid_credentials$' -v`
- Multiple named tests by regex:
- `go test ./handlers/payroll -run 'TestCalculate|TestCloseCycle' -v`
- If test file is in current package directory:
- `go test -run '^TestName$' -v`

## Development Workflow for Agents

- Prefer small, domain-scoped changes (handler + model + route as needed).
- Keep route/middleware ordering consistent with existing `routes/routes.go` style.
- If behavior changes, add/update tests in the same package as the changed code.
- Run at least: `go test ./...` and `go vet ./...` before finishing.
- If database logic changes, sanity-check with `go run ./seed` against local MySQL.

## Code Style Guidelines

### Imports

- Use standard Go import grouping:
- stdlib first, blank line, internal modules, blank line, third-party.
- Keep import aliases short and meaningful (`authhandler`, `sethandler`, etc.).
- Remove unused imports immediately; do not leave commented imports.

### Formatting

- Enforce `gofmt` formatting; do not hand-format alignment.
- Prefer early returns for guard clauses and error paths.

### Types and data modeling

- Use explicit structs for request/response payloads when practical.
- Use pointer fields (`*uint`, `*time.Time`) for nullable DB columns.
- Keep GORM tags and JSON tags in sync with API contract.
- Maintain existing soft-delete patterns (`soft_delete.DeletedAt`).

### Naming conventions

- Package names: lowercase, single word where possible.
- Exported identifiers: `PascalCase`.
- Unexported identifiers: `camelCase`.
- Avoid all-caps local vars; prefer `employee` over `Employees` as local variable names.
- Keep names domain-specific (`companiesID`, `cycleID`, `dataScope`).

### Handler conventions (Gin)

- Parse inputs with `ShouldBindJSON` and validate immediately.
- Return JSON errors with stable keys, primarily `{"error": "..."}`.
- Use correct HTTP status codes:
- `400` bad request, `401` unauthorized, `403` forbidden, `404` not found,
- `409` conflict, `500` internal server error.
- Abort middleware chains after writing terminal errors (`c.Abort()`).

### Database and repository conventions

- Use `config.DB` and existing repository scopes (`ApplyScopes`, `ScopeCompanies`).
- Keep company/data-scope filtering consistent for multi-tenant safety.
- Prefer `Preload` only for fields actually needed by the response.
- For cross-table checks, place reusable logic in `repository/*`.
- Use transactions for multi-step writes that must be atomic.

### Error handling

- Always handle returned errors; do not ignore with `_` unless truly safe.
- Avoid leaking sensitive internals in API responses.
- For user-facing errors, prefer concise and actionable messages.

### Logging

- Use logs for operational context, not for sensitive payloads.
- Avoid noisy debug logging in hot paths unless actively troubleshooting.

### Security and secrets

- Never hardcode credentials, DSNs, or JWT secrets in new code.
- Prefer environment variables for all secrets and environment-specific values.
- Do not commit real credentials or tokens.

### Time and monetary values

- Keep timezone handling explicit and consistent.
- If changing payroll math, add targeted tests for boundary/rounding cases.

### Routes and permissions

- Register new endpoints in `routes/routes.go` under the correct domain block.
- Protect private endpoints with `AuthMiddleware()` and `CheckPermission(...)`.
- Keep module keys and action names consistent with seeded module access data.

## Testing Guidance to Add New Tests

- Use table-driven tests for handlers and business logic.
- Prefer `httptest` for endpoint tests and assert status + JSON shape.
- For permission-sensitive endpoints, include allow/deny scenarios.
- For payroll/attendance logic, test date-window and scope edge cases.

## Definition of Done (for agent changes)

- Code is formatted (`gofmt`) and compiles (`go build ./...`).
- Static checks pass (`go vet ./...`).
- Tests pass (`go test ./...`), including any newly added tests.
- API behavior, status codes, and permission checks remain consistent.
- No secrets added; configuration remains environment-driven.
