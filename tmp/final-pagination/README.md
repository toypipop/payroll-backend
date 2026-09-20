# Pending pagination regression checks

Not executed yet: the user requested running the tests together later.
`catalog_page_test.go` belongs to package `repository` and uses the existing
`company_repository_test.go` SQL boundary. To run later, temporarily copy it into
`repository`, run `go test -tags pagination_scratch ./repository -run TestCatalogPage`, then remove only that
temporary copy. Keep this source under `tmp`.

Integration checks for the coordinated frontend/backend change:
- Use more than 25 records; page 2, global search, sorting, delete-last-row clamp.
- Company and employee scope must constrain both totals and returned rows.
- Companies status and payroll closed filters must affect the total before paging.
- Complete dropdown options must include entries beyond page 1.
- Bulk salary picker loads only on opening and can select active employees from
  beyond the current salary table page; preview/apply permissions remain enforced.
- Shared audit logs remain restricted to global/shared records.
- Invalid sort and invalid boolean filters return 400.
- Reports and employee salary history retain their existing complete responses.
