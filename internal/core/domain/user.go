package domain

import "time"

// UserStatus represents the lifecycle state of a user account.
type UserStatus string

const (
	UserStatusActive   UserStatus = "ACTIVE"
	UserStatusInactive UserStatus = "INACTIVE"
)

// User is the core user entity.
type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string // bcrypt/argon2id hash — never plaintext
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsActive returns true if the user account is in ACTIVE status.
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}
