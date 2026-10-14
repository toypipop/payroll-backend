package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEqualCountPermissionChangesHaveDifferentAuditSnapshots(t *testing.T) {
	before := []RolesModuleAccess{{ModuleID: 2, CanView: true, DataScope: ScopeOwn}}
	after := []RolesModuleAccess{{ModuleID: 2, CanView: true, CanDelete: true, DataScope: ScopeCompanies}}
	b, _ := json.Marshal(PermissionAuditData(before))
	a, _ := json.Marshal(PermissionAuditData(after))
	if string(a) == string(b) || !strings.Contains(string(a), `"can_delete":true`) || !strings.Contains(string(b), `"data_scope":"Own"`) {
		t.Fatal("grant/scope difference missing")
	}
	if _, err := NewAuditLogInput(AuditLogInput{EventType: AuditEventPermissionUpdated, BeforeData: PermissionAuditData(before), AfterData: PermissionAuditData(after)}); err != nil {
		t.Fatal(err)
	}
}
