package employees

import (
	"net/http"
	"time"

	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func UpdateSalariesStructure(c *gin.Context) {
	userID, dataScope := repository.GetContextAuth(c)

	var input struct {
		EmployeesID   uint    `json:"employees_id" binding:"required"`
		CompaniesID   uint    `json:"companies_id" binding:"required"`
		BaseSalaries  float64 `json:"base_salaries"`
		EffectiveDate string  `json:"effective_date" binding:"required"`
		DeletedAt     *bool   `json:"deleted_at"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsedDate, err := time.Parse("2006-01-02", input.EffectiveDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Expected YYYY-MM-DD"})
		return
	}

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	emp, ok := repository.ResolveEmployeeForCompany(c, currentUser, dataScope, input.EmployeesID, input.CompaniesID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employees not found or access denied"})
		return
	}

	var existing []models.SalariesStructure
	config.DB.Unscoped().Where("employees_id = ? AND companies_id = ?", input.EmployeesID, emp.CompaniesID).Order("id ASC").Find(&existing)

	shouldDelete := input.DeletedAt != nil && *input.DeletedAt

	if shouldDelete {
		for i := range existing {
			if existing[i].DeletedAt == 0 {
				if err := config.DB.Model(&models.SalariesStructure{}).Where("id = ?", existing[i].ID).Update("deleted_at", time.Now().UnixNano()).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable salaries record"})
					return
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Salaries structure disabled successfully",
			"data":    nil,
		})
		return
	}

	var activeRecords []models.SalariesStructure
	for _, rec := range existing {
		if rec.DeletedAt == 0 {
			activeRecords = append(activeRecords, rec)
		}
	}

	tx := config.DB.Begin()

	for i := range activeRecords {
		if err := tx.Model(&models.SalariesStructure{}).Where("id = ?", activeRecords[i].ID).Update("deleted_at", time.Now().UnixNano()).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to archive previous salaries record"})
			return
		}
	}

	salaries := models.SalariesStructure{
		CompaniesID:   emp.CompaniesID,
		EmployeesID:   input.EmployeesID,
		BaseSalaries:  input.BaseSalaries,
		EffectiveDate: parsedDate,
	}
	if err := tx.Create(&salaries).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create salaries record"})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message": "Salaries structure saved successfully",
	})
}
