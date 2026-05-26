# AGENTS.md

Compact playbook for `payroll-backend`, a Go Gin + GORM + Postgres API. The companion frontend is `../payroll-frontend`; check it for API contracts, routes, modules, auth responses, permissions, seed data, or response-shape changes.

## Stack

- Go `1.25.5`, Gin, GORM, Postgres.
- API base path: `/api/v1`; server port: `8000`.
- CORS default: `http://localhost:5174`; override with `CORS_ALLOWED_ORIGINS` or `FRONTEND_ORIGIN`.
- Local DB: `postgres` / `1234` on `127.0.0.1:5432`, database `payrolldb`.
- Config loads `.env` via `godotenv`; never hardcode secrets.

## Layout

- `main.go`: DB connection, migrations, HTTP server.
- `routes/routes.go`: routes and middleware.
- `handlers/*`: domain handlers matching frontend modules.
- `models/payroll.go`: payroll cycles, transactions, cycle earnings/deductions.
- `models/payrollconfig.go`: payroll config, earning/deduction types/categories, employment types.
- `models/employees.go`: employee profile, salary structure, employee-level earnings/deductions.
- `models/settings.go`: companies, departments, positions, banks, holidays, work locations, leave types.
- `repository/*`: query scopes and shared checks.
- `migrations/*`: raw/schema repair migrations not safely covered by AutoMigrate.
- `seed/main.go`: local/test data, modules, permissions.

## Commands

- `docker compose up -d` - start local Postgres.
- `docker compose down` / `docker compose down -v` - stop/remove DB container/data.
- `go run .` - run API and repair migrations.
- `go run ./migrate` - run repair migrations and AutoMigrate.
- `go run ./seed` - seed local data.
- `go test ./...`, `go vet ./...`, `go build ./...` - required checks after code changes.
- `gofmt -w <files>` - format touched Go files.

## Workflow

- Prefer small domain-scoped changes; keep route order consistent in `routes/routes.go`.
- For frontend-facing changes, verify matching frontend paths in `../payroll-frontend/src`.
- For DB/seed changes, sanity-check with `go run ./seed` when practical.
- Use explicit request/response structs where practical; keep JSON/GORM tags in sync.
- Use `repository.ApplyContextScopes(c, currentUser, dataScope, ...)` for tenant/data-scope safety.
- The project uses singular table names; qualify scoped columns like `time_log.companies_id`.
- Keep existing `soft_delete.DeletedAt` patterns unless a hard delete is explicitly required.

## Auth And API

- Auth is cookie-based JWT via httpOnly `auth_token`.
- Login: `POST /api/v1/auth/login`; session check: `GET /api/v1/auth/me`; logout: `POST /api/v1/auth/logout`.
- Do not return JWTs in bodies or rely on browser `Authorization` headers.
- Protected endpoints need `AuthMiddleware()` and `CheckPermission(...)`.
- Return stable JSON errors, usually `{"error": "..."}`, with correct status codes.

## Routes And Modules

- `/api/v1/modules` is public and drives frontend routes/sidebar/breadcrumbs.
- Seeded `AppModule.ModuleKey` must match frontend page filenames exactly.
- Seeded `AppModule.Icon` must exist in `../payroll-frontend/src/components/common/Sidebar.tsx`.
- Company options route is `GET /api/v1/companies/options`.
- Do not assume `GET /api/v1/companies` exists; `GET /api/v1/companies/all` is for the Companies list page.
- Payroll category APIs are `/api/v1/master/earning-categories` and `/api/v1/master/deduction-categories`.

## Business Rules

- `employees.companies_id` is the employee home company for payroll, attendance, salary, department, position, and employment type.
- `employee_company_access.company_id` is app access only, not payroll ownership.
- Creating an employee should also create an active/default `EmployeeCompanyAccess` row for the selected company/role.
- Employee-linked create/update APIs must validate requester access and reject employee/company mismatches.
- Time-log create/edit must derive or validate company from the selected employee.
- Attendance config work times are schedule times. End time <= start time means overnight, e.g. `16:00` to `00:00` checks out next day.
- Leave request dates are date-only values normalized to midnight UTC.
- Leave Requests: only `Pending` can edit; any accessible status can be soft-deleted with delete permission.

## Definition Of Done

- `gofmt` applied.
- `go test ./...`, `go vet ./...`, and `go build ./...` pass or blockers are explained.
- API behavior, status codes, permissions, and response shapes stay consistent.
- No secrets or local agent artifacts (`.agents/`, `skills-lock.json`) added.
