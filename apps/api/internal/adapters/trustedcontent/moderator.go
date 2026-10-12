// Package trustedcontent implements the owner-approved trusted-content phase.
// It performs no classification, storage, logging or external communication.
package trustedcontent

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type Moderator struct{}

var _ ports.TextModerator = Moderator{}

func (Moderator) Check(ctx context.Context, _ ports.TextSurface, _ string) error {
	return ctx.Err()
}
