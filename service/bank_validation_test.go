package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"strings"
	"testing"
)

type bankValidationRepo struct {
	repository.BankRepository
	writes int
	saved  models.Bank
	audit  *models.AuditLog
}

func (r *bankValidationRepo) Find(id uint) (models.Bank, error) { return models.Bank{ID: id}, nil }
func (r *bankValidationRepo) Create(row *models.Bank, audit *models.AuditLog) error {
	r.writes++
	r.saved = *row
	r.audit = audit
	return nil
}
func (r *bankValidationRepo) Save(row *models.Bank, audit *models.AuditLog) error {
	return r.Create(row, audit)
}

func TestBankValidationCreateAndUpdate(t *testing.T) {
	for _, update := range []bool{false, true} {
		check := func(input models.Bank, valid bool) {
			t.Helper()
			repo := &bankValidationRepo{}
			svc := NewBankService(repo)
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 7, input)
			} else {
				_, err = svc.Create(configTestScope(), input)
			}
			if !valid {
				if apperr.KindOf(err) != apperr.KindValidation || repo.writes != 0 || repo.audit != nil {
					t.Fatalf("invalid bank saved: update=%v err=%v", update, err)
				}
				return
			}
			if err != nil || repo.writes != 1 || repo.audit == nil || repo.saved.BankCode != strings.TrimSpace(input.BankCode) || repo.saved.BankName != strings.TrimSpace(input.BankName) || repo.saved.SwiftCode != strings.TrimSpace(input.SwiftCode) {
				t.Fatalf("bank normalization failed: update=%v err=%v saved=%+v", update, err, repo.saved)
			}
			if update && repo.saved.ID != 7 {
				t.Fatal("update lost record identity")
			}
		}
		check(models.Bank{BankCode: " BBL ", BankName: " ธนาคาร ", SwiftCode: " \t "}, true)
		check(models.Bank{BankCode: " \t ", BankName: "Bank"}, false)
		check(models.Bank{BankCode: "BBL", BankName: " \t "}, false)
		for _, field := range []struct {
			name    string
			maximum int
		}{{"code", 10}, {"name", 200}, {"swift", 20}} {
			for _, character := range []string{"A", "ก", "😀"} {
				for _, extra := range []int{0, 1} {
					input := models.Bank{BankCode: "BBL", BankName: "Bank"}
					value := " " + strings.Repeat(character, field.maximum+extra) + " "
					switch field.name {
					case "code":
						input.BankCode = value
					case "name":
						input.BankName = value
					case "swift":
						input.SwiftCode = value
					}
					check(input, extra == 0)
				}
			}
		}
	}
}
