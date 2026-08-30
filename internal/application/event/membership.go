package event

import (
	"context"

	"github.com/google/uuid"

	domainevent "yadegar/internal/domain/event"
)

// TagMemberUseCase invites a user into an Event.
type TagMemberUseCase struct {
	repo     domainevent.Repository
	notifier domainevent.Notifier
}

func NewTagMemberUseCase(repo domainevent.Repository, notifier domainevent.Notifier) *TagMemberUseCase {
	return &TagMemberUseCase{repo: repo, notifier: notifier}
}

type TagMemberInput struct {
	EventID  string
	CallerID string
	TargetID string
}

func (uc *TagMemberUseCase) Execute(ctx context.Context, in TagMemberInput) (*domainevent.Member, error) {
	approved, err := uc.repo.IsApprovedMember(ctx, in.EventID, in.CallerID)
	if err != nil || !approved {
		return nil, domainevent.ErrNotFound
	}

	status, found, err := uc.repo.MemberStatus(ctx, in.EventID, in.TargetID)
	if err != nil {
		return nil, err
	}
	if found {
		if status == domainevent.MemberStatusRejected {
			return nil, domainevent.ErrTagRejected
		}
		return nil, domainevent.ErrAlreadyMember
	}

	m := &domainevent.Member{
		ID:       uuid.New().String(),
		EventID:  in.EventID,
		UserID:   in.TargetID,
		Status:   domainevent.MemberStatusInvited,
		TaggedBy: in.CallerID,
	}

	if err := uc.repo.AddMember(ctx, m); err != nil {
		return nil, err
	}

	if err := uc.notifier.NotifyTagRequest(ctx, in.TargetID, in.EventID, in.CallerID); err != nil {
		return nil, err
	}

	return m, nil
}

// ApproveMemberUseCase confirms a tag. Only the tagged user can approve their own tag.
type ApproveMemberUseCase struct {
	repo     domainevent.Repository
	notifier domainevent.Notifier
}

func NewApproveMemberUseCase(repo domainevent.Repository, notifier domainevent.Notifier) *ApproveMemberUseCase {
	return &ApproveMemberUseCase{repo: repo, notifier: notifier}
}

func (uc *ApproveMemberUseCase) Execute(ctx context.Context, memberID, userID string) error {
	eventID, taggedBy, err := uc.repo.ApproveMember(ctx, memberID, userID)
	if err != nil {
		return domainevent.ErrForbidden
	}
	return uc.notifier.NotifyTagApproved(ctx, taggedBy, eventID, userID)
}

// RejectMemberUseCase declines a tag. Only the tagged user can reject their own tag.
type RejectMemberUseCase struct {
	repo     domainevent.Repository
	notifier domainevent.Notifier
}

func NewRejectMemberUseCase(repo domainevent.Repository, notifier domainevent.Notifier) *RejectMemberUseCase {
	return &RejectMemberUseCase{repo: repo, notifier: notifier}
}

func (uc *RejectMemberUseCase) Execute(ctx context.Context, memberID, userID string) error {
	eventID, taggedBy, err := uc.repo.RejectMember(ctx, memberID, userID)
	if err != nil {
		return domainevent.ErrForbidden
	}
	return uc.notifier.NotifyTagRejected(ctx, taggedBy, eventID, userID)
}

// RemoveMemberUseCase removes an approved membership. Only the member
// themselves can remove their own membership.
type RemoveMemberUseCase struct {
	repo domainevent.Repository
}

func NewRemoveMemberUseCase(repo domainevent.Repository) *RemoveMemberUseCase {
	return &RemoveMemberUseCase{repo: repo}
}

func (uc *RemoveMemberUseCase) Execute(ctx context.Context, memberID, userID string) error {
	if err := uc.repo.RemoveMember(ctx, memberID, userID); err != nil {
		return domainevent.ErrForbidden
	}
	return nil
}
