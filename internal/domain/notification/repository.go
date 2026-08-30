package notification

import "context"

// Repository is a port. The adapters layer implements this port with
// real SQL. The application layer, and the Event domain's Notifier
// adapter, only call this port.
type Repository interface {
	// Create saves a new Notification.
	Create(ctx context.Context, n *Notification) error

	// ListForUser returns every Notification for one user, newest first.
	ListForUser(ctx context.Context, userID string) ([]Notification, error)

	// MarkRead sets one Notification's Read field to true. MarkRead only
	// matches a Notification that belongs to userID.
	MarkRead(ctx context.Context, notificationID, userID string) error
}
