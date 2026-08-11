package models

// ModuleScopeLevel says whether a module's rows are owned by a person or only
// by a company. This is a *fact about the handler*, not a policy choice: a
// module is employee-level exactly when its queries go through
// repository.ApplyEmployeeScopes, and company-level when they go through
// repository.ApplyCompanyScopes.
//
// Getting this wrong reintroduces the bug it exists to prevent — an admin
// setting DataScope=Own on a company-level module, the UI reporting success,
// and nothing actually being restricted.
type ModuleScopeLevel string

const (
	ScopeLevelCompany  ModuleScopeLevel = "company"
	ScopeLevelEmployee ModuleScopeLevel = "employee"
)

// Permission actions, matching the strings CheckPermission accepts.
const (
	ActionView   = "VIEW"
	ActionCreate = "CREATE"
	ActionEdit   = "EDIT"
	ActionDelete = "DELETE"
)

// ModuleCapability declares what the backend can actually enforce for one
// module, so the permission UI never offers a control the server ignores.
type ModuleCapability struct {
	// ScopeLevel decides which DataScope values are meaningful.
	ScopeLevel ModuleScopeLevel
	// AllowedActions lists only the actions some route actually gates on.
	// A module whose DELETE route is gated by CheckPermission(..., "EDIT")
	// does not list DELETE — ticking it would do nothing.
	AllowedActions []string
	// ActionsAtomic presents create/edit/delete as one on/off toggle instead
	// of four independent checkboxes. Unlike the two fields above this is a
	// presentation policy, not a technical limit.
	ActionsAtomic bool
}

// AllowedScopes returns the DataScope values that actually change the query
// result for this module. Derived from ScopeLevel so the two can never
// disagree. ScopeAll is deliberately absent: it is not assignable through the
// permission API.
func (mc ModuleCapability) AllowedScopes() []string {
	if mc.ScopeLevel == ScopeLevelEmployee {
		return []string{string(ScopeOwn), string(ScopeDepartment), string(ScopeCompanies)}
	}
	return []string{string(ScopeCompanies)}
}

// AllowsAction reports whether ticking this action has any effect.
func (mc ModuleCapability) AllowsAction(action string) bool {
	for _, a := range mc.AllowedActions {
		if a == action {
			return true
		}
	}
	return false
}

// AllowsScope reports whether this DataScope is honoured for this module.
func (mc ModuleCapability) AllowsScope(scope DataScope) bool {
	for _, s := range mc.AllowedScopes() {
		if s == string(scope) {
			return true
		}
	}
	return false
}

func companyLevel(actions ...string) ModuleCapability {
	return ModuleCapability{ScopeLevel: ScopeLevelCompany, AllowedActions: actions}
}

func employeeLevel(actions ...string) ModuleCapability {
	return ModuleCapability{ScopeLevel: ScopeLevelEmployee, AllowedActions: actions}
}

func atomic(mc ModuleCapability) ModuleCapability {
	mc.ActionsAtomic = true
	return mc
}

// moduleCapabilities is the single source of truth. AppModule rows are
// refreshed from this on every boot (see migrations.SyncModuleCapabilities),
// so the database is a projection of this map and cannot drift from it.
//
// When adding a module: the ScopeLevel must match which Apply*Scopes its
// handler calls, and AllowedActions must match the action strings its routes
// pass to CheckPermission. Boot fails loudly if a seeded module is missing here.
var moduleCapabilities = map[ModuleKey]ModuleCapability{
	// --- Employee-level: rows carry an employee column, so Own/Department bite ---
	"EmployeesListPage":         employeeLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"SalariesStructurePage":     employeeLevel(ActionView, ActionCreate, ActionDelete),
	"EmployeeEarningsPage":      employeeLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"EmployeeDeductionsPage":    employeeLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"TimelogsListPage":          employeeLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"LeaveRequestsPage":         employeeLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"LeaveApprovedPage":         employeeLevel(ActionView, ActionEdit),
	"EarningSummaryPage":        employeeLevel(ActionView),
	"DeductionSummaryPage":      employeeLevel(ActionView),
	"CycleEarningsPage":         employeeLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"CycleDeductionsPage":       employeeLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"EmployeeCompanyAccessPage": employeeLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),

	// DashboardPage is deliberately absent: it is not permission-controlled at
	// all. Like My Payslips it is a universal landing page — App.tsx hardcodes
	// the /dashboard route and redirects "/" to it, so revoking the module
	// only ever hid the sidebar link while the page stayed reachable. What a
	// user actually sees there is decided by the modules it reads from
	// (EmployeesListPage, PayrollCyclesPage), each enforcing its own scope.

	// --- Company-level: rows belong to a company, no per-person meaning ---
	"PayrollCyclesPage": companyLevel(ActionView, ActionCreate, ActionEdit),
	"RunPayrollPage":            companyLevel(ActionView, ActionEdit),
	"TransactionsPage":          companyLevel(ActionView),
	"PayrollGlobalSettingsPage": companyLevel(ActionView, ActionEdit),
	// Delete routes for these two are gated by EDIT, so DELETE is not offered.
	"EarningTypesPage":   companyLevel(ActionView, ActionCreate, ActionEdit),
	"DeductionTypesPage": companyLevel(ActionView, ActionEdit),
	"EmploymentTypePage": companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"AttendanceSettingsPage": companyLevel(ActionView, ActionEdit),
	"DepartmentSettings":     companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"PositionSettings":       companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	// Every role write route is gated by EDIT.
	"RolesPage":           companyLevel(ActionView, ActionEdit),
	"PermissionSettings":  companyLevel(ActionView, ActionEdit),
	"HolidayCalendarPage": companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"DayOffPage":          companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"WorkLocationPage":    companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),
	"MasterLeaveTypePage": companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete),

	// Reports are GET-only, so View is the only action that exists.
	"PayslipsReportPage":     companyLevel(ActionView),
	"BankTransferReportPage": companyLevel(ActionView),
	"TaxFormsReportPage":     companyLevel(ActionView),
	"SSOReportPage":          companyLevel(ActionView),
	"PayrollAnalysisPage":    companyLevel(ActionView),

	// Presented as a single on/off toggle.
	"CompaniesPage": atomic(companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete)),
	"MasterBankPage": atomic(companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete)),
	"HelpPage":       atomic(companyLevel(ActionView, ActionCreate, ActionEdit, ActionDelete)),
}

// CapabilityFor looks up a module's declared capability.
func CapabilityFor(key ModuleKey) (ModuleCapability, bool) {
	mc, ok := moduleCapabilities[key]
	return mc, ok
}
