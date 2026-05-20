# AGENTS.md

Compact playbook for `payroll-backend`, a Go Gin + GORM + MySQL API for payroll, attendance, settings, and reporting. The companion frontend is `../payroll-frontend`; check it whenever API contracts, routes, modules, auth responses, permissions, seed data, or response shapes change.

## Stack And Layout

- Go version from `go.mod`: `1.25.5`.
- API base path: `/api/v1`; app listens on port `8000`.
- Database: MySQL on `127.0.0.1:3306`, default DB `PayrollDB`.
- CORS defaults to `http://localhost:5173`; override with `CORS_ALLOWED_ORIGINS` or `FRONTEND_ORIGIN`.
- Config loads `.env` via `godotenv`; important keys are `JWT_SECRET`, DB settings, CORS settings, and optional environment mode keys.
- Key files:
- `main.go`: DB connection, migrations, HTTP server.
- `routes/routes.go`: route registration and middleware.
- `handlers/*`: domain HTTP handlers.
- `models/*`: GORM models and API structs.
- `repository/*`: query scopes and shared checks.
- `middleware/*`: auth, permission, delete guard middleware.
- `migrations/*`: SQL/GORM migrations not safely covered by auto-migration.
- `seed/main.go`: local/test data, modules, permissions.

## Commands

Run from the backend root.

- `docker compose up -d` - start local MySQL only.
- `docker compose ps` - check MySQL status.
- `docker compose logs -f mysql` - follow MySQL logs.
- `docker compose stop` - stop MySQL.
- `docker compose down` - remove container.
- `docker compose down -v` - remove container and local MySQL data.
- `go run .` - run API and AutoMigrate.
- `go run ./seed` - seed/migrate local data.
- `go build ./...` - compile all packages.
- `go test ./...` - run tests; current packages may report `[no test files]`.
- `go vet ./...` - static checks.
- `go fmt ./...` or `gofmt -w .` - formatting.

## Docker

- `docker-compose.yml` is intentionally MySQL-only for local development.
- Do not add frontend/backend containers unless explicitly requested.
- MySQL container: `payroll-mysql`.
- Published port: `localhost:3306`.
- Database: `PayrollDB`.
- Local `.env` should use:
- `DB_USER=root`
- `DB_PASSWORD=1234`
- `DB_HOST=127.0.0.1`
- `DB_PORT=3306`
- `DB_NAME=PayrollDB`
- If Docker commands fail with `dockerDesktopLinuxEngine` pipe errors, start Docker Desktop and retry.

## Workflow

- Prefer small, domain-scoped changes.
- Keep route/middleware ordering consistent with `routes/routes.go`.
- If behavior changes, add/update tests in the same package when practical.
- Run at least `go test ./...`, `go vet ./...`, and `go build ./...` before finishing when code changed.
- If database or seed behavior changes, sanity-check with `go run ./seed` against local MySQL when practical.
- For API contract changes, check matching frontend usage in `../payroll-frontend/src`.
- When adding/changing frontend-facing endpoints, verify every `api.get/post/put/patch/delete` path in frontend has a matching method/path in `routes/routes.go`.

## Go Style

- Use standard Go import grouping: stdlib, internal modules, third-party.
- Keep import aliases short and meaningful.
- Enforce `gofmt`; do not hand-align code.
- Prefer early returns for guards and errors.
- Use explicit structs for request/response payloads when practical.
- Use pointer fields for nullable DB columns.
- Keep GORM tags and JSON tags in sync with API contracts.
- Maintain existing `soft_delete.DeletedAt` patterns.
- Avoid ignored errors unless truly safe.

## Handlers And Errors

- Parse JSON with `ShouldBindJSON` and validate immediately.
- Read params/query/auth context using nearby handler patterns.
- Return JSON errors with stable keys, usually `{"error": "..."}`.
- Use correct status codes: `400`, `401`, `403`, `404`, `409`, `500`.
- Abort middleware chains after terminal errors with `c.Abort()`.
- Avoid leaking sensitive internals in API responses.
- Avoid noisy debug logs and `config.DB.Debug()` in normal runtime.

## Database And Scopes

- Use `config.DB` and existing repository scopes.
- Prefer `repository.ApplyContextScopes(c, currentUser, dataScope, ...)` in handlers so active role/company mappings are honored.
- Keep company/data-scope filtering consistent for multi-tenant safety.
- Preload only fields needed by the response.
- Put reusable cross-table checks in `repository/*`.
- Use transactions for multi-step writes that must be atomic.
- The project uses singular table names via GORM naming strategy.
- Qualify scoped columns with singular table names, for example `employment_type.companies_id`, `time_log.companies_id`, and `employee_company_access.company_id`.
- Keep raw SQL migrations in `migrations/*` when auto-migration cannot represent the change safely.

## Auth And Security

- JWT auth is cookie-based.
- Login: `POST /api/v1/auth/login`; set httpOnly `auth_token` cookie and return only non-sensitive profile data.
- Session check: `GET /api/v1/auth/me`.
- Logout: `POST /api/v1/auth/logout`; clear cookie.
- Do not return JWTs in response bodies.
- Do not rely on browser `Authorization: Bearer` headers.
- Cookie must be `HttpOnly`, `Path=/`, have expiration/max age, and use `Secure=true` in production. Localhost may use `Secure=false`.
- Validate JWT signature, signing method, and expiration in auth middleware.
- Never hardcode credentials, DSNs, JWT secrets, or production tokens.

## Routes, Modules, And Permissions

- Register endpoints in `routes/routes.go` under the correct domain block.
- Protect private endpoints with `AuthMiddleware()` and `CheckPermission(...)`.
- Keep action names consistent with seeded access rows.
- `/api/v1/modules` is public and returns active `AppModule` rows sorted by `order`; the frontend uses it for routes, sidebar, and breadcrumbs.
- Seeded `AppModule.ModuleKey` values must match frontend page filenames exactly.
- Seeded `AppModule.Icon` values must exist in `../payroll-frontend/src/components/common/Sidebar.tsx`; missing keys render the frontend fallback icon.
- If adding/renaming modules, update `seed/main.go`, route permission keys, frontend page files/routes, and sidebar icon map when needed.
- `EmployeeCompanyAccessPage` manages `employee_company_access`; keep seed module, routes, permission checks, and frontend page in sync.
- Company dropdown options are served by `GET /api/v1/companies/options`.
- Do not assume `GET /api/v1/companies` exists.
- `GET /api/v1/companies/all` is for the Companies settings list/index.
- Payroll config category routes are `/api/v1/master/earning-categories` and `/api/v1/master/deduction-categories`; keep them registered with `EarningCategoryPage` permissions and category delete guards.

## Company Access Model

- `employees.companies_id` is the employee's primary/home company for payroll, attendance, department, position, salary, and employment type logic.
- `employee_company_access` maps `employee_id`, `company_id`, and `role_id`, with `is_default` and `is_active`.
- `employee_company_access.company_id` controls app access; it does not change payroll ownership.
- Creating an employee should also create an active/default `EmployeeCompanyAccess` row for selected `companies_id` and `roles_id`.
- Add extra company access through `EmployeeCompanyAccessPage` and `/api/v1/employee-company-access`.
- Normally only one mapping per employee should have `is_default = true`; the access API clears other defaults.
- Create/update APIs accepting an employee id must accept or derive `companies_id`, validate requester access to that company, and reject employee/company mismatches.
- Employee-linked records should save `companies_id` when the model has that field.

## Frontend Contract

- Frontend API client: `../payroll-frontend/src/api/axiosInstance.ts`.
- Frontend axios uses `withCredentials: true`; browser sends the httpOnly cookie automatically.
- Frontend must not store JWTs; it may store non-sensitive display/profile data.
- Login and `/auth/me` responses include `roles`, default `companies_id`, and `accessible_company_ids`.
- Broad access comes from normal role/module/access rows plus `employee_company_access` mappings; there is no hardcoded privileged-user bypass.
- Seeded top-level role is `SYSTEM_ADMIN`.
- 401 responses should cause the frontend to clear profile state and redirect to `/login`.
- Keep response JSON shapes stable unless the frontend is updated in the same change.

## Testing Guidance

- Use table-driven tests for handlers and business logic.
- Prefer `httptest` for endpoints; assert status and JSON shape.
- Include allow/deny cases for permission-sensitive endpoints.
- Add boundary/rounding tests for payroll math changes.

## Artifacts

- Keep local agent artifacts out of source control: `.agents/`, `skills-lock.json`.
- Sync private agent artifacts outside git if needed.

## Definition Of Done

- Code is formatted.
- `go build ./...` passes.
- `go vet ./...` passes.
- `go test ./...` passes, or any inability to run it is explained.
- API behavior, status codes, permissions, and response shapes remain consistent.
- No secrets added; configuration remains environment-driven.
