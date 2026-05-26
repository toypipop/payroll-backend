# AGENTS.md

Compact playbook for `payroll-backend`, the Go 1.25.5 + Gin + GORM + Postgres API. Check `../payroll-frontend/AGENTS.md` when changing frontend-facing routes, modules, auth responses, permissions, seed data, or response shapes.

## Core

- API base path: `/api/v1`; server port: `8000`.
- CORS default: `http://localhost:5174`; override with `CORS_ALLOWED_ORIGINS` or `FRONTEND_ORIGIN`.
- Docker DB host is `postgres:5432`; host-run backend needs a host-reachable DB such as `127.0.0.1:5432`.
- Config loads `.env` via `godotenv`; never hardcode secrets.
- Seeded demo users use their email as the password; do not reintroduce seed-password env keys.

## Key Files

- `main.go`: DB connection, migrations, HTTP server.
- `routes/routes.go`: routes and middleware.
- `handlers/*`: domain handlers matching frontend modules.
- `models/payroll.go`: cycles, transactions, cycle earnings/deductions.
- `models/payrollconfig.go`: payroll config, earning/deduction types/categories, employment types.
- `models/employees.go`: employees, salary structures, employee earnings/deductions.
- `models/settings.go`: companies, departments, positions, banks, holidays, work locations, leave types.
- `repository/*`: query scopes and shared checks.
- `migrations/*`: raw/schema repairs not safely covered by AutoMigrate.
- `seed/main.go`: local/test data, modules, permissions.

## Commands

- Docker DB/full stack helpers: `docker compose up -d`, `docker compose down`, `docker compose down -v`.
- Docker tools: `docker compose --profile tools run --rm migrate`, `docker compose --profile tools run --rm seed`.
- Local run/tools: `go run .`, `go run ./migrate`, `go run ./seed`.
- Required after code edits: `gofmt -w <files>`, `go test ./...`, `go vet ./...`, `go build ./...`.

## Workflow

- Prefer small domain-scoped changes; keep route order consistent in `routes/routes.go`.
- Verify frontend-facing changes against `../payroll-frontend/src`.
- For DB/seed changes, sanity-check with `go run ./seed` when practical.
- After Docker volume reset, run migrate and seed before expecting `/api/v1/modules` or frontend healthchecks to pass.
- Use explicit request/response structs where practical; keep JSON/GORM tags aligned.
- Use `repository.ApplyContextScopes(c, currentUser, dataScope, ...)` for tenant/data-scope safety.
- Tables are singular; qualify scoped columns such as `time_log.companies_id`.
- Keep existing `soft_delete.DeletedAt` patterns unless hard delete is explicitly required.
- Do not add secrets or local agent artifacts such as `.agents/` or `skills-lock.json`.

## Auth/API

- Auth is cookie-based JWT via httpOnly `auth_token`; do not return JWTs in bodies or rely on browser `Authorization` headers.
- Login/check/logout: `POST /api/v1/auth/login`, `GET /api/v1/auth/me`, `POST /api/v1/auth/logout`.
- Protected endpoints need `AuthMiddleware()` and `CheckPermission(...)`.
- Return stable JSON errors, usually `{ "error": "..." }`, with correct status codes.

## Routes/Modules

- `/api/v1/modules` is public and drives frontend routes/sidebar/breadcrumbs.
- Seeded `AppModule.ModuleKey` must match frontend page filenames exactly.
- Seeded `AppModule.Icon` must exist in `../payroll-frontend/src/components/common/Sidebar.tsx`.
- Company options route: `GET /api/v1/companies/options`.
- Do not assume `GET /api/v1/companies` exists; `GET /api/v1/companies/all` is for the Companies list page.
- Payroll category APIs: `/api/v1/master/earning-categories`, `/api/v1/master/deduction-categories`.

## Business Rules

- `employees.companies_id` is home company for payroll, attendance, salary, department, position, and employment type.
- `employee_company_access.company_id` is app access only, not payroll ownership.
- Creating an employee should create an active/default `EmployeeCompanyAccess` row for the selected company/role.
- Employee-linked create/update APIs must validate requester access and reject employee/company mismatches.
- Time-log create/edit must derive or validate company from the selected employee.
- Attendance config work times are schedule times; end <= start means overnight, e.g. `16:00` to `00:00` checks out next day.
- Leave request dates are date-only values normalized to midnight UTC.
- Leave Requests: only `Pending` can edit; any accessible status can be soft-deleted with delete permission.
