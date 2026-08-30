package photo

import "context"

// Repository is a port. The adapters layer implements this port with
// real SQL. The application layer only calls this port.
type Repository interface {
	// Create saves a new Photo's record.
	Create(ctx context.Context, p *Photo) error

	// CountForEvent returns how many photos an Event already has.
	CountForEvent(ctx context.Context, eventID string) (int, error)

	// ListForEvent returns an Event's photos, oldest first. limit caps
	// the number of rows returned; pass 0 for no cap.
	ListForEvent(ctx context.Context, eventID string, limit int) ([]Photo, error)
}
