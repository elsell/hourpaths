package main

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"time"
)

func reconcileLongTimers(ctx context.Context, store *gormstore.Store, authorizer ports.Authorizer, clock ports.Clock, limiter ports.AuditRateLimiter, probe ports.Probe) {
	worker := activityapp.LongTimerNoticeWorker{Repository: activitystore.New(store.DB), Authorizer: authorizer, Clock: clock, Audits: store, RateLimiter: limiter, NewID: uuid.NewString, BatchSize: 100}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil {
			probe.Observe(ctx, ports.ProbeEvent{Name: "long_timer_notice.reconciliation", Outcome: "failed"})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
