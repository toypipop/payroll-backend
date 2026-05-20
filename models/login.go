package models

// LoginRequest from login form
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse sent to client
type LoginResponse struct {
	ID                   uint     `json:"id"`
	FullName             string   `json:"full_name"`
	Roles                []string `json:"roles"`
	CompaniesID          uint     `json:"companies_id"`
	AccessibleCompanyIDs []uint   `json:"accessible_company_ids"`
}
