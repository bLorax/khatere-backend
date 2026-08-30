// Package httpapi (this file) holds the Notification domain's HTTP adapter.
package httpapi

import (
	"encoding/json"
	"net/http"

	appnotif "yadegar/internal/application/notification"
	"yadegar/internal/middleware"
)

type notificationItem struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	EventID      string `json:"event_id"`
	EventName    string `json:"event_name"`
	FromUser     string `json:"from_user"`
	Read         bool   `json:"read"`
	CreatedAt    string `json:"created_at"`
	MemberID     string `json:"member_id,omitempty"`
	MemberStatus string `json:"member_status,omitempty"`
}

// NotificationHandler wires HTTP routes to Notification use cases.
type NotificationHandler struct {
	list *appnotif.ListNotificationsUseCase
	read *appnotif.ReadNotificationUseCase
}

func NewNotificationHandler(list *appnotif.ListNotificationsUseCase, read *appnotif.ReadNotificationUseCase) *NotificationHandler {
	return &NotificationHandler{list: list, read: read}
}

func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	notifs, err := h.list.Execute(r.Context(), userID)
	if err != nil {
		http.Error(w, "could not load notifications", http.StatusInternalServerError)
		return
	}

	results := make([]notificationItem, 0, len(notifs))
	for _, n := range notifs {
		results = append(results, notificationItem{
			ID: n.ID, Type: n.Type, EventID: n.EventID, EventName: n.EventName,
			FromUser: n.FromUsername, Read: n.Read, CreatedAt: n.CreatedAt,
			MemberID: n.MemberID, MemberStatus: n.MemberStatus,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"results": results})
}

func (h *NotificationHandler) ReadNotification(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	notifID := r.PathValue("id")

	// Same behavior as the old handler: the error is not checked. A
	// notification ID that does not belong to this user, or does not
	// exist, still gets a 204 response.
	_ = h.read.Execute(r.Context(), notifID, userID)

	w.WriteHeader(http.StatusNoContent)
}
