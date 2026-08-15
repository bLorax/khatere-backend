package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"yadegar/internal/middleware"
)

type galleryEvent struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PhotoCount int    `json:"photo_count"`
	ApprovedAt string `json:"approved_at"`
}

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

func Gallery(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)

		rows, err := conn.Query(`
			SELECT e.id, e.name, m.created_at,
			       (SELECT count(*) FROM photos p WHERE p.event_id = e.id) AS photo_count
			FROM event_members m
			JOIN events e ON e.id = m.event_id
			WHERE m.user_id = $1 AND m.status = 'approved'
			ORDER BY m.created_at DESC`,
			userID,
		)
		if err != nil {
			http.Error(w, "could not load gallery", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		events := []galleryEvent{}
		for rows.Next() {
			var e galleryEvent
			if err := rows.Scan(&e.ID, &e.Name, &e.ApprovedAt, &e.PhotoCount); err != nil {
				http.Error(w, "could not load gallery", http.StatusInternalServerError)
				return
			}
			events = append(events, e)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"events": events})
	}
}

func ListNotifications(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)

		rows, err := conn.Query(`
			SELECT n.id, n.type, n.event_id, e.name, u.username, n.read, n.created_at,
			       m.id, m.status
			FROM notifications n
			JOIN events e ON e.id = n.event_id
			JOIN users u ON u.id = n.from_user_id
			LEFT JOIN event_members m ON m.event_id = n.event_id AND m.user_id = n.user_id
			WHERE n.user_id = $1
			ORDER BY n.created_at DESC`,
			userID,
		)
		if err != nil {
			http.Error(w, "could not load notifications", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		results := []notificationItem{}
		for rows.Next() {
			var n notificationItem
			var memberID, memberStatus sql.NullString
			if err := rows.Scan(&n.ID, &n.Type, &n.EventID, &n.EventName, &n.FromUser, &n.Read, &n.CreatedAt, &memberID, &memberStatus); err != nil {
				http.Error(w, "could not load notifications", http.StatusInternalServerError)
				return
			}
			n.MemberID = memberID.String
			n.MemberStatus = memberStatus.String
			results = append(results, n)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"results": results})
	}
}

func ReadNotification(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)
		notifID := r.PathValue("id")

		_, _ = conn.Exec(
			`UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2`,
			notifID, userID,
		)

		w.WriteHeader(http.StatusNoContent)
	}
}
