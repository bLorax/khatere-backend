package event

import "context"

// Repository is a port. The adapters layer implements this port with
// real SQL. The application layer only calls this port.
type Repository interface {
	// Create saves a new Event. Create also adds the creator as an
	// approved Member. Both writes happen together, or neither happens.
	Create(ctx context.Context, e *Event) error

	// ListForUser returns every Event where userID is an approved Member.
	// search filters by Event name. Pass an empty string for no filter.
	ListForUser(ctx context.Context, userID, search string) ([]Event, error)

	// Get returns one Event by ID.
	Get(ctx context.Context, eventID string) (*Event, error)

	// IsApprovedMember reports whether userID is an approved Member of eventID.
	IsApprovedMember(ctx context.Context, eventID, userID string) (bool, error)

	// ListMembers returns every Member of one Event, joined with the
	// member's username.
	ListMembers(ctx context.Context, eventID string) ([]Member, error)

	// MemberStatus returns the current status of userID inside eventID.
	// found is false when userID is not a Member yet.
	MemberStatus(ctx context.Context, eventID, userID string) (status MemberStatus, found bool, err error)

	// AddMember adds a new Member with status "invited".
	AddMember(ctx context.Context, m *Member) error

	// ApproveMember sets a Member's status to "approved". ApproveMember
	// only succeeds when userID owns the Member row (the tagged user
	// approves their own tag). ApproveMember returns the eventID and the
	// user who created the tag, for notification purposes.
	ApproveMember(ctx context.Context, memberID, userID string) (eventID, taggedBy string, err error)

	// RejectMember sets a Member's status to "rejected". Same ownership
	// rule as ApproveMember.
	RejectMember(ctx context.Context, memberID, userID string) (eventID, taggedBy string, err error)

	// RemoveMember deletes an approved Member row. Only the member
	// themselves can remove their own membership.
	RemoveMember(ctx context.Context, memberID, userID string) error
}
