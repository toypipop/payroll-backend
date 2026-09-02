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
- `repository/pg_error.go` — Postgres SQLSTATE translation (`23505` → `apperr.Conflict`), registered on the connection in `wire.go`.
- `migrations/*` — raw/schema repairs not safely covered by AutoMigrate.
- `seed/main.go` — local/test data, modules, permissions.
- `seed/reset.go` — `--reset`: `TRUNCATE ... RESTART IDENTITY CASCADE` over every table in the public schema, refused when `APP_ENV`/`GO_ENV`/`GIN_MODE` marks a production environment. It truncates rather than drops, so the schema survives for `AutoMigrateSchema`, and id sequences restart at 1 so a reseeded database is reproducible. `schema_migration` is truncated with everything else, so the `RunOnce` repair migrations re-run against the empty tables.

## Commands

```bash
go run .                            # run server
go run ./migrate                    # run migrations
go run ./seed                       # reseed data (merges onto existing rows)
go run ./seed --reset               # wipe every table first, then reseed from empty
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

### Salary audit trail

- `salary_audit` is append-only: nothing updates or deletes a row, and there is no soft-delete column. Every write path in `SalariesStructureService` records exactly one entry — `created`, `amount_changed`, `moved`, `switched_off`, `deleted` — **inside the same transaction as the change**, so a change that cannot be recorded is not made.
- A reason is required on all of them, the delete included; `cleanReason` trims it and refuses whitespace.
- `ActorName` is stored beside `ActorID` rather than joined. An entry is a statement about what happened, and joining would let it change when the employee row does. `FindScopeUser` selects the name alongside the scoping columns so this costs no extra query — keep that `Select` explicit, or the password hash starts riding along.
- A delete leaves `SalariesStructureID` null: the row it pointed at is gone, which is why the trail is a separate table rather than a column on the timeline.
- The seed writes the trail the demo salaries would have had — first record on a timeline reads as a hire, later ones as raises — so the change log is not empty on every employee until someone edits their pay.

### Module capabilities (what the permission UI is allowed to offer)

- `models/module_capability.go` holds `moduleCapabilities`, the **single source of truth** for each module's `ScopeLevel` (`company`/`employee`), `AllowedActions`, and `ActionsAtomic`. `AllowedScopes` is derived from `ScopeLevel`, never declared separately.
- `ScopeLevel` is a fact about the handler, not a policy: it must match which `Apply*Scopes` the module's queries call. `AllowedActions` must match the action strings its routes pass to `CheckPermission` — several delete routes are gated by `"EDIT"`, so those modules do not list `DELETE`.
- A missing action in `AllowedActions` fails **silently and permanently**: `service.PermissionService.Replace` writes `CanEdit && capability.AllowsAction(EDIT)`, and the matrix never renders a checkbox to put it back. `SalariesStructurePage` shipped without `ActionEdit` while `PATCH /salaries/:id` gated on `"EDIT"`, so the first successful matrix save would have killed the Edit button for good. When a route gates on an action, the registry entry lists it — check both sides together.
- `migrations.SyncModuleCapabilities` projects the registry onto `app_module` columns on **every boot** (and at the end of seed), so the DB can never drift from the code and a deploy never depends on reseeding. An active, non-category module missing from the registry is a `log.Fatal`.
- `migrations.PruneStaleRoleScopes` rewrites saved permission rows whose `DataScope` a company-level module cannot honour. It only makes stored values agree with behaviour already in effect; no role gains or loses access.
- `service.PermissionService.Replace` coerces disallowed actions to `false` and disallowed scopes to `Companies` before writing, so the database never stores a claim the permission check ignores. It validates the whole payload *before* opening the transaction, so a rejected payload cannot clear a role's existing rows even transiently.
- **A category module is a heading, not a page.** The matrix posts the whole tree back, `ModuleEmployees`/`ModuleAttendance`/... included; those rows carry no capability and no permission row. `repository.CapabilitiesFor` returns them in `ModuleCapabilities.Categories` and `Replace` skips them — counting them as unknown ids is what made **every** matrix save fail with "Permission payload references an unknown module". `ErrUnknownModule` still fires for an id that resolves to nothing at all.
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
- **`middleware.Guard`** (`NewGuard(accessRepo, deleteGuardRepo)`) owns `Auth`, `CheckPermission` and `PreventDeleteIfReferenced`. `Guard.Auth` stores `scope_user` (id, dept_id, companies_id) on the request so `httpx.RequestScope` builds an `AccessScope` without a query of its own; `CheckPermission(moduleKey, action)` gates one module key per route. Stateless middleware (`RequireXMLHttpRequest`, CSRF) stays package-level.

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
- **Seeded clock times are built in `timeutil.BangkokLocation`, never `time.UTC`.** A wall clock only means something in a timezone: `time.Date(..., 9, 0, ..., time.UTC)` stores 09:00Z, which every correct reader renders as 16:00 Bangkok — that is how the seeded 09:00-17:00 shift showed up in the UI as 4:00 PM to 12:00 AM, and why seeded logs looked like overnight shifts. Same for payroll cycle `StartDate`/`EndDate`: a UTC midnight is 07:00 Bangkok, and a `23:59:59` UTC end lands on the next Bangkok day. `service/time_log_service.go` already did this correctly, so only the seed disagreed.
- Seeded sign-ins: **every account's password is its own email**, set in one pass at the end of `seed/main.go`. One account per role for company A — `alice@techa.com` ADMIN, `hr@techa.com` HR, `finance@techa.com` FINANCE, `bob@techa.com` MANAGER, `carol@techa.com` STAFF — and `david`/`eve`/`frank@bizgroup.com` mirroring ADMIN/MANAGER/STAFF for company B. Note the `devUsers` block near the end **overwrites `roles_id`**, so a role set at employee creation that disagrees with it is silently discarded: that is how MANAGER ended up with no holders at all and `Department` scope went untested. A person's department (`dept_id`) is not their role — Carol works in the HR department but holds the STAFF role.
- All five report modules (`PayslipsReportPage`, `BankTransferReportPage`, `TaxFormsReportPage`, `SSOReportPage`, `PayrollAnalysisPage`) live under the `Reports` category and are GET-only → declared `companyLevel(ActionView)`, so the matrix shows a View toggle and a fixed `Companies` scope, with C/E/D hidden.
- `CompaniesPage`, `MasterBankPage` and `HelpPage` are declared `atomic(...)`: one on/off toggle driving every action, no scope selector. Seeded full CRUD=true so the toggle reads "on".
- Reports cannot be `Own`-scoped — they are company-level, so the selector is not offered at all. STAFF gets no reports; staff see own pay via Own-scoped Salaries / Employee Earnings / Employee Deductions.
- Sanity check that the registry is right: every module STAFF holds at `Own` scope, and every module MANAGER holds at `Department`, must be declared `employeeLevel`. If one is `companyLevel`, either the grant or the declaration is wrong.
- **Dashboard and My Payslips are not modules and have no permission rows.** Both are universal pages whose routes are hardcoded in `App.tsx` (and `"/"` redirects to `/dashboard`), with hardcoded sidebar links. A module row for Dashboard only ever hid the sidebar link while the page stayed reachable, so it was removed — `migrations.RemoveDashboardModule` clears it from existing databases and must run before `SyncModuleCapabilities`. What a user sees on the Dashboard is governed by the modules it reads from (`EmployeesListPage`, `PayrollCyclesPage`), each enforcing its own scope.

### Master-data `/options` convention

Any master-data type consumed by a page other than its own "owning" settings page (a cross-page dropdown) needs a dedicated, **ungated** `GET .../options` sibling endpoint — do not loosen `CheckPermission` on the main CRUD endpoint instead. Pattern (mirrors companies, where the ungated `GET /companies/options` is served by `Settings.GetCompanies` and the gated full list `GET /companies/all` by `Settings.GetAllCompanies`):
- No `middleware.CheckPermission(...)` on the route.
- Scoped only by company data-access (`scope.ApplyCompany(...)`, or the legacy `repository.ApplyCompanyScopes(...)` in a not-yet-migrated handler), never by page permission.
- Returns minimal fields only (`id`, `name`, `companies_id` — never sensitive/rate fields).
- Existing examples: `/departments/options`, `/positions/options`, `/roles/options`, `/master/employment-types/options`, `/earning-types/options`, `/deduction-types/options`, `/master/leave-types/options`, `/master/banks/options`, `/master/work-locations/options`, `/master/day-offs/options`, `/payroll/cycles/options`.
- `/payroll/cycles/options` is the convention applied to something that is not master data: the salary forms warn when an effective date lands in a closed period, and `GET /payroll/cycles` is gated on `PayrollCyclesPage`. A user may adjust a salary without being allowed to open that page — `staff@admin.com` is exactly that — and they are the ones the warning is for.
- Exception: if the underlying model has genuinely sensitive fields (e.g. `AttendanceConfig` payroll rates), build a separate purpose-built endpoint instead of a generic `/options` list (see `GetTimelogScheduleConfig`).

## Business Rules

- `employees.companies_id` = home company for payroll, attendance, salary, department, position, employment type.
- `employee_company_access.company_id` = app access only, not payroll ownership.
- **Deleting an employee retires their salary records with them**, in the same transaction (`gormEmployeesRepository.Delete`), clearing `is_active` as it sets `deleted_at`. A salary record only means anything as "what this person was paid"; left behind it is a row no page reaches and no rule maintains.
- **Creating an employee does NOT need an `EmployeeCompanyAccess` row, and does not write one.** `AccessibleCompanyIDs` / `AccessibleRoleIDs` / `AccessibleCompanyRoles` (`repository/access_repository.go`) seed themselves from the employee's own `companies_id` and `roles_id` first, then add the access rows on top. A new employee therefore authenticates and holds their role from the moment the row exists; `employee_company_access` grants **extra** companies only and is managed on its own settings page.
- Employee-linked APIs must validate requester access and reject employee/company mismatches.
- Time-log create/edit must derive or validate company from the selected employee.
- Attendance config: end <= start means overnight, e.g. `16:00` → `00:00` checks out next day.
- Leave request dates are date-only, normalized to midnight UTC.
- Leave Requests: only `Pending` can edit; any accessible status can be soft-deleted with delete permission.
- **`companies_id` is immutable after creation** on every model (Employees, Departments, Positions, Roles, EarningType, DeductionType, DayOff, ...). Update handlers must never write `companies_id` from request input — re-derive/ignore it. Create handlers are the only place it's settable.
- `SalariesStructure` edits are blocked if `effective_date` falls inside an already-closed `PayrollCycle` for that employee's company (see `UpdateSalariesStructure`).
- **`is_active` says a record is switched on, not that it is the one in force.** Which record is in force is the latest effective date among the switched-on records, derived on every read (`currentSalaryCondition` in the repository, `service.currentRecord` above it). A flag that meant "current" had to be recalculated on every add, correction and delete, and the moment it was not, the page and the payroll run disagreed about what someone earns. Several records are switched on at once, and that is normal: each one is the rate its own period is paid at. Disabling a salary switches **all** of the employee's records off (`SwitchOffAll`), which is what makes a run find nothing to pay them from; saving on a record's date switches it back on. `migrations.NormalizeSalaryActiveFlags` rewrote the old meaning into this one.
- **`SalariesStructure.Version` is an optimistic lock, and it belongs to whichever record is current.** The page is seeded from the current record and sends its `version` back on save; `Save` refuses the write when the current record no longer carries that number, which reaches the client as **412 Precondition Failed** (`apperr.Precondition`) — not 409, so the form can tell "your screen is stale, reload" apart from "the date you picked is not allowed". A client that sends no version (`0`) is treated as a create.
  - `service.currentVersion` reads the version off the record in force, found by date. It deliberately does not take the highest version in the history: a delete leaves an older record in force, and comparing against the maximum would then refuse every later save.
  - A correction bumps the record it corrects; a new record takes `nextVersion` — one past the highest number this employee has ever used, deleted records included. Counting from the record in force instead would hand the same number out twice, because adding a record to the past leaves the record in force untouched. Nothing else is ever rewritten by a save, so no other record's version moves.
  - Correcting a record switches it back on **only when it holds the latest effective date** (`isLatestOn`). Switching on an older one while a later record sits switched off would quietly make the older figure the salary in force — which is not what correcting a past period asks for.
  - The writes are guarded by `WHERE id = ? AND version = ?`, so a second writer that read the same record before this one committed gets a 412 instead of overwriting the correction.
  - `AutoMigrateSchema` backfills `version = 1` for rows written before the column existed, and clears `is_active` on rows that an old erase left active behind a set `deleted_at`.
- **`PayrollTransaction` deliberately has no version column.** Every write to it happens inside a payroll run, and the run already holds `SELECT ... FOR UPDATE` on the cycle, so two writers cannot interleave in the first place — an optimistic lock there would be a second mechanism guarding a door that is already bolted. If a transaction ever becomes editable outside a run, it needs one.
- **All three salary write paths are one transaction that starts with `LockCompanyCycles`.** `Save`, `UpdateRecord` and `DeleteRecord` open `repo.WithinTx(...)` and take a `FOR SHARE` lock on the company's payroll cycles before reading anything, because every rule they enforce is a check the write depends on: "no closed cycle covers this date" and "no closed cycle ever used this record" are both meaningless if a `Close` (which takes `FOR UPDATE` on the cycle) can commit between the check and the write. The repository methods below them never open transactions of their own; they run on the handle the caller gives them.
- **A record may be added anywhere on the timeline.** There is no "must be later than the last one" rule: a run prices each cycle from the record in force on that cycle's dates, so a record slotted into the past is the rate that period is meant to be paid at, not a stray. The periods already paid for are protected by the closed-cycle check, and that check stands on its own. What the timeline decides instead is which record is **in force now** — the latest effective date among the switched-on records — so a record added behind it is switched on like any other and today's salary is left alone by arithmetic, not by a flag. A correction behaves the same way: fixing what an earlier period pays does not put that record back in force.
- **`PATCH /salaries/:id` is the only path that changes an effective date.** A save is addressed *by* date, so it can only ever reach the record already sitting on the date typed into it; moving a record is `SalariesStructureService.UpdateRecord`, behind the history page's Edit button. A move re-prices two periods at once — the one the record leaves and the one it lands in — so it is refused when the record was already calculated into a closed cycle (`ErrSalaryRecordPaid`), when the new date falls inside a closed cycle (`ErrSalaryDateInClosed`), when another live record already holds that date (`ErrSalaryDateTaken`), or when the version sent no longer matches the row (412). It never touches `is_active`: where a record applies is not the same question as whether it is in use.
- Lateness is deducted **per day, capped per day** at `AbsentRatePerDay` — a day of lateness must never cost more than a day of absence, but a cycle of daily lateness must still cost more than one late day. `runAbsence.lateMinutesPerDay` keeps one entry per late day for exactly this reason; never collapse it into a single total and cap that. `seed/main.go` mirrors the same rule in `seedLateDeduction` — it prices demo transactions itself instead of calling the engine, so a change to either late/absent/overtime pricing has to land in both.
- Seeded payroll figures must be priced from the `seed*RatePerMinute` / `seed*RatePerDay` constants at the top of `seed/main.go`, which are also what the seeded `AttendanceConfig` rows carry. Hardcoding a rate in the transaction loop is how the demo data drifted out of agreement with its own config (absent was billed at 1100 against a configured 300).
- **A payroll run is one transaction.** `PayrollRunService.Snapshot` / `Recalculate` / `Close` open a single `repo.WithinTx(...)`, take `LockCycleForUpdate` (a `SELECT ... FOR UPDATE` on the cycle row) as the first statement inside it, and only then run the per-employee loop in the unexported `snapshot`. The loop **returns on the first error** — the old code collected a `lastErr`, kept going, and answered 500 after committing half the employees. `Close` marks the cycle closed inside that same transaction, so figures can never be rewritten onto a cycle that stayed open. Anything calling `SaveSnapshot` outside a transaction is a bug; the repository method deliberately no longer opens one of its own.
- **`PayrollTransaction` is unique on `(cycle_id, employees_id, deleted_at)`** (`idx_txn_cycle_employee`) and `SaveSnapshot` upserts on exactly those columns. Find-then-create let two concurrent Calculate requests both miss the row and both insert — a duplicate payout. `migrations.DedupePayrollTransactions` runs at the top of `AutoMigrateSchema` (it must, or the index cannot be created on a database that already has duplicates) and keeps the most recently updated live row per pair.
- **A unique-constraint violation is a 409, not a 500.** `repository.RegisterErrorTranslation(db)` is registered once in `buildHandlers` and rewrites Postgres SQLSTATE `23505` into `apperr.Conflict` on every create and update. It is a connection-level callback rather than a per-repository call because the mapping is a property of Postgres, and a repository that forgets it is a silent 500.
- `EmployeesService.Update` filters its patch through the `employeeUpdatableFields` **allowlist**, not a list of forbidden keys. The map goes straight into `Updates(...)`, so an unlisted column — `companies_id`, `id`, or one added later — must be dropped rather than trusted.
- A salary lookup that fails is not an employee to skip. `salaryForCycle` treats only `repository.ErrPayrollSalaryMissing` as "no salary"; any other error aborts that employee and surfaces from the run. Skipping on a database error would drop the employee from the cycle *and* delete the transaction an earlier run wrote.
- **A run prices a cycle from the salary that was in force then, not the one in force today.** `FindSalaryOnOrBefore` picks the latest record whose effective date has already arrived by the cycle's start. Superseded records stay in the table precisely so this lookup can reach them. It does filter `is_active`, but for the flag's own meaning — a record that is switched off does not count, and disabling a salary switches every one of the employee's records off, which is what makes a run find nothing to pay them from. It is not a filter for "the current record": that would price every recalculated cycle at today's figure, or find nothing at all when today's record is dated after the cycle, and drop the employee from the run.
- `PayrollRunService.Snapshot` only applies a salary record to a cycle if `effective_date <= cycle.StartDate` — a raise effective mid-cycle takes effect starting the *next* cycle, never retroactively within the current one. If no record covers the cycle's start at all (a brand-new hire whose first record lands mid-cycle), it falls back to the first record with `effective_date <= cycle.EndDate` and prorates `BaseSalaries` by calendar days worked ÷ total days in the cycle.
- **Saving a salary never erases, or even touches, the record before it.** A raise adds a record; the one before it keeps its figure, keeps `is_active`, and stays in the history, because the salary in force until then is a fact about that period. It stops being the current salary by arithmetic alone — a later date now exists. `DeletedAt != 0` is set by exactly one thing, an explicit delete from the history page, and that delete is refused for a record a closed cycle already paid against. Any query that needs "the current salary" uses `currentSalaryCondition`, never `is_active = true` on its own.
- **One effective date is one record.** Saving on a date the history already holds corrects that record — `Save` compares `timeutil.DateKey`, calls `UpdateAmount`, and the record comes back into force, which is also how a switched-off salary is turned back on without inventing a date. The same date cannot mean two different figures; that is what `idx_emp_sal_eff_deleted` enforces, and correcting in place is what keeps a save from ever colliding with it. A *later* date is a new step and adds a record; an *earlier* one is refused. A record a closed cycle was already calculated from cannot be corrected at all (`ErrSalaryRecordPaid`) — the new figure belongs on a later date, because that record is the basis of a payslip already paid.
- `PayrollTransaction` snapshots `PositionName`/`DepartmentName`/`EmploymentTypeName`/`BankName`/`BankAccountName`/`BankAccountNo` at calculation time (`snapshotFor`) so closed-cycle payslips/reports stay accurate even if the employee's position/department/bank changes later. Any new closed-cycle report must read these snapshot fields, not `PayrollTransaction.Employees.*` (which reflects the employee's *current* state).
- `Employees.BankName` does not exist — bank name is always resolved live via the `Bank` relation (`Preload("Bank")`), except in the payroll snapshot above.

## Timezone Handling

The business timezone is fixed to **Asia/Bangkok (UTC+7, no DST)** — see `timeutil/timeutil.go`. Never use `time.Now().Location()`, a bare `.UTC()`, or a DB/driver's ambient session timezone to read or write a wall-clock time-of-day value (`AttendanceConfig.WorkStartTime/WorkEndTime`, `TimeLog.CheckIn/CheckOut`, or anything grouped/compared by calendar day). Always go through `timeutil.BangkokLocation` / `timeutil.ToBangkok(t)` / `timeutil.DayStart(t)` / `timeutil.DateKey(t)`. A `*time.Time` read back from Postgres carries whatever timezone the session/driver attached, not what was originally typed — extracting `.Hour()`/`.Year()`/`.Weekday()` etc. directly without normalizing through `timeutil` first is the exact bug class that shifted schedule times and misattributed late-night check-ins to the wrong day. The one thing a calendar-date field still needs is a *day boundary* in the business timezone: `service.dayOf` is `timeutil.DayStart`, never `Truncate(24 * time.Hour)`, which rounds to a UTC-epoch boundary (07:00 Bangkok) and shifts the day key for anything stored in the evening.

**One convention, everywhere.** A calendar-date column stores midnight of that day in the business
timezone; a period's closing date stores 23:59:59 of its last day, so `stored_date <= end_date` is true for
anything on the closing day without the caller normalising first. Read a `"2006-01-02"` string with
`timeutil.ParseDayStart`, never `time.Parse` — the latter silently means UTC, which is 07:00 Bangkok.
`timeutil.DayEnd` builds the closing instant. The columns this governs: `salaries_structure.effective_date`,
`holiday.date`, `leave_request.start_date/end_date`, `payroll_cycle.start_date/end_date`.

This was not always true. Services parsed with `time.Parse` while the seed used the business timezone, so
one table held both; `migrations.NormalizeBusinessDatesToBangkok` moved the stragglers, selecting on the
Bangkok time-of-day (`07:00:00`, or `06:59:59` for a period end) so a second run matches nothing. The
symptom that made it worth fixing: `payroll_run_service.go` computes `dayOf(cycle.EndDate).AddDate(0,0,1)`
through a Bangkok lens, so every cycle stored the other way reached a full extra day into the next month —
18 of 30 cycles, with real time logs inside the overlap.

A comparison between two calendar dates belongs on the day, not the instant: see `isLatestOn`, which
compares `timeutil.DayStart` values so a row a few hours off cannot outrank one on the same day.


`CreatedAt`/`UpdatedAt` and pure calendar-date fields (`Holiday.Date`, `PayrollCycle.Start/EndDate`, `LeaveRequest.Start/EndDate`, `SalariesStructure.EffectiveDate`) are NOT subject to this — they're either audit instants (must stay real time, compared against real "now") or date-only values with no time-of-day component.
