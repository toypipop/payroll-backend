package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrBankNotFound means the referenced bank row does not exist.
var ErrBankNotFound = apperr.NotFound("Bank not found")

// BankRepository is the persistence boundary for the banks domain.
type BankRepository interface {
	List() ([]models.Bank, error)
	Find(id uint) (models.Bank, error)
	Create(bank *models.Bank, audit *models.AuditLog) error
	Save(bank *models.Bank, audit *models.AuditLog) error
	Delete(bank *models.Bank, audit *models.AuditLog) error
}

type gormBankRepository struct {
	db *gorm.DB
}

// NewBankRepository returns the GORM-backed implementation on the given connection.
func NewBankRepository(db *gorm.DB) BankRepository {
	return &gormBankRepository{db: db}
}

// List returns every bank by code. Banks carry no companies_id, so no scope filter exists to apply.
func (r *gormBankRepository) List() ([]models.Bank, error) {
	var banks []models.Bank
	err := r.db.Order("bank_code").Find(&banks).Error
	return banks, err
}

func (r *gormBankRepository) Find(id uint) (models.Bank, error) {
	var bank models.Bank
	if err := r.db.First(&bank, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Bank{}, ErrBankNotFound
		}
		return models.Bank{}, err
	}
	return bank, nil
}

func (r *gormBankRepository) Create(bank *models.Bank, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(bank).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = bank.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormBankRepository) Save(bank *models.Bank, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(bank).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormBankRepository) Delete(bank *models.Bank, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(bank).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}
