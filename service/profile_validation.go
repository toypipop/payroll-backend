package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"regexp"
)

var emailPattern = regexp.MustCompile("^[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)+$")
var taxIDPattern = regexp.MustCompile(`^[0-9]{13}$`)
var phonePattern = regexp.MustCompile(`^[0-9]{9,10}$`)

func normalizeEmployeeProfile(input *models.Employees) error {
	if input.BankID != nil && *input.BankID == 0 {
		input.BankID = nil
	}
	if input.EmploymentTypeID != nil && *input.EmploymentTypeID == 0 {
		input.EmploymentTypeID = nil
	}
	patch := map[string]any{"employees_code": input.EmployeesCode, "email": input.Email, "first_name": input.FirstName, "last_name": input.LastName, "account_name": input.AccountName, "account_no": input.AccountNo}
	if err := validateEmployeeProfilePatch(patch); err != nil {
		return err
	}
	input.EmployeesCode = patch["employees_code"].(string)
	input.Email = patch["email"].(string)
	input.FirstName = patch["first_name"].(string)
	input.LastName = patch["last_name"].(string)
	input.AccountName = patch["account_name"].(string)
	input.AccountNo = patch["account_no"].(string)
	if input.DeptID == 0 || input.PosID == 0 {
		return apperr.Validation("Department and position are required")
	}
	return nil
}

func validateEmployeeProfilePatch(patch map[string]any) error {
	for _, field := range []struct {
		name             string
		minimum, maximum int
	}{
		{"employees_code", 2, 100}, {"email", 1, 150}, {"first_name", 1, 0}, {"last_name", 1, 0}, {"account_name", 0, 200}, {"account_no", 0, 50},
	} {
		raw, exists := patch[field.name]
		if !exists {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			return apperr.Validation(field.name + " must be text")
		}
		value = trimText(value)
		if err := validateText(field.name, value, field.minimum, field.maximum); err != nil {
			return err
		}
		if field.name == "email" && !emailPattern.MatchString(value) {
			return apperr.Validation("Invalid email address")
		}
		patch[field.name] = value
	}
	for _, field := range []string{"is_active", "password"} {
		if value, exists := patch[field]; exists {
			if field == "is_active" {
				if _, ok := value.(bool); !ok {
					return apperr.Validation("is_active must be boolean")
				}
			} else {
				if _, ok := value.(string); !ok {
					return apperr.Validation("password must be text")
				}
			}
		}
	}
	return nil
}

func normalizeCompanyProfile(input CompanyUpdate) (CompanyUpdate, error) {
	input.Name, input.TaxID, input.Address, input.Phone = trimText(input.Name), trimText(input.TaxID), trimText(input.Address), trimText(input.Phone)
	if err := validateText("name", input.Name, 1, 0); err != nil {
		return CompanyUpdate{}, err
	}
	if err := validateText("address", input.Address, 5, 0); err != nil {
		return CompanyUpdate{}, err
	}
	if !taxIDPattern.MatchString(input.TaxID) {
		return CompanyUpdate{}, apperr.Validation("Tax ID must be exactly 13 digits")
	}
	if !phonePattern.MatchString(input.Phone) {
		return CompanyUpdate{}, apperr.Validation("Phone must contain 9–10 digits")
	}
	return input, nil
}
