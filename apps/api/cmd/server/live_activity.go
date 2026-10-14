package main

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	pushprovider "github.com/elsell/hour-paths/apps/api/internal/adapters/push"
	liveactivityapp "github.com/elsell/hour-paths/apps/api/internal/app/liveactivity"
	"github.com/elsell/hour-paths/apps/api/internal/config"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"net/http"
	"time"
)

func newLiveActivityRuntime(cfg config.Config, repository *gormstore.PushRepository, clock ports.Clock) (ports.LiveActivityRegistrationRepository, *liveactivityapp.Worker, error) {
	if !cfg.LiveActivity.Enabled {
		return nil, nil, nil
	}
	provider, err := pushprovider.NewLiveActivityAPNs(&http.Client{Timeout: 10 * time.Second}, clock, cfg.LiveActivity.KeyID, cfg.LiveActivity.TeamID, []byte(cfg.LiveActivity.PrivateKey), cfg.LiveActivity.Sandbox)
	if err != nil {
		return nil, nil, err
	}
	worker := &liveactivityapp.Worker{Repository: repository, Provider: provider, Clock: clock, WorkerID: uuid.NewString(), NewID: uuid.NewString}
	return repository, worker, nil
}
func reconcileLiveActivities(ctx context.Context, worker *liveactivityapp.Worker, probe ports.Probe) {
	if worker == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil {
			probe.Observe(ctx, ports.ProbeEvent{Name: "live_activity_refresh.reconciliation", Outcome: "failed"})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
