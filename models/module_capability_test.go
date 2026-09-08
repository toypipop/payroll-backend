package models

import "testing"

func TestGeneralUniversalPagesAreAtomicViewModules(t *testing.T) {
	for _, key := range []ModuleKey{"DashboardPage", "MyPayslipsPage"} {
		capability, ok := CapabilityFor(key)
		if !ok {
			t.Fatalf("%s must declare a capability", key)
		}
		if !capability.ActionsAtomic {
			t.Errorf("%s must use one atomic Access toggle", key)
		}
		if capability.ScopeLevel != ScopeLevelCompany {
			t.Errorf("%s scope level = %q, want company", key, capability.ScopeLevel)
		}
		if len(capability.AllowedActions) != 1 || capability.AllowedActions[0] != ActionView {
			t.Errorf("%s actions = %v, want [VIEW]", key, capability.AllowedActions)
		}
	}
}

func TestAuditLogsPageUsesCompanyAccessScope(t *testing.T) {
	capability, ok := CapabilityFor("AuditLogsPage")
	if !ok {
		t.Fatal("AuditLogsPage must declare a capability")
	}
	if capability.ActionsAtomic || len(capability.AllowedActions) != 1 || capability.AllowedActions[0] != ActionView {
		t.Fatalf("AuditLogsPage capability = %#v, want view-only non-atomic", capability)
	}
	if len(capability.AllowedScopes()) != 1 || !capability.AllowsScope(ScopeCompanies) {
		t.Fatalf("AuditLogsPage scopes = %v, want Companies only", capability.AllowedScopes())
	}
}
