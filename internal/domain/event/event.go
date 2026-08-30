// Package event holds the Event entity, the Member entity, and the rules
// between them. This package holds no HTTP code. This package holds no
// database code.
package event

import "errors"

// Event is a gathering that a user creates. Members join an Event.
type Event struct {
	ID        string
	Name      string
	Location  string
	CreatorID string
}

// MemberStatus tracks the state of one user inside one Event.
type MemberStatus string

const (
	MemberStatusInvited  MemberStatus = "invited"
	MemberStatusApproved MemberStatus = "approved"
	MemberStatusRejected MemberStatus = "rejected"
)

// Member links one user to one Event.
type Member struct {
	ID       string
	EventID  string
	UserID   string
	Username string
	Status   MemberStatus
	TaggedBy string
}

// Domain errors. Use cases and adapters check these errors with errors.Is.
var (
	ErrNotFound      = errors.New("event not found")
	ErrForbidden     = errors.New("not your tag")
	ErrAlreadyMember = errors.New("user already tagged in this event")
	ErrTagRejected   = errors.New("this user rejected this tag; they must be asked again in person")
)
