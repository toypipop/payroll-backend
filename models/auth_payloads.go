package models

// LoginRequest represents the input from the login form
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the data sent back to the client
type LoginResponse struct {
	Token    string `json:"token"`
	FullName string `json:"full_name"`
	Roles     string `json:"roles"`
}
