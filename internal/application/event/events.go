// Package event holds the use cases for the Event domain. Each use case
// holds one application action. Each use case calls the domain ports.
// Each use case does not know which adapter serves the ports.
package event

import (
	"context"

	"github.com/google/uuid"

	domainevent "yadegar/internal/domain/event"
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

func (uc *CreateEventUseCase) Execute(ctx context.Context, in CreateEventInput) (*domainevent.Event, error) {
	e := &domainevent.Event{
		ID:        uuid.New().String(),
		Name:      in.Name,
		Location:  in.Location,
		CreatorID: in.CreatorID,
	}

	if err := uc.repo.Create(ctx, e); err != nil {
		return nil, err
	}

	return e, nil
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

func (uc *ListEventsUseCase) Execute(ctx context.Context, in ListEventsInput) ([]domainevent.Event, error) {
	return uc.repo.ListForUser(ctx, in.UserID, in.Search)
}

// GetEventUseCase returns one Event and its members. GetEventUseCase checks
// that the caller is an approved member first.
//
// NOTE: the original handler also attached recent photos to this response.
// Photo loading belongs to a future Photo domain step. Until that step, the
// HTTP adapter loads photos on its own, as a documented temporary bridge.
type GetEventUseCase struct {
	repo domainevent.Repository
}

func NewGetEventUseCase(repo domainevent.Repository) *GetEventUseCase {
	return &GetEventUseCase{repo: repo}
}

type GetEventInput struct {
	EventID string
	UserID  string
}

type GetEventOutput struct {
	Event   domainevent.Event
	Members []domainevent.Member
}

func (uc *GetEventUseCase) Execute(ctx context.Context, in GetEventInput) (*GetEventOutput, error) {
	approved, err := uc.repo.IsApprovedMember(ctx, in.EventID, in.UserID)
	if err != nil || !approved {
		return nil, domainevent.ErrNotFound
	}

	e, err := uc.repo.Get(ctx, in.EventID)
	if err != nil {
		return nil, domainevent.ErrNotFound
	}

	members, err := uc.repo.ListMembers(ctx, in.EventID)
	if err != nil {
		return nil, err
	}

	return &GetEventOutput{Event: *e, Members: members}, nil
}
