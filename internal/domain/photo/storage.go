package photo

import "io"

// Storage is a port. The adapters layer implements this port by writing
// files to a real disk (or, later, to something like S3). The
// application layer only calls this port; it never opens a file itself.
type Storage interface {
	// Save writes content to storage under eventID, using photoID as the
	// file's name. filename is the name the file had before upload; only
	// its extension is used. Save returns the file's public URL.
	Save(eventID, photoID, filename string, content io.Reader) (url string, err error)

	// EnsureThumbnail makes sure a thumbnail exists for the photo at url,
	// building one if needed, and returns the thumbnail's own URL.
	// EnsureThumbnail does nothing to a thumbnail that already exists.
	EnsureThumbnail(url string) (thumbnailURL string, err error)
}
