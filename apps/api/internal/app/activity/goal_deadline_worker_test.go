package activity

import (
	"context"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

type deadlineQueue struct {
	scanned   []GoalDeadlineCandidate
	published []GoalDeadlineNoticeCommand
}

func (q *deadlineQueue) ListGoalDeadlineCandidates(_ context.Context, _ time.Time, after GoalDeadlineCandidate, _ int) (GoalDeadlinePage, error) {
	q.scanned = append(q.scanned, after)
	if after == (GoalDeadlineCandidate{}) {
		return GoalDeadlinePage{Next: GoalDeadlineCandidate{ParticipantID: "owner", PathID: "a"}}, nil
	}
	return GoalDeadlinePage{Candidates: []GoalDeadlineCandidate{{ParticipantID: "owner", PathID: "allowed"}, {ParticipantID: "owner", PathID: "denied"}}}, nil
}
func (q *deadlineQueue) PublishGoalDeadlineNotice(_ context.Context, c GoalDeadlineNoticeCommand) (bool, error) {
	q.published = append(q.published, c)
	return true, nil
}
func TestGoalDeadlineWorkerAdvancesFilteredPagesAndFailsClosed(t *testing.T) {
	q := &deadlineQueue{}
	a := &longTimerAccess{fail: true}
	events := &longTimerAudit{}
	w := GoalDeadlineNoticeWorker{Repository: q, Authorizer: a, Clock: longTimerClock{time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}, Audits: events, RateLimiter: longTimerLimiter{}, NewID: func() string { return "deadline-audit" }, BatchSize: 10}
	if n, more, err := w.RunOnce(context.Background()); err != nil || n != 0 || !more {
		t.Fatalf("filtered page: %d %v %v", n, more, err)
	}
	if _, _, err := w.RunOnce(context.Background()); err == nil || len(q.published) != 0 {
		t.Fatal("authorization dependency failed open")
	}
	a.fail = false
	if n, more, err := w.RunOnce(context.Background()); err != nil || n != 1 || more {
		t.Fatalf("due page: %d %v %v", n, more, err)
	}
	if len(q.published) != 1 || q.published[0].Candidate.PathID != "allowed" || len(events.events) != 1 || events.events[0].Outcome != audit.Denied {
		t.Fatalf("unsafe delivery %+v %+v", q.published, events.events)
	}
}
