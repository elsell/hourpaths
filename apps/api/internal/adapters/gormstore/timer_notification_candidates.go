package gormstore

import (
	"context"
	timerstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationtimer"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (r *TimerSubscriptionRepository) TimerNotificationCandidates(ctx context.Context, actor, path string) ([]string, error) {
	if r == nil {
		return nil, ports.ErrInvalidArgument
	}
	return timerstore.Candidates(ctx, r.db, actor, path)
}
