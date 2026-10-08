package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"payroll-backend/models"
	"payroll-backend/repository"
)

// SessionService owns the lifecycle of independently revocable login sessions.
type SessionService struct {
	repo repository.SessionRepository
}

func NewSessionService(repo repository.SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

func (s *SessionService) Create(employeeID uint, expiresAt time.Time) (string, error) {
	if s == nil || s.repo == nil {
		return "", errors.New("authentication session store is not configured")
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	tokenID := base64.RawURLEncoding.EncodeToString(bytes)
	if err := s.repo.Create(&models.AuthSession{EmployeeID: employeeID, TokenID: tokenID, ExpiresAt: expiresAt}); err != nil {
		return "", err
	}
	return tokenID, nil
}

func (s *SessionService) IsActive(tokenID string, employeeID uint, now time.Time) (bool, error) {
	if s == nil || s.repo == nil {
		return false, errors.New("authentication session store is not configured")
	}
	return s.repo.IsActive(tokenID, employeeID, now)
}

func (s *SessionService) Revoke(tokenID string, employeeID uint, now time.Time) error {
	if s == nil || s.repo == nil {
		return errors.New("authentication session store is not configured")
	}
	return s.repo.Revoke(tokenID, employeeID, now)
}

func (s *SessionService) RevokeAll(employeeID uint, now time.Time) error {
	if s == nil || s.repo == nil {
		return errors.New("authentication session store is not configured")
	}
	return s.repo.RevokeAll(employeeID, now)
}
