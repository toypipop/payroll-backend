# CLAUDE.md — payroll-backend

Playbook for `payroll-backend`, Go 1.25+ + Fiber/Gin + GORM + PostgreSQL.
Check `../payroll-frontend/CLAUDE.md` when changing frontend-facing routes, modules, auth responses, permissions, seed data, or response shapes.

## Core

- API base path: `/api/v1`; server port: `8000`.
- CORS default: `http://localhost:5174`; override with `CORS_ALLOWED_ORIGINS` or `FRONTEND_ORIGIN`.
- Docker DB host: `postgres:5432`; host-run backend needs `127.0.0.1:5432`.
- Config loads `.env` via `godotenv`; never hardcode secrets.
- Seeded demo users use their email as the password.

## Key Files

- `main.go` — DB connection, migrations, HTTP server.
- `routes/routes.go` — routes and middleware.
- `handlers/*` — domain handlers matching frontend modules.
- `models/payroll.go` — cycles, transactions, cycle earnings/deductions.
- `models/payrollconfig.go` — payroll config, earning/deduction types/categories, employment types.
- `models/employees.go` — employees, salary structures, employee earnings/deductions.
- `models/settings.go` — companies, departments, positions, banks, holidays, work locations, leave types.
- `repository/*` — query scopes and shared checks.
- `migrations/*` — raw/schema repairs not safely covered by AutoMigrate.
- `seed/main.go` — local/test data, modules, permissions.

## Commands

```bash
go run .                            # run server
go run ./migrate                    # run migrations
go run ./seed                       # reseed data
gofmt -w <files>                    # required after edits
go vet ./... && go test ./...       # required after edits
go build ./...                      # verify build
docker compose -f docker-compose.local.yml up -d    # local Docker
docker compose -f docker-compose.uat.yml up -d      # UAT Docker
```

## Workflow

- Small domain-scoped changes; keep route order consistent in `routes/routes.go`.
- Verify frontend-facing changes against `../payroll-frontend/src`.
- After Docker volume reset: run migrate and seed before expecting `/api/v1/modules` to pass.
- Use explicit request/response structs; keep JSON/GORM tags aligned.
- Use `repository.ApplyContextScopes(c, currentUser, dataScope, ...)` for tenant/data-scope safety.
- Tables are singular; qualify scoped columns e.g. `time_log.companies_id`.
- Keep `soft_delete.DeletedAt` patterns unless hard delete is explicitly required.
- Do not add secrets or local agent artifacts.

## Auth/API

- Cookie-based JWT via httpOnly `auth_token` — never return JWTs in bodies.
- Login / check / logout: `POST /api/v1/auth/login`, `GET /api/v1/auth/me`, `POST /api/v1/auth/logout`.
- Protected endpoints need `AuthMiddleware()` + `CheckPermission(...)`.
- Return stable JSON errors `{ "error": "..." }` with correct status codes.

## Routes/Modules

- `GET /api/v1/modules` is public and drives frontend routes/sidebar/breadcrumbs.
- Seeded `AppModule.ModuleKey` must match frontend page filenames exactly.
- Seeded `AppModule.Icon` must exist in `../payroll-frontend/src/components/common/Sidebar.tsx`.
- Company options: `GET /api/v1/companies/options` (not `/companies`).
- Companies list: `GET /api/v1/companies/all` (not `/companies`).
- Payroll categories: `/api/v1/master/earning-categories`, `/api/v1/master/deduction-categories`.

## Business Rules

- `employees.companies_id` = home company for payroll, attendance, salary, department, position, employment type.
- `employee_company_access.company_id` = app access only, not payroll ownership.
- Creating an employee must create an active/default `EmployeeCompanyAccess` row for the selected company/role.
- Employee-linked APIs must validate requester access and reject employee/company mismatches.
- Time-log create/edit must derive or validate company from the selected employee.
- Attendance config: end <= start means overnight, e.g. `16:00` → `00:00` checks out next day.
- Leave request dates are date-only, normalized to midnight UTC.
- Leave Requests: only `Pending` can edit; any accessible status can be soft-deleted with delete permission.
