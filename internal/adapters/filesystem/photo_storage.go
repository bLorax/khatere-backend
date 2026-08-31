// Package filesystem holds adapters that store files on local disk.
package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"yadegar/internal/imaging"
)

// PhotoStorage implements domainphoto.Storage with the local filesystem.
// baseDir is the directory files are written under, e.g. "uploads".
type PhotoStorage struct {
	baseDir string
}

func NewPhotoStorage(baseDir string) *PhotoStorage {
	return &PhotoStorage{baseDir: baseDir}
}

func (s *PhotoStorage) Save(eventID, photoID, filename string, content io.Reader) (string, error) {
	ext := filepath.Ext(filename)
	eventDir := filepath.Join(s.baseDir, eventID)
	if err := os.MkdirAll(eventDir, 0o755); err != nil {
		return "", fmt.Errorf("create storage directory: %w", err)
	}

	destPath := filepath.Join(eventDir, photoID+ext)
	dest, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}

	if _, err := io.Copy(dest, content); err != nil {
		dest.Close()
		return "", fmt.Errorf("write file: %w", err)
	}
	if err := dest.Close(); err != nil {
		return "", fmt.Errorf("close file: %w", err)
	}

	return "/" + destPath, nil
}

// EnsureThumbnail makes sure a thumbnail exists for the photo at url, and
// returns the thumbnail's own URL. EnsureThumbnail does nothing to a
// thumbnail that already exists.
//
// Example: the photo at "/uploads/event-id/photo.jpg" gets a thumbnail
// at "/uploads/event-id/photo_thumb.jpg".
func (s *PhotoStorage) EnsureThumbnail(url string) (string, error) {
	sourcePath := strings.TrimPrefix(url, "/")
	ext := strings.ToLower(filepath.Ext(sourcePath))
	base := strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath))
	thumbnailPath := base + "_thumb.jpg"

	if _, err := os.Stat(thumbnailPath); err == nil {
		return "/" + thumbnailPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check thumbnail: %w", err)
	}

	input, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}
	img, err := imaging.Decode(ext, input)
	input.Close()
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	thumbBytes, err := imaging.Thumbnail(img)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(thumbnailPath, thumbBytes, 0o644); err != nil {
		return "", fmt.Errorf("write thumbnail: %w", err)
	}

	return "/" + thumbnailPath, nil
}

// PublicURL is the identity function for local disk: the key already IS
// the servable path (served by the app's static file route).
func (s *PhotoStorage) PublicURL(key string) (string, error) {
	return key, nil
}
