package gallery

import "context"

// Repository is a port. The adapters layer implements this port with
// real SQL. The application layer only calls this port.
type Repository interface {
	ListForUser(ctx context.Context, userID string) ([]Event, error)
}
