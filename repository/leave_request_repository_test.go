package repository

import (
	"testing"
	"time"
)

func TestDaysByCalendarYearSplitsLeaveAtYearBoundary(t *testing.T) {
	days := daysByCalendarYear(
		time.Date(2026, time.December, 30, 0, 0, 0, 0, time.UTC),
		time.Date(2027, time.January, 2, 0, 0, 0, 0, time.UTC),
	)

	if days[2026] != 2 || days[2027] != 2 {
		t.Fatalf("days by year = %#v, want 2026:2 and 2027:2", days)
	}
}
