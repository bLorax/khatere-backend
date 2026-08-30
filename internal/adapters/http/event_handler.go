// Package httpapi (this file) holds the Event domain's HTTP adapter.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	appevent "yadegar/internal/application/event"
	domainevent "yadegar/internal/domain/event"
	"yadegar/internal/handlers"
	"yadegar/internal/middleware"

	"database/sql"
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

type eventListItem = eventResponse

type eventDetailMember struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Status   string `json:"status"`
	TaggedBy string `json:"tagged_by"`
}

// EventHandler wires HTTP routes to Event use cases.
//
// photoDB is a TEMPORARY dependency. The GetEvent route still attaches
// recent photos and thumbnails to its response. Photo loading belongs to
// a future Photo domain step. Until that step, this handler queries
// photos directly, the same way the old handler did. Remove photoDB once
// the Photo domain exists.
type EventHandler struct {
	create  *appevent.CreateEventUseCase
	list    *appevent.ListEventsUseCase
	get     *appevent.GetEventUseCase
	tag     *appevent.TagMemberUseCase
	approve *appevent.ApproveMemberUseCase
	reject  *appevent.RejectMemberUseCase
	remove  *appevent.RemoveMemberUseCase
	photoDB *sql.DB
}

func NewEventHandler(
	create *appevent.CreateEventUseCase,
	list *appevent.ListEventsUseCase,
	get *appevent.GetEventUseCase,
	tag *appevent.TagMemberUseCase,
	approve *appevent.ApproveMemberUseCase,
	reject *appevent.RejectMemberUseCase,
	remove *appevent.RemoveMemberUseCase,
	photoDB *sql.DB,
) *EventHandler {
	return &EventHandler{
		create: create, list: list, get: get,
		tag: tag, approve: approve, reject: reject, remove: remove,
		photoDB: photoDB,
	}
}

func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	e, err := h.create.Execute(r.Context(), appevent.CreateEventInput{
		Name: req.Name, Location: req.Location, CreatorID: userID,
	})
	if err != nil {
		http.Error(w, "could not create event", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(eventResponse{ID: e.ID, Name: e.Name, Location: e.Location, CreatorID: e.CreatorID})
}

func (h *EventHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	search := r.URL.Query().Get("search")

	events, err := h.list.Execute(r.Context(), appevent.ListEventsInput{UserID: userID, Search: search})
	if err != nil {
		http.Error(w, "could not list events", http.StatusInternalServerError)
		return
	}

	results := make([]eventListItem, 0, len(events))
	for _, e := range events {
		results = append(results, eventListItem{ID: e.ID, Name: e.Name, Location: e.Location, CreatorID: e.CreatorID})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"results": results})
}

func (h *EventHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	eventID := r.PathValue("id")

	out, err := h.get.Execute(r.Context(), appevent.GetEventInput{EventID: eventID, UserID: userID})
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	members := make([]eventDetailMember, 0, len(out.Members))
	for _, m := range out.Members {
		members = append(members, eventDetailMember{
			ID: m.ID, UserID: m.UserID, Username: m.Username,
			Status: string(m.Status), TaggedBy: m.TaggedBy,
		})
	}

	photos, err := h.loadPhotosWithThumbnails(eventID)
	if err != nil {
		http.Error(w, "could not load photos", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"event":   eventResponse{ID: out.Event.ID, Name: out.Event.Name, Location: out.Event.Location, CreatorID: out.Event.CreatorID},
		"members": members,
		"photos":  photos,
	})
}

type photoItem struct {
	ID           string `json:"id"`
	EventID      string `json:"event_id"`
	UploaderID   string `json:"uploader_id"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// loadPhotosWithThumbnails is a TEMPORARY bridge. It carries the same
// behavior as the old handler. A future Photo domain step replaces this
// method with a proper application/photo use case.
func (h *EventHandler) loadPhotosWithThumbnails(eventID string) ([]photoItem, error) {
	rows, err := h.photoDB.Query(
		`SELECT id, event_id, uploader_id, storage_key
		 FROM photos
		 WHERE event_id = $1
		 ORDER BY created_at ASC
		 LIMIT 2`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := []photoItem{}
	for rows.Next() {
		var p photoItem
		if err := rows.Scan(&p.ID, &p.EventID, &p.UploaderID, &p.URL); err != nil {
			return nil, err
		}

		sourcePath := strings.TrimPrefix(p.URL, "/")
		ext := filepath.Ext(sourcePath)
		base := strings.TrimSuffix(sourcePath, ext)
		thumbnailPath := base + "_thumb.jpg"

		if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
			if err := handlers.GenerateThumbnail(sourcePath); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}

		p.ThumbnailURL = "/" + thumbnailPath
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// --- Membership routes ---

type tagRequest struct {
	UserID string `json:"user_id"`
}

func (h *EventHandler) TagMember(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.UserID(r)
	eventID := r.PathValue("id")

	var req tagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	m, err := h.tag.Execute(r.Context(), appevent.TagMemberInput{
		EventID: eventID, CallerID: callerID, TargetID: req.UserID,
	})
	if err != nil {
		switch {
		case errors.Is(err, domainevent.ErrTagRejected):
			writeJSONError(w, http.StatusUnprocessableEntity, "TAG_REJECTED", err.Error())
		case errors.Is(err, domainevent.ErrAlreadyMember):
			writeJSONError(w, http.StatusUnprocessableEntity, "ALREADY_MEMBER", err.Error())
		case errors.Is(err, domainevent.ErrNotFound):
			http.Error(w, "event not found", http.StatusNotFound)
		default:
			http.Error(w, "could not create tag", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id": m.ID, "event_id": m.EventID, "user_id": m.UserID, "status": string(m.Status), "tagged_by": m.TaggedBy,
	})
}

func (h *EventHandler) ApproveMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	memberID := r.PathValue("id")

	if err := h.approve.Execute(r.Context(), memberID, userID); err != nil {
		http.Error(w, "not your tag to approve", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"id": memberID, "status": "approved"})
}

func (h *EventHandler) RejectMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	memberID := r.PathValue("id")

	if err := h.reject.Execute(r.Context(), memberID, userID); err != nil {
		http.Error(w, "not your tag to reject", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"id": memberID, "status": "rejected"})
}

func (h *EventHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	memberID := r.PathValue("id")

	if err := h.remove.Execute(r.Context(), memberID, userID); err != nil {
		http.Error(w, "nothing to remove", http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
