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
- `timeutil/timeutil.go` — the ONLY source of truth for the business timezone (see Timezone Handling below).
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

### Master-data `/options` convention

Any master-data type consumed by a page other than its own "owning" settings page (a cross-page dropdown) needs a dedicated, **ungated** `GET .../options` sibling endpoint — do not loosen `CheckPermission` on the main CRUD endpoint instead. Pattern (mirrors `GetCompanies`/`GetCompaniesOptions`):
- No `middleware.CheckPermission(...)` on the route.
- Scoped only by `repository.ApplyContextScopes(...)` (company data-access), never by page permission.
- Returns minimal fields only (`id`, `name`, `companies_id` — never sensitive/rate fields).
- Existing examples: `/departments/options`, `/positions/options`, `/roles/options`, `/master/employment-types/options`, `/earning-types/options`, `/deduction-types/options`, `/master/leave-types/options`, `/master/banks/options`, `/master/work-locations/options`, `/master/day-offs/options`.
- Exception: if the underlying model has genuinely sensitive fields (e.g. `AttendanceConfig` payroll rates), build a separate purpose-built endpoint instead of a generic `/options` list (see `GetTimelogScheduleConfig`).

## Business Rules

- `employees.companies_id` = home company for payroll, attendance, salary, department, position, employment type.
- `employee_company_access.company_id` = app access only, not payroll ownership.
- Creating an employee must create an active/default `EmployeeCompanyAccess` row for the selected company/role.
- Employee-linked APIs must validate requester access and reject employee/company mismatches.
- Time-log create/edit must derive or validate company from the selected employee.
- Attendance config: end <= start means overnight, e.g. `16:00` → `00:00` checks out next day.
- Leave request dates are date-only, normalized to midnight UTC.
- Leave Requests: only `Pending` can edit; any accessible status can be soft-deleted with delete permission.
- **`companies_id` is immutable after creation** on every model (Employees, Departments, Positions, Roles, EarningType, DeductionType, DayOff, ...). Update handlers must never write `companies_id` from request input — re-derive/ignore it. Create handlers are the only place it's settable.
- `SalariesStructure` edits are blocked if `effective_date` falls inside an already-closed `PayrollCycle` for that employee's company (see `UpdateSalariesStructure`).
- `SalariesStructure.EffectiveDate` must not move backward from the employee's most recent record (active or not) — enforced in `UpdateSalariesStructure` so salary history always stays chronological.
- `runPayrollSnapshot` only applies a salary record to a cycle if `effective_date <= cycle.StartDate` — a raise effective mid-cycle takes effect starting the *next* cycle, never retroactively within the current one. If no record covers the cycle's start at all (a brand-new hire whose first record lands mid-cycle), it falls back to the first record with `effective_date <= cycle.EndDate` and prorates `BaseSalaries` by calendar days worked ÷ total days in the cycle.
- `SalariesStructure` has three real states, not two: `IsActive=true` (the current record), `IsActive=false, DeletedAt=0` (superseded but *was* used in a real `PayrollTransaction` — kept forever for audit, never hard-deleted), and `DeletedAt!=0` (never used in any calculation — safe to erase). `archiveSalaryRecord` (in `salaries_handler.go`) decides which by checking `PayrollTransaction.SalariesStructureID` for any reference to the record being replaced/disabled. Any query that needs "the current salary" must filter `is_active = true` explicitly (e.g. `Preload("SalariesStructure", "is_active = ?", true)`) — soft-delete alone no longer means "not current".
- `PayrollTransaction` snapshots `PositionName`/`DepartmentName`/`EmploymentTypeName`/`BankName`/`BankAccountName`/`BankAccountNo` at calculation time (`runPayrollSnapshot`) so closed-cycle payslips/reports stay accurate even if the employee's position/department/bank changes later. Any new closed-cycle report must read these snapshot fields, not `PayrollTransaction.Employees.*` (which reflects the employee's *current* state).
- `Employees.BankName` does not exist — bank name is always resolved live via the `Bank` relation (`Preload("Bank")`), except in the payroll snapshot above.

## Timezone Handling

The business timezone is fixed to **Asia/Bangkok (UTC+7, no DST)** — see `timeutil/timeutil.go`. Never use `time.Now().Location()`, a bare `.UTC()`, or a DB/driver's ambient session timezone to read or write a wall-clock time-of-day value (`AttendanceConfig.WorkStartTime/WorkEndTime`, `TimeLog.CheckIn/CheckOut`, or anything grouped/compared by calendar day). Always go through `timeutil.BangkokLocation` / `timeutil.ToBangkok(t)` / `timeutil.DayStart(t)` / `timeutil.DateKey(t)`. A `*time.Time` read back from Postgres carries whatever timezone the session/driver attached, not what was originally typed — extracting `.Hour()`/`.Year()`/`.Weekday()` etc. directly without normalizing through `timeutil` first is the exact bug class that shifted schedule times and misattributed late-night check-ins to the wrong day. `CreatedAt`/`UpdatedAt` and pure calendar-date fields (`Holiday.Date`, `PayrollCycle.Start/EndDate`, `LeaveRequest.Start/EndDate`, `SalariesStructure.EffectiveDate`) are NOT subject to this — they're either audit instants (must stay real time, compared against real "now") or date-only values with no time-of-day component.
