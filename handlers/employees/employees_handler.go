package employees

import (
	"errors"
	"net/http"
	"strconv"

	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// employeesService is the domain entry point for this handler. It is resolved
// lazily so the package does not capture config.DB before ConnectDatabase runs.
func employeesService() *service.EmployeesService {
	return service.NewEmployeesService(repository.NewEmployeesRepository())
}

// requestScope resolves the caller's authorization context, replacing the
// "load currentUser then read data_scope" preamble each handler used to repeat.
func requestScope(c *gin.Context) (repository.AccessScope, bool) {
	scope, err := repository.NewAccessScope(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return repository.AccessScope{}, false
	}
	return scope, true
}

// respondServiceError maps domain errors onto HTTP without letting driver or
// SQL detail reach the client.
func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEmployeeNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found or access denied"})
	case errors.Is(err, service.ErrCompanyAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this company"})
	case errors.Is(err, service.ErrRoleCompanyMismatch):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not belong to selected company"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Request could not be completed"})
	}
}

func parseIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid employee id"})
		return 0, false
	}
	return uint(id), true
}

func GetEmployees(c *gin.Context) {
	scope, ok := requestScope(c)
	if !ok {
		return
	}

	employees, err := employeesService().List(scope)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, employees)
}

func CreateEmployees(c *gin.Context) {
	scope, ok := requestScope(c)
	if !ok {
		return
	}

	var input models.Employees
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	created, err := employeesService().Create(scope, input)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, created)
}

func UpdateEmployees(c *gin.Context) {
	scope, ok := requestScope(c)
	if !ok {
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	updated, err := employeesService().Update(scope, id, patch)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, updated)
}

func DeleteEmployees(c *gin.Context) {
	scope, ok := requestScope(c)
	if !ok {
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	if err := employeesService().Delete(scope, id); err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}
