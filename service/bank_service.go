package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Bank rule failures.
var (
	ErrBankCodeRequired = apperr.Validation("bank_code is required")
	ErrBankNameRequired = apperr.Validation("bank_name is required")
)

// ErrBankNotFound is re-exported so handlers import a single package.
var ErrBankNotFound = repository.ErrBankNotFound

// BankService owns the bank rules. Banks are global master data shared by every company, so nothing here is company-scoped.
type BankService struct {
	repo repository.BankRepository
}

// NewBankService wires the service to a repository implementation.
func NewBankService(repo repository.BankRepository) *BankService {
	return &BankService{repo: repo}
}

// List returns every bank, ordered by code.
func (s *BankService) List() ([]models.Bank, error) {
	return s.repo.List()
}

// Create adds a bank after trimming its fields.
func (s *BankService) Create(input models.Bank) (models.Bank, error) {
	code, name := strings.TrimSpace(input.BankCode), strings.TrimSpace(input.BankName)
	if code == "" {
		return models.Bank{}, ErrBankCodeRequired
	}
	if name == "" {
		return models.Bank{}, ErrBankNameRequired
	}

	bank := models.Bank{BankCode: code, BankName: name, SwiftCode: strings.TrimSpace(input.SwiftCode)}
	if err := s.repo.Create(&bank); err != nil {
		return models.Bank{}, err
	}
	return bank, nil
}

// Update rewrites a bank's code, name and swift code.
func (s *BankService) Update(id uint, input models.Bank) (models.Bank, error) {
	code, name := strings.TrimSpace(input.BankCode), strings.TrimSpace(input.BankName)
	if code == "" {
		return models.Bank{}, ErrBankCodeRequired
	}
	if name == "" {
		return models.Bank{}, ErrBankNameRequired
	}

	bank, err := s.repo.Find(id)
	if err != nil {
		return models.Bank{}, err
	}

	bank.BankCode = code
	bank.BankName = name
	bank.SwiftCode = strings.TrimSpace(input.SwiftCode)
	if err := s.repo.Save(&bank); err != nil {
		return models.Bank{}, err
	}
	return bank, nil
}

// Delete removes a bank; reference checks run in middleware before this.
func (s *BankService) Delete(id uint) error {
	bank, err := s.repo.Find(id)
	if err != nil {
		return err
	}
	return s.repo.Delete(&bank)
}
