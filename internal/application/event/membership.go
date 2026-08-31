package event

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	domainevent "yadegar/internal/domain/event"
	"yadegar/internal/telemetry"
)

// TagMemberUseCase invites a user into an Event.
type TagMemberUseCase struct {
	repo domainevent.Repository
}

func NewTagMemberUseCase(repo domainevent.Repository) *TagMemberUseCase {
	return &TagMemberUseCase{repo: repo}
}

type TagMemberInput struct {
	EventID  string
	CallerID string
	TargetID string
}

// ApproveMemberUseCase confirms a tag. Only the tagged user can approve their own tag.
type ApproveMemberUseCase struct {
	repo domainevent.Repository
}

func NewApproveMemberUseCase(repo domainevent.Repository) *ApproveMemberUseCase {
	return &ApproveMemberUseCase{repo: repo}
}

// RejectMemberUseCase declines a tag. Only the tagged user can reject their own tag.
type RejectMemberUseCase struct {
	repo domainevent.Repository
}

func NewRejectMemberUseCase(repo domainevent.Repository) *RejectMemberUseCase {
	return &RejectMemberUseCase{repo: repo}
}

// RemoveMemberUseCase removes an approved membership. Only the member
// themselves can remove their own membership.
type RemoveMemberUseCase struct {
	repo domainevent.Repository
}

func NewRemoveMemberUseCase(repo domainevent.Repository) *RemoveMemberUseCase {
	return &RemoveMemberUseCase{repo: repo}
}

func (uc *TagMemberUseCase) Execute(ctx context.Context, in TagMemberInput) (*domainevent.Member, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "TagMemberUseCase.Execute")
	defer span.End()
	span.SetAttributes(attribute.String("event.id", in.EventID))

	approved, err := uc.repo.IsApprovedMember(ctx, in.EventID, in.CallerID)
	if err != nil || !approved {
		span.RecordError(domainevent.ErrNotFound)
		span.SetStatus(codes.Error, domainevent.ErrNotFound.Error())
		return nil, domainevent.ErrNotFound
	}

	status, found, err := uc.repo.MemberStatus(ctx, in.EventID, in.TargetID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if found {
		if status == domainevent.MemberStatusRejected {
			span.RecordError(domainevent.ErrTagRejected)
			span.SetStatus(codes.Error, domainevent.ErrTagRejected.Error())
			return nil, domainevent.ErrTagRejected
		}
		span.RecordError(domainevent.ErrAlreadyMember)
		span.SetStatus(codes.Error, domainevent.ErrAlreadyMember.Error())
		return nil, domainevent.ErrAlreadyMember
	}

	m := &domainevent.Member{
		ID:       uuid.New().String(),
		EventID:  in.EventID,
		UserID:   in.TargetID,
		Status:   domainevent.MemberStatusInvited,
		TaggedBy: in.CallerID,
	}

	// AddMember writes the membership row and stages the member-tagged
	// outbox event in one Postgres transaction — see
	// adapters/postgres/event_repo.go. No separate notifier call needed
	// here anymore; a background publisher delivers the outbox row to
	// Kafka.
	if err := uc.repo.AddMember(ctx, m); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return m, nil
}

func (uc *ApproveMemberUseCase) Execute(ctx context.Context, memberID, userID string) error {
	ctx, span := telemetry.Tracer().Start(ctx, "ApproveMemberUseCase.Execute")
	defer span.End()

	// ApproveMember updates the row and stages the member-approved
	// outbox event in the same transaction — see
	// adapters/postgres/event_repo.go.
	_, _, err := uc.repo.ApproveMember(ctx, memberID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (uc *RejectMemberUseCase) Execute(ctx context.Context, memberID, userID string) error {
	ctx, span := telemetry.Tracer().Start(ctx, "RejectMemberUseCase.Execute")
	defer span.End()

	// RejectMember updates the row and stages the member-rejected
	// outbox event in the same transaction — see
	// adapters/postgres/event_repo.go.
	_, _, err := uc.repo.RejectMember(ctx, memberID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (uc *RemoveMemberUseCase) Execute(ctx context.Context, memberID, userID string) error {
	ctx, span := telemetry.Tracer().Start(ctx, "RemoveMemberUseCase.Execute")
	defer span.End()

	if _, err := uc.repo.RemoveMember(ctx, memberID, userID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}
