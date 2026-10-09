package activity

import (
	"context"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type GoalDeadlineNoticeWorker struct {
	Repository  GoalDeadlineNoticeRepository
	Authorizer  ports.Authorizer
	Clock       ports.Clock
	Audits      ports.Audits
	RateLimiter ports.AuditRateLimiter
	NewID       func() string
	BatchSize   int
	after       GoalDeadlineCandidate
}

// RunOnce returns whether another bounded page remains in the current scan.
// The runtime can drain pages before waiting, without delaying each account by
// one polling interval per earlier page.
func (w *GoalDeadlineNoticeWorker) RunOnce(ctx context.Context) (int, bool, error) {
	if w == nil || w.Repository == nil || w.Authorizer == nil || w.Clock == nil || w.Audits == nil || w.RateLimiter == nil || w.NewID == nil || w.BatchSize < 1 || w.BatchSize > 100 {
		return 0, false, ports.ErrInvalidArgument
	}
	at := durableInstant(w.Clock.Now())
	if at.IsZero() {
		return 0, false, ports.ErrInvalidArgument
	}
	page, err := w.Repository.ListGoalDeadlineCandidates(ctx, at, w.after, w.BatchSize)
	if err != nil {
		return 0, false, err
	}
	if len(page.Candidates) > w.BatchSize {
		return 0, false, ports.ErrInvalidArgument
	}
	previous := w.after
	for _, c := range page.Candidates {
		if !validPathID(c.ParticipantID) || !validPathID(c.PathID) || !c.After(previous) {
			return 0, false, ports.ErrInvalidArgument
		}
		previous = c
	}
	more := page.Next != (GoalDeadlineCandidate{})
	if more && (!validPathID(page.Next.ParticipantID) || !validPathID(page.Next.PathID) || !page.Next.After(w.after) || previous.After(page.Next)) {
		return 0, false, ports.ErrInvalidArgument
	}
	published := 0
	for _, c := range page.Candidates {
		if !w.RateLimiter.Allow(c.ParticipantID, at) {
			continue
		}
		allowed, err := w.Authorizer.Check(ctx, "path", c.PathID, "track", c.ParticipantID)
		if err != nil {
			return published, false, err
		}
		event := audit.Event{ID: w.NewID(), OwnerUserID: c.ParticipantID, ActorUserID: c.ParticipantID, Action: audit.ResourceCreated, TargetType: "goal_deadline_notice", TargetID: c.PathID, Outcome: audit.Succeeded, CorrelationID: w.NewID(), OccurredAt: at}
		if !event.Valid() {
			return published, false, ports.ErrInvalidArgument
		}
		if !allowed {
			event.Action = audit.ResourceAccessDenied
			event.Outcome = audit.Denied
			if err := w.Audits.AppendAuditEvent(ctx, event); err != nil {
				return published, false, err
			}
			continue
		}
		sent, err := w.Repository.PublishGoalDeadlineNotice(ctx, GoalDeadlineNoticeCommand{Candidate: c, At: at, Audit: event})
		if err != nil {
			return published, false, err
		}
		if sent {
			published++
		}
	}
	w.after = page.Next
	return published, more, nil
}
