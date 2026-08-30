// Package photo holds the Photo entity and the rules around it. This
// package holds no HTTP code. This package holds no database code. This
// package holds no filesystem code.
package photo

import "errors"

// Photo is one uploaded image or video, attached to one Event.
type Photo struct {
	ID           string
	EventID      string
	UploaderID   string
	URL          string
	ThumbnailURL string
}

// MaxPhotosPerEvent is the most photos one Event may hold.
const MaxPhotosPerEvent = 10

// ThumbnailedPhotoLimit is how many of an Event's earliest photos carry a
// thumbnail. UploadPhoto builds a thumbnail right away for a photo at this
// position or earlier. ListEventPhotos builds a thumbnail lazily, the
// first time someone asks, for the same first photos of an Event.
const ThumbnailedPhotoLimit = 2

// Domain errors. Use cases and adapters check these errors with errors.Is.
var (
	ErrEventNotFound     = errors.New("event not found")
	ErrPhotoLimitReached = errors.New("this event already has the maximum of 10 photos/videos")
)
