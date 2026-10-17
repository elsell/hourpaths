package app

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
	"time"
)

func (a App) importProviderPicture(ctx context.Context, token, source string) {
	if a.ProviderPictures == nil || a.PictureProcessor == nil || a.Pictures == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	image, err := a.ProviderPictures.Fetch(ctx, source)
	if err != nil {
		return
	}
	authorization := "Bearer " + token
	preview, err := a.PreviewProfilePicture(ctx, authorization, image)
	if err != nil {
		return
	}
	size := min(preview.Width, preview.Height)
	if size < 1 {
		return
	}
	// The initial revision makes a concurrent removal/replacement authoritative.
	_, _ = a.UpdateProfilePicture(ctx, authorization, newID(), PictureUpdate{
		Image: preview.JPEG, ExpectedRevision: 1,
		Crop: &profilepicture.Crop{X: (preview.Width - size) / 2, Y: (preview.Height - size) / 2, Size: size},
	})
}
