package service

import (
	"strings"
	"sync"
	"time"
)

const (
	loginAttemptWarningAt = 5
	loginAttemptLimit     = 10
	loginAttemptWindow    = 15 * time.Minute
)

// LoginAttemptLimiter is deliberately keyed only by a normalized email address.
// It is process-local, which is appropriate while the API runs as one instance.
// A multi-instance deployment must replace it with a shared store such as Redis.
type LoginAttemptLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
	now      func() time.Time
}

type loginAttempt struct {
	count        int
	expiresAt    time.Time
	limitedUntil time.Time
}

type LoginAttemptResult struct {
	Count      int
	Warn       bool
	Limited    bool
	RetryAfter time.Duration
}

func NewLoginAttemptLimiter() *LoginAttemptLimiter {
	return &LoginAttemptLimiter{
		attempts: make(map[string]loginAttempt),
		now:      time.Now,
	}
}

// NormalizeLoginEmail gives equivalent email spellings one shared attempt budget.
func NormalizeLoginEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Check reports whether an address is in its temporary cooldown without adding
// another failed attempt. This avoids extending a lock through repeated retries.
func (l *LoginAttemptLimiter) Check(email string) LoginAttemptResult {
	return l.withAttempt(email, false)
}

// RecordFailure increments the failed-login count and applies a 15-minute
// cooldown on the tenth failure. A successful login must call Clear.
func (l *LoginAttemptLimiter) RecordFailure(email string) LoginAttemptResult {
	return l.withAttempt(email, true)
}

func (l *LoginAttemptLimiter) Clear(email string) {
	key := NormalizeLoginEmail(email)
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func (l *LoginAttemptLimiter) withAttempt(email string, failed bool) LoginAttemptResult {
	key := NormalizeLoginEmail(email)
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[key]
	if ok && !attempt.expiresAt.After(now) && !attempt.limitedUntil.After(now) {
		delete(l.attempts, key)
		ok = false
	}
	if !ok {
		attempt = loginAttempt{}
	}
	if attempt.limitedUntil.After(now) {
		return LoginAttemptResult{Count: attempt.count, Limited: true, RetryAfter: attempt.limitedUntil.Sub(now)}
	}
	if !failed {
		return LoginAttemptResult{Count: attempt.count, Warn: attempt.count >= loginAttemptWarningAt}
	}

	attempt.count++
	attempt.expiresAt = now.Add(loginAttemptWindow)
	if attempt.count >= loginAttemptLimit {
		attempt.limitedUntil = now.Add(loginAttemptWindow)
		l.attempts[key] = attempt
		return LoginAttemptResult{Count: attempt.count, Limited: true, RetryAfter: loginAttemptWindow}
	}
	l.attempts[key] = attempt
	return LoginAttemptResult{Count: attempt.count, Warn: attempt.count >= loginAttemptWarningAt}
}
