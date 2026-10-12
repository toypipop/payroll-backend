package attendance

import (
	"payroll-backend/timeutil"
	"testing"
)

func TestLeaveDateWireContract(t *testing.T) {
	for _, value := range []string{"2026-10-12", "2026-10-12T00:00:00Z", "2026-10-11T17:00:00Z"} {
		date, err := parseLeaveDate(value)
		if err != nil || timeutil.DateKey(date) != "2026-10-12" || date.Hour() != 0 {
			t.Fatalf("%s: %v %v", value, date, err)
		}
	}
	for _, value := range []string{"", "2026-02-30", "2026-13-01", "2026-02-30T00:00:00Z"} {
		if _, err := parseLeaveDate(value); err == nil {
			t.Fatalf("invalid date accepted: %s", value)
		}
	}
}
