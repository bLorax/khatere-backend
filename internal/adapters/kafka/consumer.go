package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	domainevent "yadegar/internal/domain/event"

	"github.com/segmentio/kafka-go"

	domainnotif "yadegar/internal/domain/notification"
)

// maxHandleRetries and retryBaseDelay govern how hard consumeLoop tries
// a message before giving up on it. Backoff is attempt-number * base,
// so 200ms, 400ms — short, since these are local DB writes, not calls
// to a flaky external service.
const (
	maxHandleRetries = 3
	retryBaseDelay   = 200 * time.Millisecond
)

// NotificationConsumer reads from all four topics and turns each
// message into a notification row via domainnotif.Repository. This is
// the pattern-match for the instructions' request to reuse the
// thumbnailqueue-style bounded worker approach: a fixed number of
// goroutines per topic, not one goroutine per message.
type NotificationConsumer struct {
	repo      domainnotif.Repository
	eventRepo domainevent.Repository
	readers   []*kafka.Reader
	cancel    context.CancelFunc
}

func NewNotificationConsumer(brokers []string, groupID string, repo domainnotif.Repository, eventRepo domainevent.Repository) *NotificationConsumer {
	topics := []string{TopicMemberTagged, TopicMemberApproved, TopicMemberRejected, TopicPhotoUploaded}

	readers := make([]*kafka.Reader, 0, len(topics))
	for _, topic := range topics {
		readers = append(readers, kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			GroupID: groupID,
			Topic:   topic,
		}))
	}

	return &NotificationConsumer{repo: repo, eventRepo: eventRepo, readers: readers}
}

// Start launches one goroutine per topic. Each goroutine reads messages
// one at a time and blocks until the notification row is written before
// reading the next — this bounds concurrency to exactly len(readers)
// in-flight writes, the same bounded-work idea as the thumbnail pool,
// just expressed as "one worker per partition" instead of a shared
// channel, which fits Kafka's own consumption model better.
func (c *NotificationConsumer) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	for _, reader := range c.readers {
		go c.consumeLoop(ctx, reader)
	}
}

// consumeLoop uses FetchMessage, not the ReadMessage convenience method.
// ReadMessage commits the offset internally BEFORE returning the message
// to the caller — meaning a message would count as "done" even if
// handle() then failed, with no way to retry it. FetchMessage instead
// leaves committing entirely up to us: the offset only advances after
// handleWithRetry confirms the message was actually processed (or
// deliberately given up on — see that function's comment).
func (c *NotificationConsumer) consumeLoop(ctx context.Context, reader *kafka.Reader) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			log.Printf("kafka fetch error on topic %s: %v", reader.Config().Topic, err)
			continue
		}

		c.handleWithRetry(ctx, reader, msg)
	}
}

// handleWithRetry calls handle() up to maxHandleRetries times, with a
// short backoff between attempts, and only commits the message once
// handle() succeeds — or once every retry has been exhausted.
//
// A crash between FetchMessage and a successful commit means the next
// FetchMessage call on restart reads the SAME message again, so this
// gives real at-least-once processing instead of ReadMessage's
// commit-then-process ordering.
//
// If every retry fails, this still commits and moves on, rather than
// blocking every later message on the same topic behind one message
// that can never succeed (a "poison" message — e.g. malformed JSON
// that will never unmarshal no matter how many times it's retried).
// That trade-off is deliberate: one silently-dropped message, loudly
// logged, beats an entire topic stalling forever. The "giving up" log
// line below is the signal worth alerting on.
func (c *NotificationConsumer) handleWithRetry(ctx context.Context, reader *kafka.Reader, msg kafka.Message) {
	var err error
	for attempt := 1; attempt <= maxHandleRetries; attempt++ {
		if err = c.handle(ctx, reader.Config().Topic, msg); err == nil {
			break
		}
		log.Printf("failed to handle message on topic %s (attempt %d/%d): %v",
			reader.Config().Topic, attempt, maxHandleRetries, err)

		if attempt < maxHandleRetries {
			select {
			case <-ctx.Done():
				return // shutting down mid-retry — leave uncommitted, don't sleep
			case <-time.After(retryBaseDelay * time.Duration(attempt)):
			}
		}
	}

	if err != nil {
		log.Printf("giving up on message (topic %s, partition %d, offset %d) after %d attempts — committing anyway to avoid blocking the partition: %v",
			reader.Config().Topic, msg.Partition, msg.Offset, maxHandleRetries, err)
	}

	if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
		log.Printf("failed to commit message on topic %s: %v", reader.Config().Topic, commitErr)
	}
}

func (c *NotificationConsumer) handle(ctx context.Context, topic string, msg kafka.Message) error {
	switch topic {
	case TopicMemberTagged, TopicMemberApproved, TopicMemberRejected:
		var ev MemberEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			return err
		}
		notifType := map[string]string{
			TopicMemberTagged:   domainnotif.TypeTagRequest,
			TopicMemberApproved: domainnotif.TypeTagApproved,
			TopicMemberRejected: domainnotif.TypeTagRejected,
		}[topic]
		return c.repo.Create(ctx, &domainnotif.Notification{
			UserID: ev.ToUserID, Type: notifType, EventID: ev.EventID, FromUserID: ev.FromUserID,
		})

	case TopicPhotoUploaded:
		var ev PhotoUploadedEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			return err
		}

		members, err := c.eventRepo.ListMembers(ctx, ev.EventID)
		if err != nil {
			return err
		}

		for _, m := range members {
			if m.Status != domainevent.MemberStatusApproved || m.UserID == ev.UploaderID {
				continue
			}
			if err := c.repo.Create(ctx, &domainnotif.Notification{
				UserID: m.UserID, Type: domainnotif.TypePhotoUploaded, EventID: ev.EventID, FromUserID: ev.UploaderID,
			}); err != nil {
				// Log and continue — one failed recipient shouldn't block
				// notifying the rest of the event's members. Unlike the
				// top-level handle() error, a per-recipient failure here
				// is NOT retried by handleWithRetry, since this loop
				// always returns nil below. See PLACEMENT.md for why
				// that asymmetry was left as-is for this step.
				log.Printf("failed to create photo_uploaded notification for user %s: %v", m.UserID, err)
			}
		}
		return nil
	}
	return nil
}

// Shutdown stops all consumer goroutines and closes every reader.
func (c *NotificationConsumer) Shutdown() {
	if c.cancel != nil {
		c.cancel()
	}
	for _, r := range c.readers {
		_ = r.Close()
	}
}
