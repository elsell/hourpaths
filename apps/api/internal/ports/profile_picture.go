package ports

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
)

// ProfilePictureProcessor owns bounded, content-validated image normalization.
type ProfilePictureProcessor interface {
	Process(context.Context, []byte, *profilepicture.Crop) (profilepicture.Prepared, error)
}
