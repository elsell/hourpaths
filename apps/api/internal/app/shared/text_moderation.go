package shared

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

// CheckText fails closed on missing dependencies and does not expose processor
// diagnostics (which could contain submitted text) to clients or logs.
func CheckText(ctx context.Context, moderator ports.TextModerator, surface ports.TextSurface, text string) error {
	if moderator == nil {
		return ports.ErrUnavailable
	}
	if err := moderator.Check(ctx, surface, text); err != nil {
		return ports.ErrUnavailable
	}
	return nil
}
