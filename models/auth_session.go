package models

import "time"

// AuthSession is the server-side revocation record for one signed-in browser or device.
// TokenID is random and appears only inside the signed JWT; RevokedAt makes a copied
// cookie unusable immediately without waiting for the JWT expiry.
type AuthSession struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`

	EmployeeID uint       `gorm:"not null;index" json:"-"`
	TokenID    string     `gorm:"not null;uniqueIndex;size:64" json:"-"`
	ExpiresAt  time.Time  `gorm:"not null;index" json:"-"`
	RevokedAt  *time.Time `gorm:"index" json:"-"`
}
