package models

import (
	"gorm.io/gorm/schema"
	"sync"
	"testing"
)

func TestCopiedTextColumnsRetainSourceCapacity(t *testing.T) {
	for _, tc := range []struct {
		source      any
		sourceField string
		target      any
		targetField string
	}{
		{LeaveTypeSetting{}, "Name", LeaveRequest{}, "LeaveType"},
		{LeaveTypeSetting{}, "Name", LeaveBalance{}, "LeaveType"},
		{Bank{}, "BankName", PayrollTransaction{}, "BankName"},
		{EmploymentType{}, "Name", PayrollTransaction{}, "EmploymentTypeName"},
		{Position{}, "PosName", PayrollTransaction{}, "PositionName"},
		{Department{}, "DeptName", PayrollTransaction{}, "DepartmentName"},
		{Employees{}, "AccountName", PayrollTransaction{}, "BankAccountName"},
		{Employees{}, "AccountNo", PayrollTransaction{}, "BankAccountNo"},
	} {
		source, err := schema.Parse(tc.source, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatal(err)
		}
		target, err := schema.Parse(tc.target, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatal(err)
		}
		if target.FieldsByName[tc.targetField].Size < source.FieldsByName[tc.sourceField].Size {
			t.Fatalf("%s.%s cannot hold %s.%s", target.Name, tc.targetField, source.Name, tc.sourceField)
		}
	}
}
