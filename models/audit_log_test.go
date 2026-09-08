package models

import "testing"

func TestNewAuditLogInputRejectsSensitiveFields(t *testing.T) {
	_, err := NewAuditLogInput(AuditLogInput{
		EventType:  AuditEventEmployeeUpdated,
		BeforeData: map[string]any{"password": "secret"},
	})
	if err == nil {
		t.Fatal("NewAuditLogInput accepted a sensitive field")
	}
}

func TestNewAuditLogInputMarshalsSafeObjectData(t *testing.T) {
	input, err := NewAuditLogInput(AuditLogInput{
		EventType:  AuditEventEmployeeUpdated,
		BeforeData: map[string]any{"first_name": "Ada"},
	})
	if err != nil {
		t.Fatalf("NewAuditLogInput returned error: %v", err)
	}
	if string(input.BeforeData) != `{"first_name":"Ada"}` {
		t.Fatalf("before data = %s", input.BeforeData)
	}
}
