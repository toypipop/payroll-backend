package service

import (
	"testing"
	"time"
)

func TestLoginAttemptLimiterWarnsLimitsAndClears(t *testing.T) {
	limiter := NewLoginAttemptLimiter()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	for i := 1; i <= loginAttemptLimit; i++ {
		result := limiter.RecordFailure(" USER@Example.TEST ")
		if result.Count != i {
			t.Fatalf("attempt %d: count = %d", i, result.Count)
		}
		if result.Warn != (i >= loginAttemptWarningAt && i < loginAttemptLimit) {
			t.Fatalf("attempt %d: warn = %t", i, result.Warn)
		}
		if result.Limited != (i == loginAttemptLimit) {
			t.Fatalf("attempt %d: limited = %t", i, result.Limited)
		}
	}
	if result := limiter.Check("user@example.test"); !result.Limited || result.RetryAfter != loginAttemptWindow {
		t.Fatalf("check = %+v, want active cooldown", result)
	}

	limiter.Clear("user@example.test")
	if result := limiter.Check("user@example.test"); result.Count != 0 || result.Warn || result.Limited {
		t.Fatalf("clear left attempt state: %+v", result)
	}
}
