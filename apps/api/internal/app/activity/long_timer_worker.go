package activity

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
)

// LongTimerNoticeWorker owns a bounded cursor. It is run serially per process;
// database receipts coordinate independent workers and process restarts.
type LongTimerNoticeWorker struct {
	Repository  LongTimerNoticeRepository
	Authorizer  ports.Authorizer
	Clock       ports.Clock
	Audits      ports.Audits
	RateLimiter ports.AuditRateLimiter
	NewID       func() string
	BatchSize   int
	afterID     string
}

func (w *LongTimerNoticeWorker) RunOnce(ctx context.Context) (int, error) {
	if w == nil || w.Repository == nil || w.Authorizer == nil || w.Clock == nil || w.Audits == nil || w.RateLimiter == nil || w.NewID == nil || w.BatchSize < 1 || w.BatchSize > 100 {
		return 0, ports.ErrInvalidArgument
	}
	at := durableInstant(w.Clock.Now())
	if at.IsZero() {
		return 0, ports.ErrInvalidArgument
	}
	candidates, err := w.Repository.ListLongTimerCandidates(ctx, at, w.afterID, w.BatchSize)
	if err != nil {
		return 0, err
	}
	if len(candidates) > w.BatchSize {
		return 0, ports.ErrInvalidArgument
	}
	previous := w.afterID
	for _, c := range candidates {
		if c.TimerID <= previous || strings.TrimSpace(c.PathID) == "" || strings.TrimSpace(c.ParticipantID) == "" {
			return 0, ports.ErrInvalidArgument
		}
		previous = c.TimerID
	}
	published := 0
	for _, c := range candidates {
		if !w.RateLimiter.Allow(c.ParticipantID, at) {
			w.afterID = c.TimerID
			continue
		}
		allowed, err := w.Authorizer.Check(ctx, "path", c.PathID, "track", c.ParticipantID)
		if err != nil {
			return published, err
		}
		event := audit.Event{ID: w.NewID(), OwnerUserID: c.ParticipantID, ActorUserID: c.ParticipantID, Action: audit.ResourceCreated, TargetType: "long_timer_notice", TargetID: c.TimerID, Outcome: audit.Succeeded, CorrelationID: w.NewID(), OccurredAt: at}
		if !event.Valid() {
			return published, ports.ErrInvalidArgument
		}
		if !allowed {
			event.Action = audit.ResourceAccessDenied
			event.Outcome = audit.Denied
			if err := w.Audits.AppendAuditEvent(ctx, event); err != nil {
				return published, err
			}
		} else {
			sent, err := w.Repository.PublishLongTimerNotice(ctx, LongTimerNoticeCommand{Candidate: c, At: at, Audit: event})
			if err != nil {
				return published, err
			}
			if sent {
				published++
			}
		}
		w.afterID = c.TimerID
	}
	if len(candidates) < w.BatchSize {
		w.afterID = ""
	}
	return published, nil
}
