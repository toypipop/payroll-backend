# Service-layer refactor — working checklist

Goal: every handler goes handler → service → repository, like the `employees` reference. No handler touches `config.DB`.

## Decisions (settled, do not re-litigate)

1. **Per domain, three files:** `repository/<domain>_repository.go` (interface + `gorm*` impl + `New*`), `service/<domain>_service.go` (rules + sentinel errors, no gin, no `config.DB`), `handlers/<pkg>/<domain>_handler.go` (bind → service → map error).
2. **Errors:** domain sentinels are built from `apperr.Validation/NotFound/Forbidden/Conflict/Unauthorized`. The *kind* picks the status. `handlers/httpx.RespondError` is the only status mapper in the codebase — never write a per-domain `switch`. Anything not authored via `apperr` is `KindInternal` → 500 with a generic message, so driver/SQL text cannot leak.
3. **Shared handler plumbing:** `httpx.RequestScope`, `httpx.ParseID`, `httpx.BindJSON`, `httpx.RespondError`. Call them directly; do not re-wrap them per package.
4. **Scope-filter oracle:** `models/module_capability.go` decides which filter is correct. `ScopeLevel == "employee"` ⟺ repository uses `scope.ApplyEmployee`; `"company"` ⟺ `scope.ApplyCompany`. Check this pair for every domain before committing — this is where a silent cross-company data leak would come from.
5. **Comments:** English, exactly one line per comment, no wrapping. Excess rationale gets cut, not moved. New code follows this already; the sweep over pre-existing comments is round 9.

## Verification per round

```bash
gofmt -l -w <only the dirs you touched>   # NOT the whole repo, see note
go vet ./... && go build ./...
```

Note: files in this repo are checked out CRLF, so `gofmt -w` rewrites line endings on every file it reads. Git normalizes them away, but scope `gofmt` to the directories you actually edited to keep the working tree quiet.

No test suite exists. `go build` catches signature breakage only — it cannot catch a wrong scope filter or a dropped business rule. Smoke-test the touched endpoints against a running server each round.

## Rounds

- [x] **1 — plumbing + pilot** · `apperr/`, `handlers/httpx/`, `department_categories` (1 file)
- [ ] **2 — settings master-data** (11) · banks, holidays, work_locations, leave_type_settings, day_offs, departments, positions, roles, companies, employee_company_access, permission
- [ ] **3 — payrollconfig** (4) · earning, deduction, employment_types, payrollconfig
- [ ] **4 — employees remainder** (4) · employee_earnings, employee_deductions, salary, salaries
- [ ] **5 — attendance** (5) · attendance_config, timelogs, leave, attendance_summary, earning_summary
- [ ] **6 — payroll** (9) · cycles, cycle_earnings, cycle_deductions, calculate, precheck, diff, myslips, payroll, snapshot_service
- [ ] **7 — reports** (1) · reports
- [ ] **8 — auth** (1) · auth — repository + service for lookup/claims; JWT signing and cookie writing stay in the handler because those are HTTP
- [ ] **9 — comment sweep** · both repos, rule 5 above

## Endgame

Once every domain is migrated, the `gin.Context`-taking scope helpers (`repository.ApplyCompanyScopes`, `repository.ApplyEmployeeScopes`) have no callers left and get deleted. `AccessScope` becomes the only way to scope a query.
