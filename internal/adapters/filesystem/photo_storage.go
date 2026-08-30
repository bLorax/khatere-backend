// Package filesystem holds adapters that store files on local disk.
package filesystem

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const thumbnailMaxSize = 400

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

	if err := generateThumbnail(sourcePath, thumbnailPath, ext); err != nil {
		return "", err
	}

	return "/" + thumbnailPath, nil
}

// generateThumbnail decodes the image at sourcePath, resizes it, and
// writes it to thumbnailPath as JPEG. This is the same logic the old
// internal/handlers/thumbnails.go used; it now lives here because
// building a thumbnail is a filesystem-and-image-codec concern, which
// belongs in an adapter, not in application or domain code.
func generateThumbnail(sourcePath, thumbnailPath, ext string) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer input.Close()

	var img image.Image
	switch ext {
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(input)
	case ".png":
		img, err = png.Decode(input)
	default:
		return fmt.Errorf("unsupported image type: %s", ext)
	}
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	thumbnail := resizeImage(img, thumbnailMaxSize)

	output, err := os.Create(thumbnailPath)
	if err != nil {
		return fmt.Errorf("create thumbnail: %w", err)
	}
	defer output.Close()

	if err := jpeg.Encode(output, thumbnail, &jpeg.Options{Quality: 80}); err != nil {
		return fmt.Errorf("encode thumbnail: %w", err)
	}

	return nil
}

// resizeImage scales an image down while preserving its aspect ratio.
// The longest side will be at most maxSize.
func resizeImage(src image.Image, maxSize int) image.Image {
	bounds := src.Bounds()

	width := bounds.Dx()
	height := bounds.Dy()

	// Don't enlarge small images.
	if width <= maxSize && height <= maxSize {
		return src
	}

	scale := float64(maxSize) / float64(width)
	if height > width {
		scale = float64(maxSize) / float64(height)
	}

	newWidth := int(float64(width) * scale)
	newHeight := int(float64(height) * scale)

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := bounds.Min.X + int(float64(x)*float64(width)/float64(newWidth))
			srcY := bounds.Min.Y + int(float64(y)*float64(height)/float64(newHeight))
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}
