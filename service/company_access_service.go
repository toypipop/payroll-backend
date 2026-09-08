package service

import (
	"context"
	"payroll-backend/apperr"
	"payroll-backend/repository"
)

// CompanyAccessService applies lifecycle policy independently of HTTP and permissions.
// Authentication still sees inactive companies so managers can reactivate them.
type CompanyAccessService struct {
	repo repository.CompanyAccessRepository
}
type CompanyAccessSession struct {
	lease  repository.CompanyAccessLease
	states map[uint]bool
	active []uint
}

func NewCompanyAccessService(repo repository.CompanyAccessRepository) *CompanyAccessService {
	return &CompanyAccessService{repo: repo}
}

func (s *CompanyAccessService) Begin(ctx context.Context, ids []uint) (*CompanyAccessSession, error) {
	lease, err := s.repo.LockCompanies(ctx, ids)
	if err != nil {
		return nil, err
	}
	session := &CompanyAccessSession{lease: lease, states: map[uint]bool{}, active: []uint{}}
	for _, state := range lease.States() {
		session.states[state.ID] = state.IsActive
		if state.IsActive {
			session.active = append(session.active, state.ID)
		}
	}
	return session, nil
}

func (s *CompanyAccessSession) ActiveCompanyIDs() []uint { return s.active }
func (s *CompanyAccessSession) Close()                   { s.lease.Close() }
func (s *CompanyAccessSession) RequireCompany(id uint) error {
	active, found := s.states[id]
	if !found {
		return apperr.Forbidden("Company access denied")
	}
	if !active {
		return apperr.CompanyInactive()
	}
	return nil
}

func (s *CompanyAccessSession) RequireRecords(records []repository.CompanyRecord) error {
	seen := map[repository.CompanyRecord]bool{}
	for _, record := range records {
		if seen[record] {
			continue
		}
		seen[record] = true
		id, err := s.lease.RecordCompany(record)
		if err != nil {
			return err
		}
		if err := s.RequireCompany(id); err != nil {
			return err
		}
	}
	return nil
}
