// Package photo holds the use cases for the Photo domain. Each use case
// holds one application action. Each use case calls the domain ports.
// Each use case does not know which adapter serves the ports.
package photo

import (
	"context"
	"io"

	"github.com/google/uuid"

	domainphoto "yadegar/internal/domain/photo"
)

// UploadPhotoUseCase saves a new photo to an Event, after checking that
// the uploader is an approved member and the Event is under its photo
// limit.
type UploadPhotoUseCase struct {
	repo       domainphoto.Repository
	storage    domainphoto.Storage
	membership domainphoto.MembershipChecker
}

func NewUploadPhotoUseCase(
	repo domainphoto.Repository,
	storage domainphoto.Storage,
	membership domainphoto.MembershipChecker,
) *UploadPhotoUseCase {
	return &UploadPhotoUseCase{repo: repo, storage: storage, membership: membership}
}

type UploadPhotoInput struct {
	EventID    string
	UploaderID string
	Filename   string
	Content    io.Reader
}

func (uc *UploadPhotoUseCase) Execute(ctx context.Context, in UploadPhotoInput) (*domainphoto.Photo, error) {
	approved, err := uc.membership.IsApprovedMember(ctx, in.EventID, in.UploaderID)
	if err != nil || !approved {
		return nil, domainphoto.ErrEventNotFound
	}

	count, err := uc.repo.CountForEvent(ctx, in.EventID)
	if err != nil {
		return nil, err
	}
	if count >= domainphoto.MaxPhotosPerEvent {
		return nil, domainphoto.ErrPhotoLimitReached
	}

	p := &domainphoto.Photo{
		ID:         uuid.New().String(),
		EventID:    in.EventID,
		UploaderID: in.UploaderID,
	}

	url, err := uc.storage.Save(in.EventID, p.ID, in.Filename, in.Content)
	if err != nil {
		return nil, err
	}
	p.URL = url

	// Only the first two photos of an Event get a thumbnail right away.
	// A later photo gets one lazily, the first time ListEventPhotos runs.
	if count < domainphoto.ThumbnailedPhotoLimit {
		thumbnailURL, err := uc.storage.EnsureThumbnail(url)
		if err != nil {
			return nil, err
		}
		p.ThumbnailURL = thumbnailURL
	}

	if err := uc.repo.Create(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}
