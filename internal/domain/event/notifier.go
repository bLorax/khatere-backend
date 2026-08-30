package event

import "context"

// Notifier is a port. The Event domain calls this port to tell another
// part of the system that something happened. Today an adapter implements
// this port with a direct SQL insert into the notifications table. A
// later step can add a Kafka adapter that publishes an event instead.
// The application layer does not need to change for that later step.
type Notifier interface {
	NotifyTagRequest(ctx context.Context, toUserID, eventID, fromUserID string) error
	NotifyTagApproved(ctx context.Context, toUserID, eventID, fromUserID string) error
	NotifyTagRejected(ctx context.Context, toUserID, eventID, fromUserID string) error
}
