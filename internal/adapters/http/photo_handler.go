// Package httpapi (this file) holds the Photo domain's HTTP adapter.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	appphoto "yadegar/internal/application/photo"
	domainphoto "yadegar/internal/domain/photo"
	"yadegar/internal/middleware"
)

// PhotoHandler wires HTTP routes to Photo use cases.
type PhotoHandler struct {
	upload *appphoto.UploadPhotoUseCase
}

func NewPhotoHandler(upload *appphoto.UploadPhotoUseCase) *PhotoHandler {
	return &PhotoHandler{upload: upload}
}

func (h *PhotoHandler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	eventID := r.PathValue("id")

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	p, err := h.upload.Execute(r.Context(), appphoto.UploadPhotoInput{
		EventID:    eventID,
		UploaderID: userID,
		Filename:   header.Filename,
		Content:    file,
	})
	if err != nil {
		switch {
		case errors.Is(err, domainphoto.ErrEventNotFound):
			http.Error(w, "event not found", http.StatusNotFound)
		case errors.Is(err, domainphoto.ErrPhotoLimitReached):
			writeJSONError(w, http.StatusUnprocessableEntity, "EVENT_PHOTO_LIMIT_REACHED", err.Error())
		default:
			http.Error(w, "could not save photo", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id": p.ID, "event_id": p.EventID, "uploader_id": p.UploaderID, "url": p.URL,
	})
}
