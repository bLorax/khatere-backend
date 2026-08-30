package photo

import "context"

// MembershipChecker is a port. Uploading a photo requires approved
// membership in an Event, but the Photo domain does not need the Event
// domain's full Repository port to check that — it only needs this one
// question answered. Keeping the port narrow means any adapter that can
// answer this one question satisfies it, including the Event domain's
// own Repository, with no adapter code written twice.
type MembershipChecker interface {
	IsApprovedMember(ctx context.Context, eventID, userID string) (bool, error)
}
