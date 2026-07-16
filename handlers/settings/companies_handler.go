package settings

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// --- COMPANIES ---
// Companies has no owning company of its own. /companies/options is limited to
// the caller's accessible companies (home + employee_company_access), while
// /companies/all keeps the full list for the management page.

func GetCompanies(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)
	// AccessibleCompanyIDs (home + employee_company_access), not
	// GetAuthorizedCompanyIDs: the latter is scoped to companies whose role can
	// view CompaniesPage, which company-level ADMIN intentionally lacks.
	companyIDs := repository.AccessibleCompanyIDs(userID)

	var companies []models.Companies
	if err := config.DB.Where("id IN ?", companyIDs).Order("id ASC").Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, companies)
}

func GetAllCompanies(c *gin.Context) {
	var companies []models.Companies
	if err := config.DB.Order("id desc").Find(&companies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch companies"})
		return
	}
	c.JSON(http.StatusOK, companies)
}

// CreateCompanies bootstraps a usable tenant in one transaction: the company
// itself, a default ADMIN role with full permissions on every active module
// (except CompaniesPage, which stays SYSTEM_ADMIN-only per seed convention),
// and company access for the creator so the new company is immediately usable.
func CreateCompanies(c *gin.Context) {
	var companies models.Companies
	if err := c.ShouldBindJSON(&companies); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := repository.GetContextAuth(c)

	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&companies).Error; err != nil {
			return err
		}

		adminRole := models.Roles{
			CompaniesID: companies.ID,
			RolesName:   "ADMIN",
			Description: "Company Admin - full access to all modules",
		}
		if err := tx.Create(&adminRole).Error; err != nil {
			return err
		}

		var modules []models.AppModule
		if err := tx.Where("is_active = ?", true).Find(&modules).Error; err != nil {
			return err
		}

		accesses := make([]models.RolesModuleAccess, 0, len(modules))
		for _, m := range modules {
			if m.ModuleKey == "CompaniesPage" {
				continue
			}
			accesses = append(accesses, models.RolesModuleAccess{
				CompaniesID: companies.ID,
				RolesID:     adminRole.ID,
				ModuleID:    m.ID,
				CanView:     true,
				CanCreate:   true,
				CanEdit:     true,
				CanDelete:   true,
				DataScope:   models.ScopeCompanies,
			})
		}
		if len(accesses) > 0 {
			if err := tx.Create(&accesses).Error; err != nil {
				return err
			}
		}

		if userID != 0 {
			creatorAccess := models.EmployeeCompanyAccess{
				EmployeeID: userID,
				CompanyID:  companies.ID,
				RoleID:     adminRole.ID,
				IsActive:   true,
			}
			if err := tx.Create(&creatorAccess).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create companies"})
		return
	}
	c.JSON(http.StatusCreated, companies)
}

func UpdateCompanies(c *gin.Context) {
	id := c.Param("id")

	var companies models.Companies
	if err := config.DB.First(&companies, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Companies not found"})
		return
	}

	var updateData struct {
		Name    string `json:"name"`
		TaxID   string `json:"tax_id"`
		Address string `json:"address"`
		Phone   string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config.DB.Model(&companies).Updates(updateData)
	c.JSON(http.StatusOK, companies)
}

func DeleteCompanies(c *gin.Context) {
	id := c.Param("id")

	result := config.DB.Delete(&models.Companies{}, id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Companies not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}
