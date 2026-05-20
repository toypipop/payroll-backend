# AGENTS.md

Agent playbook for `payroll-backend`.
This repository is a Go (Gin + GORM + MySQL) backend for payroll, attendance, and reporting.
Follow these conventions when editing code, adding tests, or running local checks.
The companion React/Vite frontend lives next to this repo at `../payroll-frontend`.

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
- `migrations/*`: ad hoc SQL migrations that are not covered by GORM auto-migration.
- `config/database.go`: DB bootstrap and global `config.DB`.

## Environment Assumptions

- Go version from `go.mod`: `1.25.5`.
- Database: MySQL on `127.0.0.1:3306` by default.
- Default DB name in code and local `.env`: `PayrollDB`.
- App listens on port `8000`.
- Frontend CORS allowlist defaults to `http://localhost:5173`; production/custom origins can be set with `CORS_ALLOWED_ORIGINS` or `FRONTEND_ORIGIN`.
- Local config is loaded from `.env` with `godotenv`; important keys are `JWT_SECRET`, `DB_USER`, `DB_PASSWORD`, `DB_HOST`, `DB_PORT`, `DB_NAME`, and optionally `CORS_ALLOWED_ORIGINS`, `FRONTEND_ORIGIN`, `APP_ENV`, `GO_ENV`, or `GIN_MODE`.
- API base path is `/api/v1`; the frontend typically points `VITE_API_URL` to `http://localhost:8000/api/v1`.

## Build, Run, Lint, and Test Commands

Run from repository root.

### Local MySQL Docker

- Added May 20, 2026: `docker-compose.yml` is intentionally MySQL-only for local development; do not add frontend or backend containers unless explicitly requested.
- Start the local database from this backend repo with `docker compose up -d`.
- Check database status with `docker compose ps` and logs with `docker compose logs -f mysql`.
- The MySQL container is `payroll-mysql`, publishes `localhost:3306`, and creates `PayrollDB`.
- Local `.env` should use `DB_USER=root`, `DB_PASSWORD=1234`, `DB_HOST=127.0.0.1`, `DB_PORT=3306`, and `DB_NAME=PayrollDB`.
- Run the backend locally with `go run .`; it connects to the Docker MySQL database and runs migrations.
- Stop the database with `docker compose stop`; remove the container with `docker compose down`; remove all local MySQL data with `docker compose down -v`.
- If Docker commands fail with `dockerDesktopLinuxEngine` pipe errors, Docker Desktop is not running. Start it with `Start-Process "C:\Program Files\Docker\Docker\Docker Desktop.exe"` from PowerShell, wait until Docker is ready, then retry.

### Dependency and compile checks

- `go mod tidy` - clean and sync module dependencies.
- `go build ./...` - compile all packages.
- `go run .` - run API server from `main.go`.
- `go run ./seed` - run seed script (requires reachable MySQL).
- `go run .` also runs GORM `AutoMigrate` before starting the server.

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
- For API contract changes, check the matching frontend page/API usage in `../payroll-frontend/src` before finishing.

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
- Read path params, query params, and auth context using existing local patterns in nearby handlers.
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
- The project uses singular table names via `schema.NamingStrategy{SingularTable: true}`.
- Keep raw SQL migrations in `migrations/*` when a schema change cannot be represented safely by auto-migration alone.

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
- JWT auth is cookie-based. Do not return JWTs in response bodies and do not rely on `Authorization: Bearer` headers for browser auth.
- Login should set the JWT in the httpOnly `auth_token` cookie; logout should clear that cookie.
- The auth cookie must be `HttpOnly`, `Path=/`, have an expiration/max age, and use `Secure=true` in production. Localhost/dev may use `Secure=false`.
- Validate JWT signature, signing method, and expiration in `AuthMiddleware`; invalid or expired cookies must return `401`.
- Avoid returning raw internal errors (`err.Error()`) in API responses for operational/database failures.
- Avoid `config.DB.Debug()` in normal runtime code; reserve SQL debug logs for temporary troubleshooting.

### Local agent artifacts

- Keep local agent cache files out of source control (`.agents/`, `skills-lock.json`).
- If working on multiple machines, sync these artifacts privately (or reinstall locally) rather than committing them.

### Time and monetary values

- Keep timezone handling explicit and consistent.
- If changing payroll math, add targeted tests for boundary/rounding cases.

### Routes and permissions

- Register new endpoints in `routes/routes.go` under the correct domain block.
- Protect private endpoints with `AuthMiddleware()` and `CheckPermission(...)`.
- Keep module keys and action names consistent with seeded module access data.
- `POST /api/v1/auth/login` is public, sets the httpOnly auth cookie, and returns only non-sensitive user profile data.
- `GET /api/v1/auth/me` validates the auth cookie and returns non-sensitive user profile data for frontend route guards.
- `POST /api/v1/auth/logout` clears the auth cookie.
- `GET /api/v1/modules` is public and returns active `AppModule` rows sorted by `order`; the frontend uses it for routes, sidebar navigation, and breadcrumbs.
- Seeded `AppModule.ModuleKey` values must match frontend page filenames exactly, for example `EmployeesListPage` maps to `../payroll-frontend/src/pages/**/EmployeesListPage.jsx`.
- If adding or renaming modules, update `seed/main.go`, route permission keys, and the frontend page file/routes together.

### Frontend contract notes

- Companion frontend: `../payroll-frontend`, built with React 19, Vite, and React Router v7.
- Frontend API client: `../payroll-frontend/src/api/axiosInstance.js`.
- Frontend axios uses `withCredentials: true`; the browser sends the httpOnly auth cookie automatically.
- Frontend must not store JWTs in `localStorage` or `sessionStorage`; it may store only non-sensitive user display/profile data.
- 401 responses clear frontend profile state and redirect to `/login`.
- Keep response JSON shapes stable for existing pages unless the frontend is updated in the same change.

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
