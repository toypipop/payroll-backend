# Service-layer refactor — working checklist

Goal: every handler goes handler → service → repository, like the `employees` reference. No handler touches `config.DB`.

## Decisions (settled, do not re-litigate)

1. **Per domain, three files:** `repository/<domain>_repository.go` (interface + `gorm*` impl + `New*`), `service/<domain>_service.go` (rules + sentinel errors, no gin, no `config.DB`), `handlers/<pkg>/<domain>_handler.go` (bind → service → map error).
2. **Errors:** domain sentinels are built from `apperr.Validation/NotFound/Forbidden/Conflict/Unauthorized`. The *kind* picks the status. `handlers/httpx.RespondError` is the only status mapper in the codebase — never write a per-domain `switch`. Anything not authored via `apperr` is `KindInternal` → 500 with a generic message, so driver/SQL text cannot leak.
3. **Shared handler plumbing:** `httpx.RequestScope`, `httpx.ParseID`, `httpx.BindJSON`, `httpx.RespondError`. Call them directly; do not re-wrap them per package.
4. **Scope-filter oracle:** `models/module_capability.go` decides which filter is correct. `ScopeLevel == "employee"` ⟺ repository uses `scope.ApplyEmployee`; `"company"` ⟺ `scope.ApplyCompany`. Check this pair for every domain before committing — this is where a silent cross-company data leak would come from. One nuance: `companyLevel` states that `Own`/`Department` are meaningless for the module, **not** that its table must have a `companies_id`. `MasterBankPage` and `DepartmentSettings`' category list are declared company-level over globally shared tables (`bank`, `department_category`) that have no company column, so their repositories correctly apply no filter at all.
5. **Comments:** English, exactly one line per comment, no wrapping. Excess rationale gets cut, not moved. New code follows this already; the sweep over pre-existing comments is round 9.

## Verification per round

```bash
gofmt -l -w <only the dirs you touched>   # NOT the whole repo, see note
go vet ./... && go build ./...
```

To check a package is really off `config.DB`, test for the **import**, not the string:

```bash
for f in handlers/<pkg>/*.go; do grep -q '"payroll-backend/config"' "$f" && echo "$f"; done
```

Grepping for `config.DB` gives false positives — the doc comment on every migrated
service accessor mentions it by name. And `Select-String -SimpleMatch 'config\.DB'`
in PowerShell searches for a literal backslash, so it reports 0 no matter what.

Note: files in this repo are checked out CRLF, so `gofmt -w` rewrites line endings on every file it reads. Git normalizes them away, but scope `gofmt` to the directories you actually edited to keep the working tree quiet.

No test suite exists. `go build` catches signature breakage only — it cannot catch a wrong scope filter or a dropped business rule. Smoke-test the touched endpoints against a running server each round.

## Rounds

- [x] **1 — plumbing + pilot** · `apperr/`, `handlers/httpx/`, `department_categories` (1 file)
- [x] **2 — settings master-data** (11) · `handlers/settings` is now entirely free of `config.DB`
  - [x] 2a · departments, positions, roles
  - [x] 2b · banks, holidays, work_locations, leave_type_settings, day_offs
  - [x] 2c · companies, employee_company_access, permission
- [x] **3 — payrollconfig** (4) · earning, deduction, employment_types, payrollconfig — plus `handlers/payroll/payroll_handler.go`, which held the *routed* copies of the payroll-config endpoints
- [x] **4 — employees remainder** (4) · employee_earnings, employee_deductions, salary, salaries — `handlers/employees` is now free of `config.DB`
- [ ] **5 — attendance** (5) · attendance_config, timelogs, leave, attendance_summary, earning_summary
- [ ] **6 — payroll** (8) · cycles, cycle_earnings, cycle_deductions, calculate, precheck, diff, myslips, snapshot_service — `payroll_handler.go` was already done in round 3
- [ ] **7 — reports** (1) · reports
- [ ] **8 — auth** (1) · auth — repository + service for lookup/claims; JWT signing and cookie writing stay in the handler because those are HTTP
- [ ] **9 — comment sweep** · both repos, rule 5 above

## Check the route table before refactoring a handler

`GetPayrollConfigs` and `UpdatePayrollConfig` existed in **both**
`handlers/payroll` and `handlers/payrollconfig`, with different behaviour, and only
the `handlers/payroll` pair was routed (`/global`). The unrouted copy used
archive-then-insert versioning that would have collided with the plain unique
index on `payroll_config.companies_id` — a soft-deleted row keeps occupying that
value — which is presumably why it was abandoned. Refactoring it and wiring it up
would have broken a working endpoint. Grep `routes/routes.go` for the handler name
before assuming a function is live, and note the import aliases: `payhandler` is
`handlers/payroll`, `cfghandler` is `handlers/payrollconfig`.

## Preserved deliberately in round 5b — questionable, but not this refactor's call

These were carried over unchanged. Each is a behaviour change if "fixed", so they
need their own decision rather than being folded into a refactor.

- `BulkCreate` writes logs one at a time and returns on the first failure, leaving
  everything created so far in place. No transaction, so a mid-run error yields a
  partial range with a 500.
- The "this date already has a log" check builds its window from the parsed
  calendar date, which is a UTC midnight, not a business-timezone day. The window
  therefore covers 07:00 to 07:00 Bangkok. With the shifts actually in use (09:00,
  16:00) it lands correctly, but a check-in before 07:00 Bangkok would be counted
  against the previous date.
- `DeriveStatus` ignores errors from its holiday and day-off counts and from the
  attendance-config lookup, treating a failed query as "no holiday, no day off,
  default 09:00 start". A database hiccup silently downgrades a status rather than
  failing the request.

## Found while migrating — decide separately, do not fold into a refactor commit

- ~~`applyCompanyFilter` returns the query unfiltered on an empty authorized company list~~ — **resolved**, now denies. Investigated first: this was never reachable. `AuthMiddleware` 403s when `AccessibleCompanyIDs` is empty, `CheckPermission` 403s unless it resolved at least one company, and `fk_employees_companies` makes `companies_id = 0` unstorable (verified by attempting the update: FK violation). Changed anyway so the branch fails closed if any of those guards ever moves. No user's access changed — confirmed by re-measuring all four seeded logins.
- `GetRoles` had no scoping whatsoever; fixed in round 2a because RolesPage is declared company-level and the doc comment already claimed the scoping existed.

## Database reconciliation (checked 2026-08-11)

An existing database does not need wiping to match a fresh seed. The boot chain
`RemoveDashboardModule` → `SyncModuleCapabilities` → `PruneStaleRoleScopes`
already reconciles the module and capability projection on every start. Measured
on the local database after fixing the sync: no seeded module missing, no stale
module row, no capability column disagreeing with the registry, no company-level
module holding a non-Companies scope.

Reconciliation belongs in `migrations/` (runs every boot), never in seed — seed
only creates and updates, so it can never remove a row the code stopped
declaring. That asymmetry is why `RemoveDashboardModule` had to exist.

## Endgame

Once every domain is migrated, the `gin.Context`-taking scope helpers (`repository.ApplyCompanyScopes`, `repository.ApplyEmployeeScopes`) have no callers left and get deleted. `AccessScope` becomes the only way to scope a query.
