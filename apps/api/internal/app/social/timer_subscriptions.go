package social

import (
	"context"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

// TimerSubscriptionScope identifies the relationship that grants a subscription,
// not an authorization grant. Delivery must separately check current visibility.
type TimerSubscriptionScope string

const (
	TimerSubscriptionPerson          TimerSubscriptionScope = "person"
	TimerSubscriptionPath            TimerSubscriptionScope = "path"
	UpdateTimerSubscriptionOperation                        = "notification.timer_subscription.update"
)

func (scope TimerSubscriptionScope) Valid() bool {
	return scope == TimerSubscriptionPerson || scope == TimerSubscriptionPath
}

type TimerSubscription struct {
	Enabled  bool
	Revision int64
}

type TimerSubscriptionCommand struct {
	ActorUserID, SubjectID string
	Scope                  TimerSubscriptionScope
	Enabled                bool
	ExpectedRevision       int64
	OccurredAt             time.Time
	Idempotency            ports.Idempotency
	Audit                  audit.Event
}

type TimerSubscriptionResult struct {
	Preference TimerSubscription
	Replayed   bool
}

type TimerSubscriptionRepository interface {
	GetTimerSubscription(context.Context, string, TimerSubscriptionScope, string) (TimerSubscription, error)
	UpdateTimerSubscription(context.Context, TimerSubscriptionCommand) (TimerSubscriptionResult, error)
}
