package postgres

import (
	"context"

	domainnotif "yadegar/internal/domain/notification"
)

// EventNotifier implements domainevent.Notifier. EventNotifier delegates
// every write to NotificationRepository, so the notifications table has
// one owner. A later step can add a Kafka-based adapter instead, with no
// change to the Event application layer that calls this port.
type EventNotifier struct {
	notifications domainnotif.Repository
}

func NewEventNotifier(notifications domainnotif.Repository) *EventNotifier {
	return &EventNotifier{notifications: notifications}
}

func (n *EventNotifier) NotifyTagRequest(ctx context.Context, toUserID, eventID, fromUserID string) error {
	return n.create(ctx, toUserID, domainnotif.TypeTagRequest, eventID, fromUserID)
}

func (n *EventNotifier) NotifyTagApproved(ctx context.Context, toUserID, eventID, fromUserID string) error {
	return n.create(ctx, toUserID, domainnotif.TypeTagApproved, eventID, fromUserID)
}

func (n *EventNotifier) NotifyTagRejected(ctx context.Context, toUserID, eventID, fromUserID string) error {
	return n.create(ctx, toUserID, domainnotif.TypeTagRejected, eventID, fromUserID)
}

func (n *EventNotifier) create(ctx context.Context, toUserID, notifType, eventID, fromUserID string) error {
	return n.notifications.Create(ctx, &domainnotif.Notification{
		UserID: toUserID, Type: notifType, EventID: eventID, FromUserID: fromUserID,
	})
}
