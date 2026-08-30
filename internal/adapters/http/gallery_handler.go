// Package httpapi (this file) holds the Gallery read-model's HTTP adapter.
package httpapi

import (
	"encoding/json"
	"net/http"

	appgallery "yadegar/internal/application/gallery"
	"yadegar/internal/middleware"
)

type galleryEvent struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PhotoCount int    `json:"photo_count"`
	ApprovedAt string `json:"approved_at"`
}

// GalleryHandler wires HTTP routes to the Gallery use case.
type GalleryHandler struct {
	list *appgallery.ListGalleryUseCase
}

func NewGalleryHandler(list *appgallery.ListGalleryUseCase) *GalleryHandler {
	return &GalleryHandler{list: list}
}

func (h *GalleryHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	events, err := h.list.Execute(r.Context(), userID)
	if err != nil {
		http.Error(w, "could not load gallery", http.StatusInternalServerError)
		return
	}

	results := make([]galleryEvent, 0, len(events))
	for _, e := range events {
		results = append(results, galleryEvent{ID: e.ID, Name: e.Name, PhotoCount: e.PhotoCount, ApprovedAt: e.ApprovedAt})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"events": results})
}
