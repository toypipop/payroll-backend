package service

import (
	"math"
	"payroll-backend/apperr"
	"payroll-backend/models"
	"strings"
	"testing"
	"time"
)

func TestEmployeeProfilePatchValidation(t *testing.T) {
	for _, tc := range []struct {
		field string
		max   int
	}{{"employees_code", 100}, {"email", 150}, {"account_name", 200}, {"account_no", 50}} {
		value := strings.Repeat("ก", tc.max+1)
		if err := validateEmployeeProfilePatch(map[string]any{tc.field: value}); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatalf("%s: %v", tc.field, err)
		}
		if err := validateEmployeeProfilePatch(map[string]any{tc.field: 12}); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatal("non-string accepted")
		}
	}
	for _, field := range []string{"first_name", "last_name", "employees_code", "email"} {
		if err := validateEmployeeProfilePatch(map[string]any{field: "   "}); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatal("blank accepted")
		}
	}
	p := map[string]any{"employees_code": " E01 ", "email": " user+pay@example.com ", "account_no": " 01234 ", "account_name": strings.Repeat("😀", 200)}
	if err := validateEmployeeProfilePatch(p); err != nil || p["account_no"] != "01234" {
		t.Fatalf("normalized patch: %v", err)
	}
	for _, email := range []string{"a@", "a@localhost", "User <user@example.com>", "a b@example.com"} {
		if err := validateEmployeeProfilePatch(map[string]any{"email": email}); err == nil {
			t.Fatal("invalid email accepted")
		}
	}
	for _, p := range []map[string]any{{"is_active": "false"}, {"password": false}} {
		if err := validateEmployeeProfilePatch(p); err == nil {
			t.Fatal("bad type accepted")
		}
	}
}

func TestCompanyProfileValidation(t *testing.T) {
	valid := CompanyUpdate{Name: " Company ", TaxID: " 0123456789012 ", Address: " Address ", Phone: " 0812345678 "}
	result, err := normalizeCompanyProfile(valid)
	if err != nil || result.TaxID != "0123456789012" || result.Phone != "0812345678" {
		t.Fatal(err)
	}
	for _, field := range []string{"name", "tax", "address", "phone"} {
		input := valid
		switch field {
		case "name":
			input.Name = " "
		case "tax":
			input.TaxID = "123"
		case "address":
			input.Address = "1234"
		case "phone":
			input.Phone = "123"
		}
		if _, err := normalizeCompanyProfile(input); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatal("invalid company accepted")
		}
	}
}

func TestSharedTextRulesMatchClientUnicodeSemantics(t *testing.T) {
	if trimText("\ufeff Name \ufeff") != "Name" {
		t.Fatal("BOM trimming differs from JavaScript")
	}
	if trimText("\u0085Name\u0085") != "\u0085Name\u0085" {
		t.Fatal("NEL must be preserved")
	}
	if validLeaveTypeMaxDays(0.001) {
		t.Fatal("sub-cent leave limit became unlimited after database rounding")
	}
	if !validLeaveTypeMaxDays(0.29) {
		t.Fatal("binary floating-point noise rejected a valid decimal")
	}
}

func TestSalaryBoundsBeforeIO(t *testing.T) {
	for _, value := range []float64{-1, 1e13, 30.001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		svc := NewSalariesStructureService(nil, nil)
		if _, err := svc.Save(configTestScope(), SalaryInput{BaseSalaries: value}); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatal(err)
		}
		if err := svc.UpdateRecord(configTestScope(), 1, SalaryRecordEdit{BaseSalaries: value}); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatal(err)
		}
	}
	for _, value := range []float64{0, 0.01, maximumAdjustmentAmount} {
		if err := validateSalaryAmount(value, 0); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAttendanceFieldBounds(t *testing.T) {
	id := uint(1)
	for _, field := range []string{"remark", "location"} {
		input := TimeLogInput{EmployeesID: 1, WorkLocationID: &id}
		if field == "remark" {
			input.Remark = strings.Repeat("ก", 256)
		} else {
			input.LocationIn = strings.Repeat("😀", 256)
		}
		if err := validateTimeLogInput(input); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatal(err)
		}
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, days := range []int{1, 999, 1000} {
		input := LeaveInput{StartDate: start, EndDate: start.AddDate(0, 0, days-1), Reason: strings.Repeat("ก", 500)}
		err := validateLeaveInput(input)
		if (err == nil) != (days <= 999) {
			t.Fatalf("%d days: %v", days, err)
		}
	}
	if err := validateLeaveInput(LeaveInput{StartDate: start, EndDate: start, Reason: strings.Repeat("ก", 501)}); apperr.KindOf(err) != apperr.KindValidation {
		t.Fatal(err)
	}
	if err := normalizeEmployeeProfile(&models.Employees{}); err == nil {
		t.Fatal("empty employee accepted")
	}
}
