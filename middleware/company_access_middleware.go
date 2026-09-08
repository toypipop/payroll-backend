package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"payroll-backend/apperr"
	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// CompanyLifecycleExempt identifies global data and the company management
// control plane. Company profile/status writes perform their own locked check.
func CompanyLifecycleExempt(route string) bool {
	path := strings.TrimPrefix(route, "/api/v1")
	return path == "/companies" || path == "/companies/all" || path == "/companies/:id" || path == "/companies/:id/status" ||
		path == "/department-categories" || path == "/master/banks" || strings.HasPrefix(path, "/master/banks/")
}

// CompanyRecordKind is the ownership inventory for routes whose :id names a
// record rather than a company. A new ID route must declare its owner here;
// unknown kinds fail closed instead of bypassing company lifecycle policy.
func CompanyRecordKind(route string) string {
	path := strings.TrimPrefix(route, "/api/v1")
	if strings.Contains(path, ":transaction_id") {
		return "transactions"
	}
	if strings.Contains(path, ":cycle_id") {
		return "cycles"
	}
	if strings.Contains(path, ":employees_id") {
		return "employees"
	}
	if strings.HasPrefix(path, "/payroll/cycles/") {
		return "cycles"
	}
	if strings.HasPrefix(path, "/leaves/") {
		return "leaves"
	}
	if strings.HasPrefix(path, "/permissions/") {
		return "roles"
	}
	path = strings.TrimPrefix(path, "/master/")
	path = strings.TrimPrefix(path, "/")
	root := strings.Split(path, "/")[0]
	switch root {
	case "employees", "salaries", "employee-earnings", "employee-deductions", "timelogs", "cycle-earnings", "cycle-deductions", "earning-types", "deduction-types", "employment-types", "departments", "positions", "roles", "employee-company-access", "holidays", "day-offs", "work-locations", "leave-types":
		return root
	}
	return ""
}

// ActiveCompanies protects reads as well as writes, including ungated options
// and reports. Keep Auth's identity intact, but narrow the operational scope to
// active companies. CheckPermission also intersects its role list with this set.
func (g *Guard) ActiveCompanies() gin.HandlerFunc {
	return func(c *gin.Context) {
		if CompanyLifecycleExempt(c.FullPath()) {
			c.Next()
			return
		}
		// Read uploads before taking company locks, so a slow request body does
		// not hold up company status changes.
		requestedCompanies, records, err := companyRequestReferences(c)
		if err != nil {
			abortCompanyError(c, err)
			return
		}
		ids, _ := c.Get("accessible_company_ids")
		companyIDs, _ := ids.([]uint)
		session, err := g.companies.Begin(c.Request.Context(), companyIDs)
		if err != nil {
			abortCompanyError(c, err)
			return
		}
		defer session.Close()

		for _, id := range requestedCompanies {
			if err := session.RequireCompany(id); err != nil {
				abortCompanyError(c, err)
				return
			}
		}
		if err := session.RequireRecords(records); err != nil {
			abortCompanyError(c, err)
			return
		}
		path := strings.TrimPrefix(c.FullPath(), "/api/v1")
		// My Payslips is an employee-owned read without a module gate. Creates
		// that default their company also need the home-company state checked.
		if path == "/payroll/my-slips" || (c.Request.Method == http.MethodPost && len(requestedCompanies) == 0 && len(records) == 0) {
			if value, ok := c.Get("scope_user"); ok {
				if user, ok := value.(models.Employees); ok && user.CompaniesID != 0 {
					if err := session.RequireCompany(user.CompaniesID); err != nil {
						abortCompanyError(c, err)
						return
					}
				}
			}
		}
		active := session.ActiveCompanyIDs()
		if len(active) == 0 && path != "/companies/options" {
			abortCompanyError(c, apperr.CompanyInactive())
			return
		}
		c.Set("active_company_ids", active)
		c.Set("accessible_company_ids", active)
		c.Next()
	}
}

func abortCompanyError(c *gin.Context, err error) { httpx.RespondError(c, err); c.Abort() }

func companyRequestReferences(c *gin.Context) ([]uint, []repository.CompanyRecord, error) {
	companies := []uint{}
	records := []repository.CompanyRecord{}
	for _, param := range c.Params {
		kind := CompanyRecordKind(c.FullPath())
		if kind == "" {
			return nil, nil, apperr.Internal("Company ownership resolver is not configured")
		}
		id, err := strconv.ParseUint(param.Value, 10, 64)
		if err != nil || id == 0 {
			return nil, nil, apperr.Validation("Invalid record id")
		}
		records = append(records, repository.CompanyRecord{Kind: kind, ID: uint(id)})
	}
	for key, values := range c.Request.URL.Query() {
		for _, value := range values {
			if err := appendCompanyReference(key, json.Number(value), &companies, &records); err != nil {
				return nil, nil, err
			}
		}
	}
	if c.Request.Body != nil && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		const maxBody = 8 << 20
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBody+1))
		if err != nil {
			return nil, nil, apperr.Validation("Could not read request body")
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		if len(body) > maxBody {
			return nil, nil, apperr.Validation("Request body is too large")
		}
		if len(bytes.TrimSpace(body)) > 0 {
			var input any
			decoder := json.NewDecoder(bytes.NewReader(body))
			decoder.UseNumber()
			if err := decoder.Decode(&input); err != nil {
				return nil, nil, apperr.Validation("Invalid input format")
			}
			if err := collectCompanyReferences(input, &companies, &records); err != nil {
				return nil, nil, err
			}
		}
	}
	return companies, records, nil
}

func collectCompanyReferences(value any, companies *[]uint, records *[]repository.CompanyRecord) error {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if err := appendCompanyReference(key, item, companies, records); err != nil {
				return err
			}
			if err := collectCompanyReferences(item, companies, records); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range value {
			if err := collectCompanyReferences(item, companies, records); err != nil {
				return err
			}
		}
	}
	return nil
}

func appendCompanyReference(key string, value any, companies *[]uint, records *[]repository.CompanyRecord) error {
	key = strings.ToLower(key)
	kind := ""
	switch key {
	case "companies_id", "company_id":
		kind = "company"
	case "employees_id", "employee_id", "employee_ids", "employees_ids":
		kind = "employees"
	case "cycle_id":
		kind = "cycles"
	case "role_id", "roles_id":
		kind = "roles"
	default:
		return nil
	}
	if values, ok := value.([]any); ok {
		for _, item := range values {
			if err := appendCompanyReference(key, item, companies, records); err != nil {
				return err
			}
		}
		return nil
	}
	if value == nil {
		return nil
	}
	var raw string
	switch value := value.(type) {
	case json.Number:
		raw = value.String()
	case string:
		raw = value
	default:
		return apperr.Validation("Invalid company or record id")
	}
	if raw == "" {
		return nil
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return apperr.Validation("Invalid company or record id")
	}
	if id == 0 {
		return nil
	} // Required/default-company validation stays in the domain service.
	if kind == "company" {
		*companies = append(*companies, uint(id))
	} else {
		*records = append(*records, repository.CompanyRecord{Kind: kind, ID: uint(id)})
	}
	return nil
}
