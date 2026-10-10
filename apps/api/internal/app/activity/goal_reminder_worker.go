package activity

import (
	"context"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

// Pagination is by participant, never by Path: splitting a participant across
// pages would silently turn one five-minute bundle into separate notifications.
type GoalReminderCandidate struct {
	ParticipantID string
	PathIDs       []string
}
type GoalReminderCandidatePage struct {
	Candidates        []GoalReminderCandidate
	NextParticipantID string
}
type GoalReminderDeliveryCommand struct {
	ParticipantID     string
	AuthorizedPathIDs []string
	At                time.Time
	Audit             audit.Event
}
type GoalReminderRepository interface {
	ListGoalReminderCandidates(context.Context, time.Time, string, int) (GoalReminderCandidatePage, error)
	// Recheck eligibility and group the authorized Paths within the transaction.
	// Receipts, notifications, audit and push admission commit together.
	PublishGoalReminders(context.Context, GoalReminderDeliveryCommand) (int, error)
}
type GoalReminderWorker struct {
	Repository         GoalReminderRepository
	Authorizer         ports.Authorizer
	Clock              ports.Clock
	Audits             ports.Audits
	RateLimiter        ports.AuditRateLimiter
	NewID              func() string
	BatchSize          int
	afterParticipantID string
}

func (w *GoalReminderWorker) RunOnce(ctx context.Context) (int, bool, error) {
	if w == nil || w.Repository == nil || w.Authorizer == nil || w.Clock == nil || w.Audits == nil || w.RateLimiter == nil || w.NewID == nil || w.BatchSize < 1 || w.BatchSize > 100 {
		return 0, false, ports.ErrInvalidArgument
	}
	at := durableInstant(w.Clock.Now())
	if at.IsZero() {
		return 0, false, ports.ErrInvalidArgument
	}
	page, err := w.Repository.ListGoalReminderCandidates(ctx, at, w.afterParticipantID, w.BatchSize)
	if err != nil {
		return 0, false, err
	}
	if len(page.Candidates) > w.BatchSize {
		return 0, false, ports.ErrInvalidArgument
	}
	previous := w.afterParticipantID
	for _, c := range page.Candidates {
		if !validPathID(c.ParticipantID) || c.ParticipantID <= previous || len(c.PathIDs) == 0 {
			return 0, false, ports.ErrInvalidArgument
		}
		previous = c.ParticipantID
		lastPath := ""
		for _, id := range c.PathIDs {
			if !validPathID(id) || id <= lastPath {
				return 0, false, ports.ErrInvalidArgument
			}
			lastPath = id
		}
	}
	more := page.NextParticipantID != ""
	if more && (!validPathID(page.NextParticipantID) || page.NextParticipantID <= w.afterParticipantID || page.NextParticipantID < previous) {
		return 0, false, ports.ErrInvalidArgument
	}
	published := 0
	for _, c := range page.Candidates {
		paths := make([]string, 0, len(c.PathIDs))
		admitted := true
		for _, id := range c.PathIDs {
			if !w.RateLimiter.Allow(c.ParticipantID, at) {
				admitted = false
				break
			}
			allowed, err := w.Authorizer.Check(ctx, "path", id, "track", c.ParticipantID)
			if err != nil {
				return published, false, err
			}
			if allowed {
				paths = append(paths, id)
				continue
			}
			event := audit.Event{ID: w.NewID(), OwnerUserID: c.ParticipantID, ActorUserID: c.ParticipantID, Action: audit.ResourceAccessDenied, TargetType: "path", TargetID: id, Outcome: audit.Denied, CorrelationID: w.NewID(), OccurredAt: at}
			if !event.Valid() {
				return published, false, ports.ErrInvalidArgument
			}
			if err := w.Audits.AppendAuditEvent(ctx, event); err != nil {
				return published, false, err
			}
		}
		if !admitted || len(paths) == 0 || !w.RateLimiter.Allow(c.ParticipantID, at) {
			continue
		}
		event := audit.Event{ID: w.NewID(), OwnerUserID: c.ParticipantID, ActorUserID: c.ParticipantID, Action: audit.ResourceCreated, TargetType: "goal_reminder_delivery", TargetID: c.ParticipantID, Outcome: audit.Succeeded, CorrelationID: w.NewID(), OccurredAt: at}
		if !event.Valid() {
			return published, false, ports.ErrInvalidArgument
		}
		n, err := w.Repository.PublishGoalReminders(ctx, GoalReminderDeliveryCommand{ParticipantID: c.ParticipantID, AuthorizedPathIDs: paths, At: at, Audit: event})
		if err != nil {
			return published, false, err
		}
		if n < 0 || n > len(paths) {
			return published, false, ports.ErrInvalidArgument
		}
		published += n
	}
	w.afterParticipantID = page.NextParticipantID
	return published, more, nil
}

// Participant lookup keeps all of an owner's Paths together for stop-triggered
// bundling without disturbing the periodic scan's pagination.
type GoalReminderParticipantRepository interface {
	GoalReminderRepository
	GoalReminderCandidateForParticipant(context.Context, time.Time, string) (GoalReminderCandidate, error)
}
type participantReminderScan struct {
	GoalReminderParticipantRepository
	owner string
}

func (r participantReminderScan) ListGoalReminderCandidates(ctx context.Context, at time.Time, _ string, _ int) (GoalReminderCandidatePage, error) {
	candidate, err := r.GoalReminderCandidateForParticipant(ctx, at, r.owner)
	if err != nil {
		return GoalReminderCandidatePage{}, err
	}
	if candidate.ParticipantID != r.owner {
		return GoalReminderCandidatePage{}, ports.ErrInvalidArgument
	}
	if len(candidate.PathIDs) == 0 {
		return GoalReminderCandidatePage{}, nil
	}
	return GoalReminderCandidatePage{Candidates: []GoalReminderCandidate{candidate}}, nil
}
func (w *GoalReminderWorker) RunParticipant(ctx context.Context, owner string) (int, error) {
	if w == nil || !validPathID(owner) {
		return 0, ports.ErrInvalidArgument
	}
	repository, ok := w.Repository.(GoalReminderParticipantRepository)
	if !ok {
		return 0, ports.ErrInvalidArgument
	}
	scoped := *w
	scoped.Repository = participantReminderScan{GoalReminderParticipantRepository: repository, owner: owner}
	scoped.afterParticipantID = ""
	scoped.BatchSize = 1
	n, _, err := scoped.RunOnce(ctx)
	return n, err
}

func (s *Service) reconcileStoppedGoalReminders(ctx context.Context, owner string) {
	repository, ok := s.Repository.(GoalReminderParticipantRepository)
	if !ok {
		return
	}
	worker := GoalReminderWorker{Repository: repository, Authorizer: s.Authorizer, Clock: s.Clock, Audits: s.Audits, RateLimiter: s.AuditRateLimiter, NewID: s.NewID, BatchSize: 1}
	// The timer stop is already committed. A reminder failure must not turn a
	// successful stop into an error; the periodic scan retries undelivered goals.
	_, _ = worker.RunParticipant(ctx, owner)
}
