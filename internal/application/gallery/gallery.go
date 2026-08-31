// Package gallery holds the use case for the Gallery read-model.
package gallery

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	domaingallery "yadegar/internal/domain/gallery"
	"yadegar/internal/telemetry"
)

type ListGalleryUseCase struct {
	repo domaingallery.Repository
}

func NewListGalleryUseCase(repo domaingallery.Repository) *ListGalleryUseCase {
	return &ListGalleryUseCase{repo: repo}
}

func (uc *ListGalleryUseCase) Execute(ctx context.Context, userID string) ([]domaingallery.Event, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "ListGalleryUseCase.Execute")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", userID))

	events, err := uc.repo.ListForUser(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return events, nil
}
