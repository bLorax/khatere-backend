package user

import "context"

// Repository is a port. A port is an interface. The domain layer defines the port.
// The adapters layer implements the port. This rule keeps the domain layer free
// of database code.
type Repository interface {
	Create(ctx context.Context, u *User) error
	FindByIdentifier(ctx context.Context, identifier string) (*User, error)
}
