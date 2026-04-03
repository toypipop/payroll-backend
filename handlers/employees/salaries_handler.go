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

	var emp models.Employees
	if err := config.DB.Scopes(
		repository.ApplyScopes(currentUser, dataScope, "employees.companies_id", "employees.id"),
	).Where("id = ?", input.EmployeesID).First(&emp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employees not found or access denied"})
		return
	}

	var existing []models.SalariesStructure
	config.DB.Unscoped().Where("employees_id = ?", input.EmployeesID).Order("id ASC").Find(&existing)

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

	if len(activeRecords) > 0 {
		tx := config.DB.Begin()
		for i := 1; i < len(activeRecords); i++ {
			if err := tx.Exec("UPDATE salaries_structure SET deleted_at = ? WHERE id = ?", time.Now().UnixNano(), activeRecords[i].ID).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clean duplicate records"})
				return
			}
		}
		if err := tx.Model(&models.SalariesStructure{}).Where("id = ?", activeRecords[0].ID).Updates(map[string]interface{}{
			"base_salaries":  input.BaseSalaries,
			"effective_date": parsedDate,
		}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update salaries record"})
			return
		}
		tx.Commit()
	} else {
		salaries := models.SalariesStructure{
			EmployeesID:   input.EmployeesID,
			BaseSalaries:  input.BaseSalaries,
			EffectiveDate: parsedDate,
		}
		if err := config.DB.Create(&salaries).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create salaries record"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Salaries structure updated successfully",
	})
}
