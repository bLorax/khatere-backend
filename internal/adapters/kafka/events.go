// Package kafka holds the producer and consumer adapters that connect
// domain events to a real Kafka cluster. The message shapes here are
// an infrastructure concern, not a domain one — domainevent.Notifier
// itself doesn't change; only which adapter implements it does.
package kafka

import "time"

// MemberEvent is the payload for member-tagged, member-approved, and
// member-rejected. All three share the same shape.
type MemberEvent struct {
	EventID    string    `json:"event_id"`
	ToUserID   string    `json:"to_user_id"`
	FromUserID string    `json:"from_user_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// PhotoUploadedEvent is the payload for photo-uploaded.
type PhotoUploadedEvent struct {
	EventID    string    `json:"event_id"`
	PhotoID    string    `json:"photo_id"`
	UploaderID string    `json:"uploader_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

const (
	TopicMemberTagged   = "member-tagged"
	TopicMemberApproved = "member-approved"
	TopicMemberRejected = "member-rejected"
	TopicPhotoUploaded  = "photo-uploaded"
)
