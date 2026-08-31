package photo

import (
	"context"
	"io"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"yadegar/internal/telemetry"

	"github.com/google/uuid"

	domainphoto "yadegar/internal/domain/photo"
)

// UploadPhotoUseCase saves a new photo to an Event, after checking that
// the uploader is an approved member and the Event is under its photo
// limit.
type UploadPhotoUseCase struct {
	repo           domainphoto.Repository
	storage        domainphoto.Storage
	membership     domainphoto.MembershipChecker
	thumbnailQueue domainphoto.ThumbnailQueue
	notifier       domainphoto.Notifier
}

func NewUploadPhotoUseCase(
	repo domainphoto.Repository,
	storage domainphoto.Storage,
	membership domainphoto.MembershipChecker,
	thumbnailQueue domainphoto.ThumbnailQueue,
	notifier domainphoto.Notifier,
) *UploadPhotoUseCase {
	return &UploadPhotoUseCase{repo: repo, storage: storage, membership: membership, thumbnailQueue: thumbnailQueue, notifier: notifier}
}

type UploadPhotoInput struct {
	EventID    string
	UploaderID string
	Filename   string
	Content    io.Reader
}

func (uc *UploadPhotoUseCase) Execute(ctx context.Context, in UploadPhotoInput) (*domainphoto.Photo, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "UploadPhotoUseCase.Execute")
	defer span.End()
	span.SetAttributes(attribute.String("event.id", in.EventID))

	approved, err := uc.membership.IsApprovedMember(ctx, in.EventID, in.UploaderID)
	if err != nil || !approved {
		span.RecordError(domainphoto.ErrEventNotFound)
		span.SetStatus(codes.Error, domainphoto.ErrEventNotFound.Error())
		return nil, domainphoto.ErrEventNotFound
	}

	count, err := uc.repo.CountForEvent(ctx, in.EventID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if count >= domainphoto.MaxPhotosPerEvent {
		span.RecordError(domainphoto.ErrPhotoLimitReached)
		span.SetStatus(codes.Error, domainphoto.ErrPhotoLimitReached.Error())
		return nil, domainphoto.ErrPhotoLimitReached
	}

	p := &domainphoto.Photo{
		ID:         uuid.New().String(),
		EventID:    in.EventID,
		UploaderID: in.UploaderID,
	}

	url, err := uc.storage.Save(in.EventID, p.ID, in.Filename, in.Content)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	// p.URL holds the storage KEY at this point — this is what gets
	// persisted below. It gets resolved into an actual client-facing
	// URL only afterward, and only in the in-memory copy we return.
	p.URL = url

	// Only the first two photos of an Event get a thumbnail right away.
	// A later photo gets one lazily, the first time ListEventPhotos runs.
	// The thumbnail itself is now built off the request path, by the
	// worker pool — the response no longer waits for it.
	if count < domainphoto.ThumbnailedPhotoLimit {
		uc.thumbnailQueue.Enqueue(url)
	}

	if err := uc.repo.Create(ctx, p); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if err := uc.notifier.NotifyPhotoUploaded(ctx, in.EventID, p.ID, in.UploaderID); err != nil {
		// A failed publish here doesn't fail the upload — the photo is
		// already saved. Log it and move on; this is exactly the gap
		// D3's transactional outbox step (next) is meant to close.
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return p, nil

	// Resolve the key into an actual URL only now, after the key itself
	// is safely in the database. For S3, this is a fresh presigned URL —
	// it must never be what got persisted above.
	publicURL, err := uc.storage.PublicURL(p.URL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	p.URL = publicURL

	return p, nil
}
