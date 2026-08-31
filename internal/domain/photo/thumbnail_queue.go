package photo

// ThumbnailQueue is a port. Uploading a photo hands off thumbnail work
// to whatever implements this port, instead of generating the
// thumbnail inline on the request path.
type ThumbnailQueue interface {
	// Enqueue schedules thumbnail generation for the photo at url.
	// Enqueue blocks once the queue is full, rather than dropping the
	// job or spawning unbounded work.
	Enqueue(url string)
}
