package employees

import (
	"net/http"
	"time"

	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
			if existing[i].IsActive {
				if err := archiveSalaryRecord(config.DB, existing[i]); err != nil {
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

	// Block backdating into a period that's already been paid out and closed
	var closedCycleCount int64
	config.DB.Model(&models.PayrollCycle{}).
		Where("companies_id = ? AND is_closed = ? AND start_date <= ? AND end_date >= ?", emp.CompaniesID, true, parsedDate, parsedDate).
		Count(&closedCycleCount)
	if closedCycleCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Effective date falls within an already-closed payroll cycle and cannot be used"})
		return
	}

	// Effective date must not move backward from the most recent record
	// (active or not), so history never goes out of chronological order.
	if len(existing) > 0 {
		lastRecord := existing[len(existing)-1]
		if parsedDate.Before(lastRecord.EffectiveDate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Effective date cannot be before the most recent salary record's effective date"})
			return
		}
	}

	var activeRecords []models.SalariesStructure
	for _, rec := range existing {
		if rec.IsActive {
			activeRecords = append(activeRecords, rec)
		}
	}

	tx := config.DB.Begin()

	for i := range activeRecords {
		if err := archiveSalaryRecord(tx, activeRecords[i]); err != nil {
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
		IsActive:      true,
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

// archiveSalaryRecord retires a record that's no longer the current one. If it
// was ever actually used in a real payroll calculation, it's kept forever
// (IsActive=false, DeletedAt untouched) so history stays intact; otherwise it
// never mattered and is safe to erase outright.
func archiveSalaryRecord(db *gorm.DB, record models.SalariesStructure) error {
	var usedCount int64
	if err := db.Model(&models.PayrollTransaction{}).Where("salaries_structure_id = ?", record.ID).Count(&usedCount).Error; err != nil {
		return err
	}
	if usedCount > 0 {
		return db.Model(&models.SalariesStructure{}).Where("id = ?", record.ID).Update("is_active", false).Error
	}
	return db.Model(&models.SalariesStructure{}).Where("id = ?", record.ID).Update("deleted_at", time.Now().UnixNano()).Error
}
