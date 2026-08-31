// Package imaging holds pure image decode/resize/encode logic, with no
// filesystem code and no network code. Storage adapters (filesystem, s3,
// ...) call this package; this package never reads or writes anything
// itself.
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
)

const ThumbnailMaxSize = 400

// Decode reads an image from r. ext is the source file's extension
// (".jpg", ".jpeg", or ".png") and picks the decoder.
func Decode(ext string, r io.Reader) (image.Image, error) {
	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Decode(r)
	case ".png":
		return png.Decode(r)
	default:
		return nil, fmt.Errorf("unsupported image type: %s", ext)
	}
}

// Thumbnail resizes src down to at most ThumbnailMaxSize on its longest
// side, and returns it encoded as JPEG bytes.
func Thumbnail(src image.Image) ([]byte, error) {
	resized := resize(src, ThumbnailMaxSize)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 80}); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}
	return buf.Bytes(), nil
}

// resize scales an image down while preserving its aspect ratio. The
// longest side will be at most maxSize. Small images are not enlarged.
func resize(src image.Image, maxSize int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

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
