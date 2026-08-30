package photo

import (
	"context"

	domainphoto "yadegar/internal/domain/photo"
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
	photos, err := uc.repo.ListForEvent(ctx, eventID, domainphoto.ThumbnailedPhotoLimit)
	if err != nil {
		return nil, err
	}

	for i := range photos {
		thumbnailURL, err := uc.storage.EnsureThumbnail(photos[i].URL)
		if err != nil {
			return nil, err
		}
		photos[i].ThumbnailURL = thumbnailURL
	}

	return photos, nil
}
