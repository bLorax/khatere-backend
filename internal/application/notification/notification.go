// Package notification holds the use cases for the Notification domain.
package notification

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	domainnotif "yadegar/internal/domain/notification"
	"yadegar/internal/telemetry"
)

// ListNotificationsUseCase returns a user's notifications.
type ListNotificationsUseCase struct {
	repo domainnotif.Repository
}

func NewListNotificationsUseCase(repo domainnotif.Repository) *ListNotificationsUseCase {
	return &ListNotificationsUseCase{repo: repo}
}

// ReadNotificationUseCase marks one notification as read.
type ReadNotificationUseCase struct {
	repo domainnotif.Repository
}

func NewReadNotificationUseCase(repo domainnotif.Repository) *ReadNotificationUseCase {
	return &ReadNotificationUseCase{repo: repo}
}

func (uc *ListNotificationsUseCase) Execute(ctx context.Context, userID string) ([]domainnotif.Notification, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "ListNotificationsUseCase.Execute")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", userID))

	notifs, err := uc.repo.ListForUser(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return notifs, nil
}

func (uc *ReadNotificationUseCase) Execute(ctx context.Context, notificationID, userID string) error {
	ctx, span := telemetry.Tracer().Start(ctx, "ReadNotificationUseCase.Execute")
	defer span.End()

	if err := uc.repo.MarkRead(ctx, notificationID, userID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}
