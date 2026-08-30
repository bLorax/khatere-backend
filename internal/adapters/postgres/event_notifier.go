package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// EventNotifier implements domainevent.Notifier with a direct insert into
// the notifications table. This is a temporary adapter. A later step can
// add a Kafka-based adapter that publishes an event instead, without any
// change to the application layer that calls this port.
type EventNotifier struct {
	conn *sql.DB
}

func NewEventNotifier(conn *sql.DB) *EventNotifier {
	return &EventNotifier{conn: conn}
}

func (n *EventNotifier) NotifyTagRequest(ctx context.Context, toUserID, eventID, fromUserID string) error {
	return n.insert(ctx, toUserID, "tag_request", eventID, fromUserID)
}

func (n *EventNotifier) NotifyTagApproved(ctx context.Context, toUserID, eventID, fromUserID string) error {
	return n.insert(ctx, toUserID, "tag_approved", eventID, fromUserID)
}

func (n *EventNotifier) NotifyTagRejected(ctx context.Context, toUserID, eventID, fromUserID string) error {
	return n.insert(ctx, toUserID, "tag_rejected", eventID, fromUserID)
}

func (n *EventNotifier) insert(ctx context.Context, toUserID, notifType, eventID, fromUserID string) error {
	_, err := n.conn.ExecContext(ctx,
		`INSERT INTO notifications (id, user_id, type, event_id, from_user_id) VALUES ($1, $2, $3, $4, $5)`,
		uuid.New().String(), toUserID, notifType, eventID, fromUserID,
	)
	return err
}
