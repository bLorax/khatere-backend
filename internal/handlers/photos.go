package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"yadegar/internal/middleware"

	"github.com/google/uuid"
)

func UploadPhoto(conn *sql.DB) http.HandlerFunc {
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

		var count int
		err = conn.QueryRow(`SELECT count(*) FROM photos WHERE event_id = $1`, eventID).Scan(&count)
		if err != nil {
			http.Error(w, "could not check photo count", http.StatusInternalServerError)
			return
		}
		if count >= 10 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"error": {"code": "EVENT_PHOTO_LIMIT_REACHED", "message": "this event already has the maximum of 10 photos/videos"}}`))
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		photoID := uuid.New().String()
		ext := filepath.Ext(header.Filename)
		eventDir := filepath.Join("uploads", eventID)
		if err := os.MkdirAll(eventDir, 0o755); err != nil {
			http.Error(w, "could not create storage directory", http.StatusInternalServerError)
			return
		}
		destPath := filepath.Join(eventDir, photoID+ext)

		dest, err := os.Create(destPath)
		if err != nil {
			http.Error(w, "could not save file", http.StatusInternalServerError)
			return
		}
		defer dest.Close()

		if _, err := io.Copy(dest, file); err != nil {
			http.Error(w, "could not save file", http.StatusInternalServerError)
			return
		}

		url := "/uploads/" + eventID + "/" + photoID + ext
		_, err = conn.Exec(
			`INSERT INTO photos (id, event_id, uploader_id, storage_key) VALUES ($1, $2, $3, $4)`,
			photoID, eventID, userID, url,
		)
		if err != nil {
			http.Error(w, "could not record photo", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id": photoID, "event_id": eventID, "uploader_id": userID, "url": url,
		})
	}
}
