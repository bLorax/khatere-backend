// Package notification holds the use cases for the Notification domain.
package notification

import (
	"context"

	domainnotif "yadegar/internal/domain/notification"
)

// ListNotificationsUseCase returns a user's notifications.
type ListNotificationsUseCase struct {
	repo domainnotif.Repository
}

func NewListNotificationsUseCase(repo domainnotif.Repository) *ListNotificationsUseCase {
	return &ListNotificationsUseCase{repo: repo}
}

func (uc *ListNotificationsUseCase) Execute(ctx context.Context, userID string) ([]domainnotif.Notification, error) {
	return uc.repo.ListForUser(ctx, userID)
}

// ReadNotificationUseCase marks one notification as read.
type ReadNotificationUseCase struct {
	repo domainnotif.Repository
}

func NewReadNotificationUseCase(repo domainnotif.Repository) *ReadNotificationUseCase {
	return &ReadNotificationUseCase{repo: repo}
}

func (uc *ReadNotificationUseCase) Execute(ctx context.Context, notificationID, userID string) error {
	return uc.repo.MarkRead(ctx, notificationID, userID)
}
