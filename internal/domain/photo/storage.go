package photo

import "io"

// Storage is a port. The adapters layer implements this port by writing
// files to a real disk (or, later, to something like S3). The
// application layer only calls this port; it never opens a file itself.
//
// Save and EnsureThumbnail deal in storage keys — a stable reference to
// where a file lives, safe to persist in the database. A key is NOT
// necessarily something a client can fetch directly: the filesystem
// adapter's keys happen to double as servable paths, but an S3 adapter's
// presigned URLs expire and must never be stored. PublicURL is the only
// method that resolves a key into something a client can actually use,
// and it must be called fresh, right before a response goes out — never
// cached, never persisted.
type Storage interface {
	// Save writes content to storage under eventID, using photoID as the
	// file's name. filename is the name the file had before upload; only
	// its extension is used. Save returns the file's storage key.
	Save(eventID, photoID, filename string, content io.Reader) (key string, err error)

	// EnsureThumbnail makes sure a thumbnail exists for the photo at
	// key, building one if needed, and returns the thumbnail's own key.
	// EnsureThumbnail does nothing to a thumbnail that already exists.
	EnsureThumbnail(key string) (thumbnailKey string, err error)

	// PublicURL resolves a key into a URL a client can fetch right now.
	// For local disk, this is the identity function. For S3, this
	// generates a short-lived presigned URL. Call this once, right
	// before building a response — never store the result.
	PublicURL(key string) (url string, err error)
}
