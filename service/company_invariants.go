package service

import "payroll-backend/apperr"

var ErrCompanyImmutable = apperr.Validation("Company cannot be changed after creation")

func requireSameCompany(existing, requested uint) error {
	if existing != requested {
		return ErrCompanyImmutable
	}
	return nil
}
