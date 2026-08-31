package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"

	domainphoto "yadegar/internal/domain/photo"
)

// PhotoNotifier implements domainphoto.Notifier by publishing to Kafka.
// It shares the same underlying writer as EventNotifier — see NewWriter.
type PhotoNotifier struct {
	writer *kafka.Writer
}

func NewPhotoNotifier(writer *kafka.Writer) *PhotoNotifier {
	return &PhotoNotifier{writer: writer}
}

func (n *PhotoNotifier) NotifyPhotoUploaded(ctx context.Context, eventID, photoID, uploaderID string) error {
	return publish(ctx, n.writer, TopicPhotoUploaded, eventID, PhotoUploadedEvent{
		EventID: eventID, PhotoID: photoID, UploaderID: uploaderID, OccurredAt: time.Now(),
	})
}

var _ domainphoto.Notifier = (*PhotoNotifier)(nil)
