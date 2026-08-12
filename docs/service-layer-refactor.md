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
- [x] **5 — attendance** (5) · attendance_config, timelogs, leave, attendance_summary, earning_summary — `handlers/attendance` is now free of `config.DB`

### Verifying a calculation refactor

For the two summary endpoints, correctness was established by A/B comparison rather
than by reasoning: capture the JSON from the new code, `git stash push -u` the new
files, rebuild and capture the same requests from the old code, then diff. 24
responses (3 logins x 4 date windows x 2 endpoints) came back byte-for-byte
identical. Use this for anything that computes money — it catches an arithmetic or
ordering slip that no amount of reading will.
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

## Fixed in round 5c — the approver was taken from the request body

`ApproveLeaveRequest` wrote `approved_by` from the JSON body, and
`LeaveApprovedPage.tsx` filled it from `localStorage`. Anyone able to approve could
therefore record the approval under a different employee's name by editing the
request. The approver is now `scope.User.ID`. The field is still accepted in the
body and ignored, so no frontend change is required, though the frontend could stop
sending it.

## Fixed in round 5b — three real bugs found in the time-log path

- **Bulk generation was not atomic.** Logs were written one at a time and the first
  failure returned immediately, leaving a partial date range behind. It is now one
  batched insert inside a transaction.
- **The duplicate-date check used a UTC day window.** Dates were parsed with
  `time.Parse`, giving UTC midnight, so the window ran 07:00 to 07:00 Bangkok while
  the logs it created were anchored to Bangkok midnight. A check-in before 07:00
  Bangkok was counted against the previous date, so re-running a range would
  duplicate it. Dates are now parsed with `timeutil.ParseDayStart`.
- **`DeriveStatus` swallowed query errors**, treating a failed holiday or day-off
  count as "not a holiday" and a failed config lookup as a 09:00 start. That label
  feeds payroll deductions, so a database hiccup could silently change pay. Errors
  now propagate; "this company has no configuration yet" is still a normal
  fallback, distinguished by `repository.ErrAttendanceConfigNotFound`.

`timeutil.ParseDayStart` exists for the first of these. Its doc comment names the
date-only columns it must **not** be used on (`Holiday.Date`,
`PayrollCycle.Start/EndDate`, `LeaveRequest.Start/EndDate`,
`SalariesStructure.EffectiveDate`) — those are compared against other date-only
values and anchoring them to +07 would shift them.

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
