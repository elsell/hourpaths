package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	pushprovider "github.com/elsell/hour-paths/apps/api/internal/adapters/push"
	pushapp "github.com/elsell/hour-paths/apps/api/internal/app/push"
	"github.com/elsell/hour-paths/apps/api/internal/config"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func newPushRuntime(
	db *gorm.DB,
	cfg config.Config,
	clock ports.Clock,
) (*gormstore.PushRepository, pushapp.PushDeliveryWorker, error) {
	repository, err := gormstore.NewPushRepository(db, []byte(cfg.PushTokenKey))
	if err != nil {
		return nil, pushapp.PushDeliveryWorker{}, err
	}
	provider, err := pushprovider.NewExpo(&http.Client{Timeout: 10 * time.Second}, cfg.PushProviderEndpoint)
	if err != nil {
		return nil, pushapp.PushDeliveryWorker{}, err
	}
	return repository, pushapp.PushDeliveryWorker{
		Repository: repository, Provider: provider, Notifications: pathstore.New(db),
		Clock: clock, WorkerID: uuid.NewString(),
		Lease: 30 * time.Second, ReceiptDelay: 15 * time.Minute,
		BaseRetryDelay: time.Minute, MaxRetryDelay: time.Hour,
		MaxAttempts: 8, BatchSize: 25,
	}, nil
}

func reconcilePush(ctx context.Context, worker pushapp.PushDeliveryWorker) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, err := worker.RunOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("push delivery reconciliation failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
