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
- `repository/*` — query scopes, shared checks, and per-domain persistence interfaces.
- `service/*` — domain business rules; depends on repository interfaces, never on `config.DB` or gin.
- `apperr/apperr.go` — the error-kind vocabulary shared by every layer; the kind decides the HTTP status.
- `handlers/httpx/httpx.go` — shared handler plumbing (`RequestScope`, `ParseID`, `BindJSON`, `RespondError`).
- `docs/service-layer-refactor.md` — progress checklist for migrating the remaining handlers off `config.DB`.
- `timeutil/timeutil.go` — the ONLY source of truth for the business timezone (see Timezone Handling below).
- `migrations/*` — raw/schema repairs not safely covered by AutoMigrate.
- `seed/main.go` — local/test data, modules, permissions.

## Commands

```bash
go run .                            # run server
go run ./migrate                    # run migrations
go run ./seed                       # reseed data
gofmt -w <files>                    # required after edits
go vet ./... && go build ./...      # required after edits
go test ./...                       # required after touching service/
docker compose -f docker-compose.local.yml up -d    # local Docker
docker compose -f docker-compose.uat.yml up -d      # UAT Docker
```

## CI

`.github/workflows/ci.yml` runs on every push and pull request. Beyond `go vet`, `go build`, `go test` and a
`gofmt` check, it fails the build when a file under `handlers/`, `middleware/`, `repository/`, `routes/` or
`service/` imports `config`, or when `repository/` imports gin. Those two greps are what keeps the layering
rules below from being prose nobody runs — deploy is manual and has no gate of its own.

## Deploy

- `.github/workflows/deploy.yml` (backend repo only) deploys **both** repos — it SSHes in and pulls `~/payroll-backend` and `~/payroll-frontend` at the same branch, then builds from the backend dir.
- Manual `workflow_dispatch` only; no auto-deploy on push. No CI gate before deploy.
- Every stage (uat/sit/prod) ships from `main`. The `branch` input exists for ad-hoc branch deploys but defaults to `main`.
- The GitHub Environment is named `server` because it holds only SSH creds (`SERVER_HOST`, `SERVER_USER`, `SERVER_SSH_KEY`) — it identifies the target machine, not an app stage. Real env config lives in `.env` on the server.
- Compose file is hardcoded to `docker-compose.uat.yml`; do not derive it from the environment name.

## Workflow

- Small domain-scoped changes; keep route order consistent in `routes/routes.go`.
- Verify frontend-facing changes against `../payroll-frontend/src`.
- After Docker volume reset: run migrate and seed before expecting `/api/v1/modules` to pass.
- Use explicit request/response structs; keep JSON/GORM tags aligned.
- Row-level scoping has exactly **two** entry points, and choosing between them is a compile-time decision:
  - `repository.ApplyCompanyScopes(c, currentUser, companiesCol)` — the table has no employee column (payroll cycles, holidays, master data, departments/positions/roles). Takes **no** `DataScope`, because none would change the result.
  - `repository.ApplyEmployeeScopes(c, currentUser, dataScope, companiesCol, employeeCol)` — the table has a real employee column, so `Own`/`Department` are honoured.
  - `AccessScope` equivalents: `scope.ApplyCompany(...)` / `scope.ApplyEmployee(...)`.
- There is no `ApplyContextScopes` and no `"unused"` sentinel any more. The old single function decided at runtime by string-comparing the access column, so a typo compiled fine and a non-employee column (`department.id` vs an employee id) silently filtered on nonsense.
- There is no `ApplyScopes` either — it scoped by *reachable* companies and skipped company filtering entirely at `ScopeAll`, instead of the companies the role authorizes for the action.
- Both entry points **deny everything** when the authorized company list is empty (`applyCompanyFilter` adds `1 = 0`), matching `ScopeCompanyIDs` and the `ScopeDepartment` branch of `scopeDataAccess`. Three guards mean nothing reaches it empty today — `AuthMiddleware` 403s on empty `AccessibleCompanyIDs`, `CheckPermission` 403s unless it resolved a company, and an FK to `companies(id)` makes `companies_id = 0` unstorable — so this is fail-closed defence, not a hole being patched. Do not "simplify" it back to returning the query unfiltered.
- `ScopeDepartment` matches on `department.category_id`, never `employees.dept_id`: two companies' "HR" departments share one `DepartmentCategory` so a Department-scoped role reaches its counterparts across authorized companies.

### Module capabilities (what the permission UI is allowed to offer)

- `models/module_capability.go` holds `moduleCapabilities`, the **single source of truth** for each module's `ScopeLevel` (`company`/`employee`), `AllowedActions`, and `ActionsAtomic`. `AllowedScopes` is derived from `ScopeLevel`, never declared separately.
- `ScopeLevel` is a fact about the handler, not a policy: it must match which `Apply*Scopes` the module's queries call. `AllowedActions` must match the action strings its routes pass to `CheckPermission` — several delete routes are gated by `"EDIT"`, so those modules do not list `DELETE`.
- `migrations.SyncModuleCapabilities` projects the registry onto `app_module` columns on **every boot** (and at the end of seed), so the DB can never drift from the code and a deploy never depends on reseeding. An active, non-category module missing from the registry is a `log.Fatal`.
- `migrations.PruneStaleRoleScopes` rewrites saved permission rows whose `DataScope` a company-level module cannot honour. It only makes stored values agree with behaviour already in effect; no role gains or loses access.
- `service.PermissionService.Replace` coerces disallowed actions to `false` and disallowed scopes to `Companies` before writing, so the database never stores a claim the permission check ignores. It validates the whole payload *before* opening the transaction, so a rejected payload cannot clear a role's existing rows even transiently.
- The frontend permission matrix is driven entirely by `allowed_actions` / `allowed_scopes` / `actions_atomic` on `/modules`. Never reintroduce a hardcoded module list there.

- Tables are singular; qualify scoped columns e.g. `time_log.companies_id`.
- Keep `soft_delete.DeletedAt` patterns unless hard delete is explicitly required.
- Do not add secrets or local agent artifacts.

### Layering (`employees` domain is the reference implementation)

New or refactored domains follow handler → service → repository:

- `repository.AccessScope` (`repository/access_scope.go`) carries the resolved auth context (user, DataScope, authorized company IDs) as a plain value. Handlers get it from `httpx.RequestScope(c)`, which reads what `Guard.Auth` and `CheckPermission` already stored on the request — `repository.NewAccessScope` itself takes plain values and issues no query. Use `scope.ApplyCompany(companiesCol)` / `scope.ApplyEmployee(companiesCol, employeeCol)`, `scope.ResolveCompanyID(...)`, `scope.CanAccessCompany(...)` below the handler.
- `repository/<domain>_repository.go` declares an interface plus a `gorm*` implementation. Services depend on the interface so business rules are testable without PostgreSQL.
- `service/<domain>_service.go` holds the rules and returns typed sentinel errors (`ErrCompanyAccessDenied`, `ErrRoleCompanyMismatch`, ...). It must not import gin or know about HTTP status codes.
- Domain sentinel errors are built from `apperr.Validation/NotFound/Forbidden/Conflict/Unauthorized`, so the error's *kind* selects the status. `handlers/httpx.RespondError` is the **only** place in the codebase that maps an error to a status code — never write a per-domain `switch`. Anything not authored through `apperr` is `KindInternal` → 500 with a generic message, which is what keeps driver and SQL text from reaching the client.
- Handlers only bind input, call the service, and respond. Use `httpx.RequestScope` / `httpx.ParseID` / `httpx.BindJSON` / `httpx.RespondError` directly rather than re-wrapping them per package. Never return `err.Error()` to the client.
- A domain's repository must use the filter its `models/module_capability.go` entry declares: `ScopeLevel == "employee"` ⟺ `scope.ApplyEmployee`, `"company"` ⟺ `scope.ApplyCompany`. That pair is the check that catches a cross-company data leak, which `go build` cannot.
- `gofmt -w` rewrites line endings on every file it reads (the tree is checked out CRLF), so scope it to the directories you edited instead of the whole repo.
- Use `repository.ClearField` in an update map to null a nullable column; the repository translates it to SQL `NULL` so GORM clause types stay out of services.
- Because services depend on the repository *interface*, a domain is testable with a hand-written fake and no database — see `service/payroll_run_service_test.go`, whose `fakeRunRepo` is the pattern to copy. The payroll figures there are pinned to hand-computed values, so a failing assert means the calculation changed, not that the test is stale.
- `service/payroll_run_service.go` splits the run into a `runContext` (company-wide inputs resolved once per cycle) and per-employee steps (`earningInputs`, `deductionInputs`, `attendanceFor`, `leaveFor`, `absenceFor`, then `statutoryFor`). Additional earnings must keep folding onto `runBases` in listed order — the float accumulation order is part of the stored figures.
- **Every handler is migrated.** No file under `handlers/` imports `config`, and the `*gin.Context` scope helpers (`ApplyCompanyScopes`, `ApplyEmployeeScopes`, `ResolveCompanyID`, `CanAccessCompany`, `ResolveEmployeeForCompany`, `EnsurePayrollCycleOpen`) are deleted. `handlers/httpx` (`RequestScope`, `ContextAuth`, `AuthorizedCompanyIDs`) is the only place a request object is unwrapped — `repository/` does not import gin at all. A new handler that reaches for `config.DB` is a regression, not a shortcut.
- **`wire.go` is the composition root.** `buildHandlers(db)` builds every repository, service and handler once and `buildGuard(db)` builds the middleware; `SetupRouter(h, g)` only wires them. `main.go` and `wire.go` are the only files outside `config`, `migrate/` and `seed/` that name `config.DB`. Repository constructors take the connection (`repository.NewXxxRepository(db)`), handlers are methods on a per-package `Handler` struct whose fields are the services they use, and routes read `h.<Package>.<Method>` with `g.CheckPermission(...)` gates. A new page therefore means: one field on that package's `Handler`, one assignment in `wire.go`, one route. Checkable with `grep -rln '"payroll-backend/config"' handlers/ middleware/ repository/ routes/ service/` returning nothing.
- **`middleware.Guard`** (`NewGuard(accessRepo, deleteGuardRepo)`) owns `Auth`, `CheckPermission`, `CheckPermissionAny` and `PreventDeleteIfReferenced`. `Guard.Auth` stores `scope_user` (id, dept_id, companies_id) on the request so `httpx.RequestScope` builds an `AccessScope` without a query of its own; `CheckPermission` and `CheckPermissionAny` share one `checkModuleAccess` body, differing only in how many module keys they accept (OR across the list, never AND). Stateless middleware (`RequireXMLHttpRequest`, CSRF) stays package-level.

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

### Seeded roles & permission matrix

- Six roles: `SYSTEM_ADMIN` (sysComp, cross-company, only holder of `CompaniesPage`) + five business roles seeded per operating company — `ADMIN`, `HR`, `FINANCE`, `MANAGER`, `STAFF`.
- Grants live in one explicit matrix in `seed/main.go` (`sysAdminGrants`/`adminGrants`/`hrGrants`/`financeGrants`/`managerGrants`/`staffGrants`), least-privilege: a module **not listed** for a role is invisible to it (`/modules/mine` hides it from sidebar+routes, not just 403). Approve actions (leave/time logs) = `CanEdit`; there is no approve flag. `grantAccess` `log.Fatalf`s on any key missing from the module list, so a typo fails the seed loudly.
- Data scope per role: ADMIN/HR/FINANCE = `Companies`, MANAGER = `Department` (own team), STAFF = `Own` (self). STAFF-visible modules are all `Own`-scoped.
- All five report modules (`PayslipsReportPage`, `BankTransferReportPage`, `TaxFormsReportPage`, `SSOReportPage`, `PayrollAnalysisPage`) live under the `Reports` category and are GET-only → declared `companyLevel(ActionView)`, so the matrix shows a View toggle and a fixed `Companies` scope, with C/E/D hidden.
- `CompaniesPage`, `MasterBankPage` and `HelpPage` are declared `atomic(...)`: one on/off toggle driving every action, no scope selector. Seeded full CRUD=true so the toggle reads "on".
- Reports cannot be `Own`-scoped — they are company-level, so the selector is not offered at all. STAFF gets no reports; staff see own pay via Own-scoped Salaries / Employee Earnings / Employee Deductions.
- Sanity check that the registry is right: every module STAFF holds at `Own` scope, and every module MANAGER holds at `Department`, must be declared `employeeLevel`. If one is `companyLevel`, either the grant or the declaration is wrong.
- **Dashboard and My Payslips are not modules and have no permission rows.** Both are universal pages whose routes are hardcoded in `App.tsx` (and `"/"` redirects to `/dashboard`), with hardcoded sidebar links. A module row for Dashboard only ever hid the sidebar link while the page stayed reachable, so it was removed — `migrations.RemoveDashboardModule` clears it from existing databases and must run before `SyncModuleCapabilities`. What a user sees on the Dashboard is governed by the modules it reads from (`EmployeesListPage`, `PayrollCyclesPage`), each enforcing its own scope.

### Master-data `/options` convention

Any master-data type consumed by a page other than its own "owning" settings page (a cross-page dropdown) needs a dedicated, **ungated** `GET .../options` sibling endpoint — do not loosen `CheckPermission` on the main CRUD endpoint instead. Pattern (mirrors `GetCompanies`/`GetCompaniesOptions`):
- No `middleware.CheckPermission(...)` on the route.
- Scoped only by company data-access (`scope.ApplyCompany(...)`, or the legacy `repository.ApplyCompanyScopes(...)` in a not-yet-migrated handler), never by page permission.
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
- Lateness is deducted **per day, capped per day** at `AbsentRatePerDay` — a day of lateness must never cost more than a day of absence, but a cycle of daily lateness must still cost more than one late day. `runAbsence.lateMinutesPerDay` keeps one entry per late day for exactly this reason; never collapse it into a single total and cap that. `seed/main.go` mirrors the same rule in `seedLateDeduction` — it prices demo transactions itself instead of calling the engine, so a change to either late/absent/overtime pricing has to land in both.
- Seeded payroll figures must be priced from the `seed*RatePerMinute` / `seed*RatePerDay` constants at the top of `seed/main.go`, which are also what the seeded `AttendanceConfig` rows carry. Hardcoding a rate in the transaction loop is how the demo data drifted out of agreement with its own config (absent was billed at 1100 against a configured 300).
- A salary lookup that fails is not an employee to skip. `salaryForCycle` treats only `repository.ErrPayrollSalaryMissing` as "no salary"; any other error aborts that employee and surfaces from the run. Skipping on a database error would drop the employee from the cycle *and* delete the transaction an earlier run wrote.
- `PayrollRunService.Snapshot` only applies a salary record to a cycle if `effective_date <= cycle.StartDate` — a raise effective mid-cycle takes effect starting the *next* cycle, never retroactively within the current one. If no record covers the cycle's start at all (a brand-new hire whose first record lands mid-cycle), it falls back to the first record with `effective_date <= cycle.EndDate` and prorates `BaseSalaries` by calendar days worked ÷ total days in the cycle.
- `SalariesStructure` has three real states, not two: `IsActive=true` (the current record), `IsActive=false, DeletedAt=0` (superseded but *was* used in a real `PayrollTransaction` — kept forever for audit, never hard-deleted), and `DeletedAt!=0` (never used in any calculation — safe to erase). `archiveSalaryRecord` (in `salaries_handler.go`) decides which by checking `PayrollTransaction.SalariesStructureID` for any reference to the record being replaced/disabled. Any query that needs "the current salary" must filter `is_active = true` explicitly (e.g. `Preload("SalariesStructure", "is_active = ?", true)`) — soft-delete alone no longer means "not current".
- `PayrollTransaction` snapshots `PositionName`/`DepartmentName`/`EmploymentTypeName`/`BankName`/`BankAccountName`/`BankAccountNo` at calculation time (`snapshotFor`) so closed-cycle payslips/reports stay accurate even if the employee's position/department/bank changes later. Any new closed-cycle report must read these snapshot fields, not `PayrollTransaction.Employees.*` (which reflects the employee's *current* state).
- `Employees.BankName` does not exist — bank name is always resolved live via the `Bank` relation (`Preload("Bank")`), except in the payroll snapshot above.

## Timezone Handling

The business timezone is fixed to **Asia/Bangkok (UTC+7, no DST)** — see `timeutil/timeutil.go`. Never use `time.Now().Location()`, a bare `.UTC()`, or a DB/driver's ambient session timezone to read or write a wall-clock time-of-day value (`AttendanceConfig.WorkStartTime/WorkEndTime`, `TimeLog.CheckIn/CheckOut`, or anything grouped/compared by calendar day). Always go through `timeutil.BangkokLocation` / `timeutil.ToBangkok(t)` / `timeutil.DayStart(t)` / `timeutil.DateKey(t)`. A `*time.Time` read back from Postgres carries whatever timezone the session/driver attached, not what was originally typed — extracting `.Hour()`/`.Year()`/`.Weekday()` etc. directly without normalizing through `timeutil` first is the exact bug class that shifted schedule times and misattributed late-night check-ins to the wrong day. `CreatedAt`/`UpdatedAt` and pure calendar-date fields (`Holiday.Date`, `PayrollCycle.Start/EndDate`, `LeaveRequest.Start/EndDate`, `SalariesStructure.EffectiveDate`) are NOT subject to this — they're either audit instants (must stay real time, compared against real "now") or date-only values with no time-of-day component.
