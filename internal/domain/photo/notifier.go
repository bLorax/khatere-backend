package photo

import "context"

// Notifier is a port. The Photo domain calls this port to announce that
// a photo was uploaded. An adapter implements this port by publishing
// to Kafka. Photo itself does not decide who gets notified — that fan-out
// decision belongs to whatever consumes the event, since it needs Event
// membership data that Photo has no reason to depend on.
type Notifier interface {
	NotifyPhotoUploaded(ctx context.Context, eventID, photoID, uploaderID string) error
}
