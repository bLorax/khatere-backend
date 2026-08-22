package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"yadegar/internal/middleware"

	"github.com/google/uuid"
)

type createEventRequest struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

type eventResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Location  string `json:"location"`
	CreatorID string `json:"creator_id"`
}

type eventListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Location  string `json:"location"`
	CreatorID string `json:"creator_id"`
}

type eventDetailMember struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Status   string `json:"status"`
	TaggedBy string `json:"tagged_by"`
}

type eventDetailResponse struct {
	Event   eventListItem       `json:"event"`
	Members []eventDetailMember `json:"members"`
}

func CreateEvent(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)

		var req createEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "could not start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		eventID := uuid.New().String()
		_, err = tx.Exec(
			`INSERT INTO events (id, name, location, creator_id) VALUES ($1, $2, $3, $4)`,
			eventID, req.Name, req.Location, userID,
		)
		if err != nil {
			http.Error(w, "could not create event", http.StatusInternalServerError)
			return
		}

		memberID := uuid.New().String()
		_, err = tx.Exec(
			`INSERT INTO event_members (id, event_id, user_id, status, tagged_by) VALUES ($1, $2, $3, 'approved', $4)`,
			memberID, eventID, userID, userID,
		)
		if err != nil {
			http.Error(w, "could not add creator as member", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "could not save event", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(eventResponse{ID: eventID, Name: req.Name, Location: req.Location, CreatorID: userID})
	}
}

func ListEvents(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)
		search := r.URL.Query().Get("search")

		rows, err := conn.Query(`
			SELECT e.id, e.name, e.location, e.creator_id
			FROM events e
			JOIN event_members m ON m.event_id = e.id
			WHERE m.user_id = $1 AND m.status = 'approved'
			  AND ($2 = '' OR e.name ILIKE '%' || $2 || '%')
			ORDER BY e.created_at DESC`,
			userID, search,
		)
		if err != nil {
			http.Error(w, "could not list events", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		results := []eventListItem{}
		for rows.Next() {
			var e eventListItem
			if err := rows.Scan(&e.ID, &e.Name, &e.Location, &e.CreatorID); err != nil {
				http.Error(w, "could not list events", http.StatusInternalServerError)
				return
			}
			results = append(results, e)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"results": results})
	}
}

func GetEvent(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)
		eventID := r.PathValue("id")

		var approved bool
		err := conn.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM event_members WHERE event_id = $1 AND user_id = $2 AND status = 'approved')`,
			eventID, userID,
		).Scan(&approved)
		if err != nil || !approved {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}

		var event eventListItem
		err = conn.QueryRow(
			`SELECT id, name, location, creator_id FROM events WHERE id = $1`,
			eventID,
		).Scan(&event.ID, &event.Name, &event.Location, &event.CreatorID)
		if err != nil {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}

		memberRows, err := conn.Query(`
			SELECT m.id, m.user_id, u.username, m.status, m.tagged_by
			FROM event_members m
			JOIN users u ON u.id = m.user_id
			WHERE m.event_id = $1`,
			eventID,
		)
		if err != nil {
			http.Error(w, "could not load members", http.StatusInternalServerError)
			return
		}
		defer memberRows.Close()

		members := []eventDetailMember{}
		for memberRows.Next() {
			var m eventDetailMember
			if err := memberRows.Scan(&m.ID, &m.UserID, &m.Username, &m.Status, &m.TaggedBy); err != nil {
				http.Error(w, "could not load members", http.StatusInternalServerError)
				return
			}
			members = append(members, m)
		}

		photoRows, err := conn.Query(
			`SELECT id, event_id, uploader_id, storage_key
			 FROM photos
			 WHERE event_id = $1
			 ORDER BY created_at ASC
			 LIMIT 2`,
			eventID,
		)

		if err != nil {
			http.Error(w, "could not load photos", http.StatusInternalServerError)
			return
		}
		defer photoRows.Close()

		type photoItem struct {
			ID           string `json:"id"`
			EventID      string `json:"event_id"`
			UploaderID   string `json:"uploader_id"`
			URL          string `json:"url"`
			ThumbnailURL string `json:"thumbnail_url"`
		}

		photos := []photoItem{}

		for photoRows.Next() {
			var p photoItem

			if err := photoRows.Scan(
				&p.ID,
				&p.EventID,
				&p.UploaderID,
				&p.URL,
			); err != nil {
				http.Error(w, "could not load photos", http.StatusInternalServerError)
				return
			}

			// storage_key is something like:
			// /uploads/event-id/photo-id.jpg
			//
			// Convert it to a filesystem path:
			// uploads/event-id/photo-id.jpg
			sourcePath := strings.TrimPrefix(p.URL, "/")

			ext := filepath.Ext(sourcePath)
			base := strings.TrimSuffix(sourcePath, ext)

			thumbnailPath := base + "_thumb.jpg"

			// Generate the thumbnail for existing photos if it
			// doesn't already exist.
			if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
				if err := generateThumbnail(sourcePath); err != nil {
					http.Error(
						w,
						"could not generate photo thumbnail",
						http.StatusInternalServerError,
					)
					return
				}
			} else if err != nil {
				http.Error(
					w,
					"could not check photo thumbnail",
					http.StatusInternalServerError,
				)
				return
			}

			p.ThumbnailURL = "/" + thumbnailPath

			photos = append(photos, p)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"event":   event,
			"members": members,
			"photos":  photos,
		})
	}
}
