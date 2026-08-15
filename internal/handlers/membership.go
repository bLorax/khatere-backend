package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"yadegar/internal/middleware"

	"github.com/google/uuid"
)

type tagRequest struct {
	UserID string `json:"user_id"`
}

func TagMember(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerID := middleware.UserID(r)
		eventID := r.PathValue("id")

		var req tagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var approved bool
		err := conn.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM event_members WHERE event_id = $1 AND user_id = $2 AND status = 'approved')`,
			eventID, callerID,
		).Scan(&approved)
		if err != nil || !approved {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}

		var existingStatus string
		err = conn.QueryRow(
			`SELECT status FROM event_members WHERE event_id = $1 AND user_id = $2`,
			eventID, req.UserID,
		).Scan(&existingStatus)

		if err == nil {
			if existingStatus == "rejected" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write([]byte(`{"error": {"code": "TAG_REJECTED", "message": "this user rejected this tag; they must be asked again in person"}}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"error": {"code": "ALREADY_MEMBER", "message": "user already tagged in this event"}}`))
			return
		} else if err != sql.ErrNoRows {
			http.Error(w, "lookup failed", http.StatusInternalServerError)
			return
		}

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "could not start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		memberID := uuid.New().String()
		_, err = tx.Exec(
			`INSERT INTO event_members (id, event_id, user_id, status, tagged_by) VALUES ($1, $2, $3, 'invited', $4)`,
			memberID, eventID, req.UserID, callerID,
		)
		if err != nil {
			http.Error(w, "could not create tag", http.StatusInternalServerError)
			return
		}

		notifID := uuid.New().String()
		_, err = tx.Exec(
			`INSERT INTO notifications (id, user_id, type, event_id, from_user_id) VALUES ($1, $2, 'tag_request', $3, $4)`,
			notifID, req.UserID, eventID, callerID,
		)
		if err != nil {
			http.Error(w, "could not create notification", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "could not save tag", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id": memberID, "event_id": eventID, "user_id": req.UserID, "status": "invited", "tagged_by": callerID,
		})
	}
}

func ApproveMember(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)
		memberID := r.PathValue("id")

		result, err := conn.Exec(
			`UPDATE event_members SET status = 'approved' WHERE id = $1 AND user_id = $2`,
			memberID, userID,
		)
		if err != nil {
			http.Error(w, "could not approve tag", http.StatusInternalServerError)
			return
		}

		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			http.Error(w, "not your tag to approve", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": memberID, "status": "approved"})
	}
}

func RejectMember(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)
		memberID := r.PathValue("id")

		tx, err := conn.Begin()
		if err != nil {
			http.Error(w, "could not start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var eventID, taggedBy string
		err = tx.QueryRow(
			`UPDATE event_members SET status = 'rejected' WHERE id = $1 AND user_id = $2
			 RETURNING event_id, tagged_by`,
			memberID, userID,
		).Scan(&eventID, &taggedBy)
		if err != nil {
			http.Error(w, "not your tag to reject", http.StatusForbidden)
			return
		}

		notifID := uuid.New().String()
		_, err = tx.Exec(
			`INSERT INTO notifications (id, user_id, type, event_id, from_user_id) VALUES ($1, $2, 'tag_rejected', $3, $4)`,
			notifID, taggedBy, eventID, userID,
		)
		if err != nil {
			http.Error(w, "could not create notification", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "could not save rejection", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": memberID, "status": "rejected"})
	}
}

func RemoveMember(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)
		memberID := r.PathValue("id")

		result, err := conn.Exec(
			`DELETE FROM event_members WHERE id = $1 AND user_id = $2 AND status = 'approved'`,
			memberID, userID,
		)
		if err != nil {
			http.Error(w, "could not remove tag", http.StatusInternalServerError)
			return
		}

		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			http.Error(w, "nothing to remove", http.StatusForbidden)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
