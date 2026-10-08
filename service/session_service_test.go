package service

import (
	"testing"
	"time"

	"payroll-backend/models"
)

type memorySessionRepository struct {
	sessions map[string]*models.AuthSession
}

func (r *memorySessionRepository) Create(session *models.AuthSession) error {
	if r.sessions == nil {
		r.sessions = map[string]*models.AuthSession{}
	}
	r.sessions[session.TokenID] = session
	return nil
}

func (r *memorySessionRepository) IsActive(tokenID string, employeeID uint, now time.Time) (bool, error) {
	session := r.sessions[tokenID]
	return session != nil && session.EmployeeID == employeeID && session.RevokedAt == nil && session.ExpiresAt.After(now), nil
}

func (r *memorySessionRepository) Revoke(tokenID string, employeeID uint, now time.Time) error {
	if session := r.sessions[tokenID]; session != nil && session.EmployeeID == employeeID && session.RevokedAt == nil {
		session.RevokedAt = &now
	}
	return nil
}

func (r *memorySessionRepository) RevokeAll(employeeID uint, now time.Time) error {
	for _, session := range r.sessions {
		if session.EmployeeID == employeeID && session.RevokedAt == nil {
			session.RevokedAt = &now
		}
	}
	return nil
}

func (r *memorySessionRepository) DeleteExpired(now time.Time) error {
	for tokenID, session := range r.sessions {
		if !session.ExpiresAt.After(now) {
			delete(r.sessions, tokenID)
		}
	}
	return nil
}

func TestSessionLifecycleRevokesOnlyTheSelectedDevice(t *testing.T) {
	repo := &memorySessionRepository{}
	sessions := NewSessionService(repo)
	now := time.Now()
	first, err := sessions.Create(7, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	second, err := sessions.Create(7, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) < 32 {
		t.Fatal("sessions must receive independent high-entropy identifiers")
	}

	if err := sessions.Revoke(first, 7, now); err != nil {
		t.Fatal(err)
	}
	if active, _ := sessions.IsActive(first, 7, now); active {
		t.Fatal("revoked session remained active")
	}
	if active, _ := sessions.IsActive(second, 7, now); !active {
		t.Fatal("revoking one device logged out another device")
	}

	if err := sessions.RevokeAll(7, now); err != nil {
		t.Fatal(err)
	}
	if active, _ := sessions.IsActive(second, 7, now); active {
		t.Fatal("revoke all left a session active")
	}
}

func TestSessionCleanupRemovesOnlyExpiredSessions(t *testing.T) {
	repo := &memorySessionRepository{}
	sessions := NewSessionService(repo)
	now := time.Now()
	expired, err := sessions.Create(7, now.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	active, err := sessions.Create(7, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.PruneExpired(now); err != nil {
		t.Fatal(err)
	}
	if _, exists := repo.sessions[expired]; exists {
		t.Fatal("expired session was retained")
	}
	if _, exists := repo.sessions[active]; !exists {
		t.Fatal("active session was removed")
	}
}
