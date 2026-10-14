package models

import (
	"encoding/json"
	"testing"
)

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

func TestAuditSensitiveGuardInspectsSerializedNestedContainers(t *testing.T) {
	for name, value := range map[string]any{
		"array":     []any{map[string]any{"password": "SECRET"}},
		"typed map": map[string]string{"access_token": "SECRET"},
		"raw JSON":  json.RawMessage(`{"cookie":"SECRET"}`),
		"struct": struct {
			Password string `json:"password"`
		}{Password: "SECRET"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewAuditLogInput(AuditLogInput{EventType: AuditEventEmployeeUpdated, AfterData: map[string]any{"items": value}})
			if err == nil {
				t.Fatal("sensitive nested value was accepted")
			}
		})
	}
	if _, err := NewAuditLogInput(AuditLogInput{EventType: AuditEventEmployeeUpdated, AfterData: map[string]any{"password_changed": true, "items": []map[string]any{{"module_id": 1, "can_view": true}}}}); err != nil {
		t.Fatalf("safe metadata rejected: %v", err)
	}
}
