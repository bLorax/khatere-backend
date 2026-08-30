// Package notification holds the Notification entity. This package holds
// no HTTP code and no database code.
package notification

// Notification tells one user about one event: a tag request, or a
// tag decision.
type Notification struct {
	ID           string
	UserID       string
	Type         string
	EventID      string
	EventName    string
	FromUserID   string
	FromUsername string
	Read         bool
	CreatedAt    string
	MemberID     string
	MemberStatus string
}

const (
	TypeTagRequest  = "tag_request"
	TypeTagApproved = "tag_approved"
	TypeTagRejected = "tag_rejected"
)
