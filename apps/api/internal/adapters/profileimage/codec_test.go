package profileimage

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
)

func TestPicturePreviewCropAndInvalidReplacement(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 120; x++ {
			c := color.NRGBA{R: 240, A: 255}
			if x >= 60 {
				c = color.NRGBA{B: 240, A: 255}
			}
			src.SetNRGBA(x, y, c)
		}
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, src); err != nil {
		t.Fatal(err)
	}
	preview, err := Transform(raw.Bytes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Width != 120 || preview.Height != 80 {
		t.Fatalf("preview dimensions %+v", preview)
	}
	result, err := Transform(raw.Bytes(), &profilepicture.Crop{X: 60, Y: 0, Size: 60})
	if err != nil {
		t.Fatal(err)
	}
	cropped, err := jpeg.Decode(bytes.NewReader(result.JPEG))
	if err != nil {
		t.Fatal(err)
	}
	if cropped.Bounds().Dx() != 512 || cropped.Bounds().Dy() != 512 {
		t.Fatal("not a square profile image")
	}
	r, _, b, _ := cropped.At(256, 256).RGBA()
	if b <= r {
		t.Fatal("crop did not select the requested right-hand square")
	}
	for _, crop := range []profilepicture.Crop{{X: -1, Size: 60}, {X: 90, Size: 60}, {Y: 70, Size: 20}, {Size: 0}, {Size: 1 << 30}} {
		if _, err := Transform(raw.Bytes(), &crop); err == nil {
			t.Fatalf("accepted invalid crop %+v", crop)
		}
	}
	for _, invalid := range [][]byte{[]byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), raw.Bytes()[:20], make([]byte, profilepicture.MaxUploadBytes+1)} {
		if _, err := Transform(invalid, nil); err == nil {
			t.Fatal("accepted invalid image")
		}
	}
}

func TestJPEGOrientationAndMetadataRemoval(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	var raw bytes.Buffer
	jpeg.Encode(&raw, src, nil)
	// Minimal TIFF orientation 6 in an EXIF APP1 segment; rotate 90 degrees.
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	body := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	body = append(body, raw.Bytes()[2:]...)
	result, err := Transform(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Width != 20 || result.Height != 40 {
		t.Fatalf("orientation lost: %dx%d", result.Width, result.Height)
	}
	if bytes.Contains(result.JPEG, []byte("Exif")) {
		t.Fatal("source metadata retained")
	}
}

func TestHEICAndWebPActualContent(t *testing.T) {
	for _, name := range []string{"basic.heic", "colors.webp"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Transform(data, nil)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := jpeg.Decode(bytes.NewReader(result.JPEG))
			if err != nil {
				t.Fatal(err)
			}
			if result.Width < 1 || result.Height < 1 || decoded.Bounds().Dx() != result.Width {
				t.Fatal("invalid normalized image")
			}
		})
	}
}
