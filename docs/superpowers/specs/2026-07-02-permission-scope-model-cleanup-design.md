# Permission Scope Model Cleanup — Design

Date: 2026-07-02
Repos: `payroll-backend`, `payroll-frontend`

## Background

While debugging why `Employee Company Access` couldn't grant a user access to a
company they weren't already assigned to, three related gaps in the
role/data-scope model surfaced:

1. `Data Scope = Full Organization` (`ScopeAll`) never actually bypasses the
   company boundary anywhere except the two `/companies` list endpoints
   (patched earlier in this session). Everywhere else in the app it behaves
   identically to `Data Scope = Full Companies` (`ScopeCompanies`), because
   `ApplyContextScopes` applies the `companyIDs` filter whenever
   `len(companyIDs) > 0`, regardless of scope value. Since reaching "see
   everything" still requires adding `Employee Company Access` rows per
   company either way, the `All` tier adds a confusing option with no real
   benefit and is being removed.
2. The `CompaniesPage` and `MasterBankPage` modules represent data that has no
   owning company (companies themselves, and the master bank list). Making
   admins pick a company + data-scope for these two modules in
   `PermissionSettings.tsx` is meaningless — `MasterBankPage`'s backend
   handler already ignores scope entirely; `CompaniesPage`'s handlers should
   match.
3. `Data Scope = Department` compares `employees.dept_id` by exact row ID
   (`repository/scope.go:297-305`). Since `Department` rows are unique per
   company (`models/settings.go:16-17`), this can never match "HR" in company
   A against "HR" in company B — cross-company department-scoped roles (e.g.
   "see HR across every company") are structurally impossible today.

## Goals

- Remove the unused/confusing `ScopeAll` tier.
- Make `CompaniesPage` / `MasterBankPage` permission rows scope-free.
- Let a role's `Department` scope match departments by a shared category
  (e.g. "HR") across every company the role's user has access to, instead of
  by literal department ID.

## Non-goals

- No change to `Employee Company Access` as the source of truth for *which*
  companies a user can reach — this spec only touches *how much* they see
  once reachable.
- No full CRUD settings page for `department_categories` — creation happens
  inline from the Department form (YAGNI; a dedicated page can follow if
  actually needed later).
- No change to `RolesModuleAccess.CompaniesID` (the company a *role
  definition* belongs to) — unrelated axis, out of scope.

## Design

### 1. Remove `ScopeAll`

- `models/auth.go:86`: binding tag `oneof=Own Department Companies All` →
  `oneof=Own Department Companies`. Keep the `ScopeAll` Go constant removed
  from the allowed set but the DB column stays a plain string, so no schema
  migration is required — just stop writing/accepting `"All"`.
- `seed/main.go:437`: SYSTEM_ADMIN seed changes from `models.ScopeAll` to
  `models.ScopeCompanies` (matching the pattern already used one line below
  for `sysStaffRole`). Whoever needs SYSTEM_ADMIN to see every company adds
  `Employee Company Access` rows for it, same as any other cross-company
  role — no special case.
- `payroll-frontend/src/pages/settings/PermissionSettings.tsx:24`: remove the
  `{ id: "All", name: "Full Organization" }` entry from `dataScopeOptions`.
- The `ScopeAll` special-case branch added earlier today in
  `companies_handler.go` becomes redundant once change #2 below ships (no
  scope filtering at all on those handlers) — it is deleted, not kept as dead
  code.
- Any existing rows with `data_scope = 'All'` in a running DB are the user's
  responsibility to migrate (confirmed out of scope for this change — no
  production data touched here).

### 2. `CompaniesPage` / `MasterBankPage` become scope-free

- `handlers/settings/companies_handler.go`: `GetCompanies`, `GetAllCompanies`,
  `UpdateCompanies`, `DeleteCompanies` drop `repository.ApplyContextScopes`
  entirely — they become straight `config.DB.Find(...)` /
  `config.DB.Delete(...)` calls, gated only by the existing
  `CheckPermission("CompaniesPage", <action>)` middleware. This matches
  `CreateCompanies` (already scope-free) and `banks_handler.go` (already
  scope-free for all four handlers).
- `payroll-frontend/src/pages/settings/PermissionSettings.tsx`: the "Data
  Scope" column (`PermissionMatrix`, around line 308-319) returns `null` for
  modules in `ALL_OR_NOTHING_MODULES`, same pattern already used for the
  Create/Edit/Delete columns. When persisting, rows for these two modules
  send a fixed `data_scope: "Companies"` (any valid enum value works — it is
  never read by the handlers after this change).

### 3. `department_categories`

**New table** `department_categories`:
- `id` (PK)
- `name` (string, unique, not null)
- timestamps + soft delete (matches existing settings tables)

**`Department` model** (`models/settings.go`): add `CategoryID uint` FK
(not null) referencing `department_categories.id`.

**Seed migration** (`seed/main.go`): for each seeded department, look up or
create a `department_categories` row by matching the existing `DeptName`
(`FirstOrCreate` by name), and set `CategoryID`. With current seed data this
produces 3 categories: "System IT", "Engineering" (shared by compA and
compB), "HR". No separate backfill script — seed is authoritative for
local/dev; UAT/prod data migration is the user's own responsibility (already
confirmed).

**Scope filter rewrite** (`repository/scope.go`, `case models.ScopeDepartment`
inside `scopeDataAccessWithCompanyIDs`, currently lines 297-305): replace the
literal `dept_id = user.DeptID` comparison with a category match:

```go
case models.ScopeDepartment:
    if len(companyIDs) == 0 {
        return db.Where("1 = 0")
    }
    var userCategoryID uint
    db.Session(&gorm.Session{NewDB: true}).
        Table("department").
        Select("category_id").
        Where("id = ?", user.DeptID).
        Scan(&userCategoryID)
    if userCategoryID == 0 {
        return db.Where("1 = 0")
    }
    subQuery := db.Session(&gorm.Session{NewDB: true}).
        Table("employees").
        Select("employees.id").
        Joins("JOIN department ON department.id = employees.dept_id AND department.deleted_at = 0").
        Where("department.category_id = ? AND employees.companies_id IN ?", userCategoryID, companyIDs)
    return db.Where(col+" IN (?)", subQuery)
```

This is a full replacement, not an additional scope tier (confirmed) — every
existing `Department`-scoped role automatically starts matching by category
once departments are migrated. Modules that call `ApplyContextScopes` with
`accessCol = "unused"` (leave type settings, holidays, work locations,
attendance config, day offs) are unaffected — they already skip this branch
entirely (`if col == "unused" { return db }`, line 290-292) and remain scoped
to company only, same as before.

**Backend endpoints** (new, minimal):
- `GET /department-categories` — list all (for the dropdown), gated by the
  existing `DepartmentSettings` module's `VIEW` permission (no new module
  needed).
- `POST /department-categories` — create one `{ name }`, gated by
  `DepartmentSettings` module's `CREATE` permission. Used for the inline
  "add new category" action from the Department form.
- No `PUT`/`DELETE` — not needed for this iteration (YAGNI, confirmed).

**Frontend** (`DepartmentSettings.tsx`): add a "Category" `SelectField`
populated from `GET /department-categories`, plus a lightweight inline
"+ Add new category" affordance (small text input revealed next to the
select) that calls `POST /department-categories` and selects the newly
created category — no navigation away from the Department page.

## Testing

- Backend: `go vet ./...`, `gofmt -w` on touched files, `go test ./...`.
- Manual smoke via `run-payroll-backend` skill: seed, verify
  `department_categories` populated correctly (3 rows, Engineering shared),
  verify a `Department`-scoped staff role now sees employees in the matching
  category across companies it's assigned to (not just its home company).
- Frontend: `npm run typecheck && npm run lint`, then manual check via
  `run-payroll-frontend` skill — confirm Data Scope column hidden for
  Companies/Bank rows in Permission Settings, confirm "Full Organization"
  option gone, confirm Department form's inline category creation works.
