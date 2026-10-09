package profileimage

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"

	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
	"github.com/gen2brain/h265/heic"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var ErrInvalid = errors.New("invalid profile picture")

const maxPixels = 64_000_000

// Transform runs inside the bounded image worker, never the API request process.
func Transform(data []byte, crop *profilepicture.Crop) (profilepicture.Prepared, error) {
	if len(data) == 0 || len(data) > profilepicture.MaxUploadBytes {
		return profilepicture.Prepared{}, ErrInvalid
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxPixels/cfg.Height {
		return profilepicture.Prepared{}, ErrInvalid
	}
	var src image.Image
	switch format {
	case "heic":
		src, err = heic.Decode(bytes.NewReader(data), heic.Options{AutoRotate: true, FrameSizeLimit: maxPixels, Threads: 1})
	case "jpeg", "png", "webp":
		src, _, err = image.Decode(bytes.NewReader(data))
	default:
		return profilepicture.Prepared{}, ErrInvalid
	}
	if err != nil {
		return profilepicture.Prepared{}, ErrInvalid
	}
	if format != "heic" {
		src = orient(src, readOrientation(data, format))
	}
	width, height := src.Bounds().Dx(), src.Bounds().Dy()
	if width < 1 || height < 1 || width > maxPixels/height {
		return profilepicture.Prepared{}, ErrInvalid
	}
	if width > profilepicture.PreviewEdge || height > profilepicture.PreviewEdge {
		if width >= height {
			height = max(1, height*profilepicture.PreviewEdge/width)
			width = profilepicture.PreviewEdge
		} else {
			width = max(1, width*profilepicture.PreviewEdge/height)
			height = profilepicture.PreviewEdge
		}
	}
	preview := image.NewRGBA(image.Rect(0, 0, width, height))
	// Opaque white prevents transparent source pixels turning black in JPEG output.
	draw.Draw(preview, preview.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.ApproxBiLinear.Scale(preview, preview.Bounds(), src, src.Bounds(), draw.Over, nil)
	output := preview
	if crop != nil {
		if !crop.Valid(width, height) {
			return profilepicture.Prepared{}, ErrInvalid
		}
		output = image.NewRGBA(image.Rect(0, 0, profilepicture.PictureEdge, profilepicture.PictureEdge))
		draw.ApproxBiLinear.Scale(output, output.Bounds(), preview, image.Rect(crop.X, crop.Y, crop.X+crop.Size, crop.Y+crop.Size), draw.Src, nil)
	}
	var encoded bytes.Buffer
	if err = jpeg.Encode(&encoded, output, &jpeg.Options{Quality: 88}); err != nil {
		return profilepicture.Prepared{}, ErrInvalid
	}
	return profilepicture.Prepared{JPEG: encoded.Bytes(), Width: output.Bounds().Dx(), Height: output.Bounds().Dy()}, nil
}

type oriented struct {
	image.Image
	orientation int
}

func orient(src image.Image, n int) image.Image {
	if n < 2 || n > 8 {
		return src
	}
	return oriented{src, n}
}
func (im oriented) Bounds() image.Rectangle {
	b := im.Image.Bounds()
	if im.orientation >= 5 {
		return image.Rect(0, 0, b.Dy(), b.Dx())
	}
	return image.Rect(0, 0, b.Dx(), b.Dy())
}
func (im oriented) At(x, y int) color.Color {
	b := im.Image.Bounds()
	w, h := b.Dx(), b.Dy()
	switch im.orientation {
	case 2:
		x = w - 1 - x
	case 3:
		x, y = w-1-x, h-1-y
	case 4:
		y = h - 1 - y
	case 5:
		x, y = y, x
	case 6:
		x, y = y, h-1-x
	case 7:
		x, y = w-1-y, h-1-x
	case 8:
		x, y = w-1-y, x
	}
	return im.Image.At(b.Min.X+x, b.Min.Y+y)
}
