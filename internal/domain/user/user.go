// Package user holds the User entity. This package holds no HTTP code.
// This package holds no database code. This package holds only business rules.
package user

import "errors"

// User is the core entity for a registered account.
type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
}

// Domain errors. Use cases and adapters check these errors with errors.Is.
var (
	ErrUsernameTaken      = errors.New("username or email already taken")
	ErrNotFound           = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
)
