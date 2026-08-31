// Package s3 holds an adapter that stores files in any S3-compatible
// object store: AWS S3, Minio (local dev), Cloudflare R2, Backblaze B2.
// This adapter implements the same domainphoto.Storage port that
// adapters/filesystem does. Swapping between them is a one-line change
// in main.go — nothing in domain or application changes.
//
// The bucket is assumed PRIVATE. This adapter never builds a public
// URL by concatenating strings — every client-facing URL comes from
// PublicURL, which generates a short-lived presigned GET request. This
// matters because this app gates photo access on Event membership;
// a permanently-public URL would bypass that check entirely.
package s3

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"io"

	"github.com/minio/minio-go/v7"

	"yadegar/internal/imaging"
)

// PhotoStorage implements domainphoto.Storage against an S3-compatible
// bucket. presignExpiry controls how long a PublicURL stays valid —
// 15 minutes is a reasonable default: long enough for a client to load
// an image, short enough that a leaked URL isn't useful for long.
type PhotoStorage struct {
	client        *minio.Client
	bucket        string
	presignExpiry time.Duration
}

func NewPhotoStorage(client *minio.Client, bucket string, presignExpiry time.Duration) *PhotoStorage {
	return &PhotoStorage{client: client, bucket: bucket, presignExpiry: presignExpiry}
}

func (s *PhotoStorage) Save(eventID, photoID, filename string, content io.Reader) (string, error) {
	ext := filepath.Ext(filename)
	key := eventID + "/" + photoID + ext

	// size=-1: the multipart upload's total size isn't known up front.
	// minio-go streams the upload in parts automatically in this case.
	_, err := s.client.PutObject(context.Background(), s.bucket, key, content, -1, minio.PutObjectOptions{
		ContentType: contentType(ext),
	})
	if err != nil {
		return "", fmt.Errorf("upload to object storage: %w", err)
	}

	return key, nil
}

func (s *PhotoStorage) EnsureThumbnail(key string) (string, error) {
	ext := strings.ToLower(filepath.Ext(key))
	base := strings.TrimSuffix(key, filepath.Ext(key))
	thumbnailKey := base + "_thumb.jpg"

	ctx := context.Background()

	if _, err := s.client.StatObject(ctx, s.bucket, thumbnailKey, minio.StatObjectOptions{}); err == nil {
		return thumbnailKey, nil
	} else if minio.ToErrorResponse(err).Code != "NoSuchKey" {
		return "", fmt.Errorf("check thumbnail: %w", err)
	}

	original, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("fetch original: %w", err)
	}
	defer original.Close()

	img, err := imaging.Decode(ext, original)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	thumbBytes, err := imaging.Thumbnail(img)
	if err != nil {
		return "", err
	}

	_, err = s.client.PutObject(ctx, s.bucket, thumbnailKey, bytes.NewReader(thumbBytes), int64(len(thumbBytes)), minio.PutObjectOptions{
		ContentType: "image/jpeg",
	})
	if err != nil {
		return "", fmt.Errorf("upload thumbnail: %w", err)
	}

	return thumbnailKey, nil
}

// PublicURL generates a presigned GET URL, valid for s.presignExpiry.
// Call this fresh for every response — never cache or store the result.
func (s *PhotoStorage) PublicURL(key string) (string, error) {
	reqParams := make(map[string][]string)
	presignedURL, err := s.client.PresignedGetObject(context.Background(), s.bucket, key, s.presignExpiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("presign url: %w", err)
	}
	return presignedURL.String(), nil
}

func contentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	default:
		return "image/jpeg"
	}
}
