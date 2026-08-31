// Package event holds the use cases for the Event domain. Each use case
// holds one application action. Each use case calls the domain ports.
// Each use case does not know which adapter serves the ports.
package event

import (
	"context"
	"sync"
	appphoto "yadegar/internal/application/photo"
	domainevent "yadegar/internal/domain/event"
	domainphoto "yadegar/internal/domain/photo"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"yadegar/internal/telemetry"

	"github.com/google/uuid"
)

// CreateEventUseCase creates a new Event and adds the creator as a member.
type CreateEventUseCase struct {
	repo domainevent.Repository
}

func NewCreateEventUseCase(repo domainevent.Repository) *CreateEventUseCase {
	return &CreateEventUseCase{repo: repo}
}

type CreateEventInput struct {
	Name      string
	Location  string
	CreatorID string
}

// ListEventsUseCase lists every Event a user belongs to.
type ListEventsUseCase struct {
	repo domainevent.Repository
}

func NewListEventsUseCase(repo domainevent.Repository) *ListEventsUseCase {
	return &ListEventsUseCase{repo: repo}
}

type ListEventsInput struct {
	UserID string
	Search string
}

// GetEventUseCase returns one Event and its members. GetEventUseCase checks
// that the caller is an approved member first.
//
// NOTE: the original handler also attached recent photos to this response.
// Photo loading belongs to a future Photo domain step. Until that step, the
// HTTP adapter loads photos on its own, as a documented temporary bridge.
type GetEventUseCase struct {
	repo       domainevent.Repository
	listPhotos *appphoto.ListEventPhotosUseCase
}

func NewGetEventUseCase(repo domainevent.Repository, listPhotos *appphoto.ListEventPhotosUseCase) *GetEventUseCase {
	return &GetEventUseCase{repo: repo, listPhotos: listPhotos}
}

type GetEventInput struct {
	EventID string
	UserID  string
}

type GetEventOutput struct {
	Event   domainevent.Event
	Members []domainevent.Member
	Photos  []domainphoto.Photo
}

func (uc *GetEventUseCase) Execute(ctx context.Context, in GetEventInput) (*GetEventOutput, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "GetEventUseCase.Execute")
	defer span.End()
	span.SetAttributes(attribute.String("event.id", in.EventID))

	approved, err := uc.repo.IsApprovedMember(ctx, in.EventID, in.UserID)
	if err != nil || !approved {
		span.RecordError(domainevent.ErrNotFound)
		span.SetStatus(codes.Error, domainevent.ErrNotFound.Error())
		return nil, domainevent.ErrNotFound
	}

	e, err := uc.repo.Get(ctx, in.EventID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, domainevent.ErrNotFound
	}

	var (
		members               []domainevent.Member
		photos                []domainphoto.Photo
		membersErr, photosErr error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		spanCtx, memberSpan := telemetry.Tracer().Start(ctx, "GetEventUseCase.ListMembers")
		defer memberSpan.End()
		members, membersErr = uc.repo.ListMembers(spanCtx, in.EventID)
		if membersErr != nil {
			memberSpan.RecordError(membersErr)
			memberSpan.SetStatus(codes.Error, membersErr.Error())
		}
	}()

	go func() {
		defer wg.Done()
		spanCtx, photoSpan := telemetry.Tracer().Start(ctx, "GetEventUseCase.ListPhotos")
		defer photoSpan.End()
		photos, photosErr = uc.listPhotos.Execute(spanCtx, in.EventID)
		if photosErr != nil {
			photoSpan.RecordError(photosErr)
			photoSpan.SetStatus(codes.Error, photosErr.Error())
		}
	}()

	wg.Wait()

	if membersErr != nil {
		span.RecordError(membersErr)
		span.SetStatus(codes.Error, membersErr.Error())
		return nil, membersErr
	}
	if photosErr != nil {
		span.RecordError(photosErr)
		span.SetStatus(codes.Error, photosErr.Error())
		return nil, photosErr
	}

	return &GetEventOutput{Event: *e, Members: members, Photos: photos}, nil
}

func (uc *CreateEventUseCase) Execute(ctx context.Context, in CreateEventInput) (*domainevent.Event, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "CreateEventUseCase.Execute")
	defer span.End()

	e := &domainevent.Event{
		ID:        uuid.New().String(),
		Name:      in.Name,
		Location:  in.Location,
		CreatorID: in.CreatorID,
	}

	if err := uc.repo.Create(ctx, e); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return e, nil
}

func (uc *ListEventsUseCase) Execute(ctx context.Context, in ListEventsInput) ([]domainevent.Event, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "ListEventsUseCase.Execute")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", in.UserID))

	results, err := uc.repo.ListForUser(ctx, in.UserID, in.Search)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return results, nil
}
