package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"

	domainevent "yadegar/internal/domain/event"
)

// NewWriter builds one shared Kafka writer. Pass the same *kafka.Writer
// into every Notifier adapter that needs to publish — kafka-go's Writer
// is safe for concurrent use and routes each message by its own Topic
// field, so one writer covers every topic in this app.
func NewWriter(brokers []string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
	}
}

// EventNotifier implements domainevent.Notifier by publishing to Kafka,
// instead of writing directly to Postgres. A separate consumer process
// (see consumer.go) reads these messages and creates the actual
// notification rows.
type EventNotifier struct {
	writer *kafka.Writer
}

func NewEventNotifier(writer *kafka.Writer) *EventNotifier {
	return &EventNotifier{writer: writer}
}

func (n *EventNotifier) NotifyTagRequest(ctx context.Context, toUserID, eventID, fromUserID string) error {
	return publish(ctx, n.writer, TopicMemberTagged, eventID, MemberEvent{
		EventID: eventID, ToUserID: toUserID, FromUserID: fromUserID, OccurredAt: time.Now(),
	})
}

func (n *EventNotifier) NotifyTagApproved(ctx context.Context, toUserID, eventID, fromUserID string) error {
	return publish(ctx, n.writer, TopicMemberApproved, eventID, MemberEvent{
		EventID: eventID, ToUserID: toUserID, FromUserID: fromUserID, OccurredAt: time.Now(),
	})
}

func (n *EventNotifier) NotifyTagRejected(ctx context.Context, toUserID, eventID, fromUserID string) error {
	return publish(ctx, n.writer, TopicMemberRejected, eventID, MemberEvent{
		EventID: eventID, ToUserID: toUserID, FromUserID: fromUserID, OccurredAt: time.Now(),
	})
}

func publish(ctx context.Context, writer *kafka.Writer, topic, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
	})
}

var _ domainevent.Notifier = (*EventNotifier)(nil)
