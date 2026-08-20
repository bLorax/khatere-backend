package handlers

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

const thumbnailMaxSize = 400

func generateThumbnail(sourcePath string) error {
	ext := strings.ToLower(filepath.Ext(sourcePath))

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

	base := strings.TrimSuffix(sourcePath, ext)
	thumbnailPath := base + "_thumb.jpg"

	output, err := os.Create(thumbnailPath)
	if err != nil {
		return fmt.Errorf("create thumbnail: %w", err)
	}
	defer output.Close()

	if err := jpeg.Encode(output, thumbnail, &jpeg.Options{
		Quality: 80,
	}); err != nil {
		return fmt.Errorf("encode thumbnail: %w", err)
	}

	return nil
}

func resizeImage(src image.Image, maxSize int) image.Image {
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
			srcX := bounds.Min.X +
				int(float64(x)*float64(width)/float64(newWidth))

			srcY := bounds.Min.Y +
				int(float64(y)*float64(height)/float64(newHeight))

			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}
