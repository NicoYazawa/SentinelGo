package model

import "time"

// AuthSession persists revocation independently of Redis availability.
type AuthSession struct {
	ID         string    `gorm:"primaryKey;size:36"`
	UserID     uint64    `gorm:"index;not null"`
	RefreshJTI string    `gorm:"size:36;not null"`
	ExpiresAt  time.Time `gorm:"index;not null"`
	Revoked    bool      `gorm:"not null;default:false"`
}

func (AuthSession) TableName() string { return "auth_sessions" }

type OAuthState struct {
	ID        string    `gorm:"primaryKey;size:36"`
	ExpiresAt time.Time `gorm:"index;not null"`
}
