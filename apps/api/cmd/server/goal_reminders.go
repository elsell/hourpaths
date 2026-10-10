package main

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	activityroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/routes"
	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"time"
)

func goalReminderPreferenceRegistration(store *gormstore.Store, auth ports.Authenticator, authorizer ports.Authorizer, clock ports.Clock, limiter ports.AuditRateLimiter) func(huma.API) {
	repository := activitystore.New(store.DB)
	service := activityapp.New(activityapp.Dependencies{Auth: auth, Authorizer: authorizer, Repository: repository, ReminderPreferences: repository, Audits: store, AuditRateLimiter: limiter, Clock: clock, NewID: uuid.NewString})
	return func(api huma.API) { activityroutes.RegisterGoalReminderPreferences(api, service) }
}

func reconcileGoalReminders(ctx context.Context, store *gormstore.Store, authorizer ports.Authorizer, clock ports.Clock, limiter ports.AuditRateLimiter, probe ports.Probe) {
	worker := &activityapp.GoalReminderWorker{Repository: activitystore.New(store.DB), Authorizer: authorizer, Clock: clock, Audits: store, RateLimiter: limiter, NewID: uuid.NewString, BatchSize: 100}
	reconcileGoalNotices(ctx, worker, probe, "goal_reminder.reconciliation")
}

func reconcileGoalDeadlines(ctx context.Context, store *gormstore.Store, authorizer ports.Authorizer, clock ports.Clock, limiter ports.AuditRateLimiter, probe ports.Probe) {
	worker := &activityapp.GoalDeadlineNoticeWorker{Repository: activitystore.New(store.DB), Authorizer: authorizer, Clock: clock, Audits: store, RateLimiter: limiter, NewID: uuid.NewString, BatchSize: 100}
	reconcileGoalNotices(ctx, worker, probe, "goal_deadline_notice.reconciliation")
}

type goalNoticeWorker interface {
	RunOnce(context.Context) (int, bool, error)
}

// Drain bounded pages before waiting so later participants do not incur an
// extra polling interval for every earlier page. Cancellation bounds shutdown.
func reconcileGoalNotices(ctx context.Context, worker goalNoticeWorker, probe ports.Probe, eventName string) {
	for {
		for ctx.Err() == nil {
			_, more, err := worker.RunOnce(ctx)
			if err != nil && ctx.Err() == nil {
				probe.Observe(ctx, ports.ProbeEvent{Name: eventName, Outcome: "failed"})
			}
			if err != nil || !more {
				break
			}
		}
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
