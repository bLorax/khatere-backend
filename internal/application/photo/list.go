package photo

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	domainphoto "yadegar/internal/domain/photo"
	"yadegar/internal/telemetry"
)

// ListEventPhotosUseCase returns an Event's earliest photos, each with a
// thumbnail. This use case backs the "photos" section of GetEvent. It
// replaces the temporary loadPhotosWithThumbnails bridge that used to
// live on EventHandler.
type ListEventPhotosUseCase struct {
	repo    domainphoto.Repository
	storage domainphoto.Storage
}

func NewListEventPhotosUseCase(repo domainphoto.Repository, storage domainphoto.Storage) *ListEventPhotosUseCase {
	return &ListEventPhotosUseCase{repo: repo, storage: storage}
}

func (uc *ListEventPhotosUseCase) Execute(ctx context.Context, eventID string) ([]domainphoto.Photo, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "ListEventPhotosUseCase.Execute")
	defer span.End()
	span.SetAttributes(attribute.String("event.id", eventID))

	photos, err := uc.repo.ListForEvent(ctx, eventID, domainphoto.ThumbnailedPhotoLimit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	for i := range photos {
		// photos[i].URL currently holds the storage KEY, as read from
		// the database. Resolve the thumbnail first (EnsureThumbnail
		// also takes a key), then resolve both keys into real URLs.
		thumbnailKey, err := uc.storage.EnsureThumbnail(photos[i].URL)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		thumbnailURL, err := uc.storage.PublicURL(thumbnailKey)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		photos[i].ThumbnailURL = thumbnailURL

		photoURL, err := uc.storage.PublicURL(photos[i].URL)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		photos[i].URL = photoURL
	}

	return photos, nil
}
