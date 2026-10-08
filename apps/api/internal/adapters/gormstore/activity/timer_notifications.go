package activitystore

import (
	"context"

	timerstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationtimer"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (r *Repository) TimerNotificationCandidates(ctx context.Context, actor, path string) ([]string, error) {
	if r == nil || r.DB == nil {
		return nil, ports.ErrInvalidArgument
	}
	return timerstore.Candidates(ctx, r.DB, actor, path)
}
