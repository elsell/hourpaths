package ports

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
)

// ProfilePictureProcessor owns bounded, content-validated image normalization.
type ProfilePictureProcessor interface {
	Process(context.Context, []byte, *profilepicture.Crop) (profilepicture.Prepared, error)
}

// ProviderPictureRetriever fetches only the verified provider's image bytes.
type ProviderPictureRetriever interface {
	Fetch(context.Context, string) ([]byte, error)
}
