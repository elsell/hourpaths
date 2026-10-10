package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

// A participant-scoped queue keeps candidates intact across scan pages.
type reminderQueue struct{ published []GoalReminderDeliveryCommand }

func (q *reminderQueue) ListGoalReminderCandidates(_ context.Context, _ time.Time, after string, _ int) (GoalReminderCandidatePage, error) {
	if after == "" {
		return GoalReminderCandidatePage{NextParticipantID: "earlier-owner"}, nil
	}
	return GoalReminderCandidatePage{Candidates: []GoalReminderCandidate{{ParticipantID: "owner", PathIDs: []string{"allowed", "denied"}}, {ParticipantID: "second-owner", PathIDs: []string{"second-path"}}}}, nil
}

type reminderAccess struct {
	longTimerAccess
	fail bool
}

func (a *reminderAccess) Check(_ context.Context, kind, id, permission, user string) (bool, error) {
	if a.fail {
		return false, errors.New("authorization unavailable")
	}
	return kind == "path" && permission == "track" && ((user == "owner" && id == "allowed") || (user == "second-owner" && id == "second-path")), nil
}
func (q *reminderQueue) PublishGoalReminders(_ context.Context, c GoalReminderDeliveryCommand) (int, error) {
	q.published = append(q.published, c)
	return 1, nil
}
func TestGoalReminderWorkerKeepsParticipantBundleAndFailsClosed(t *testing.T) {
	q := &reminderQueue{}
	access := &reminderAccess{fail: true}
	events := &longTimerAudit{}
	w := GoalReminderWorker{Repository: q, Authorizer: access, Clock: longTimerClock{time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)}, Audits: events, RateLimiter: longTimerLimiter{}, NewID: func() string { return "reminder-audit" }, BatchSize: 10}
	if n, more, err := w.RunOnce(context.Background()); err != nil || n != 0 || !more {
		t.Fatalf("filtered page %d %v %v", n, more, err)
	}
	if _, _, err := w.RunOnce(context.Background()); err == nil || len(q.published) != 0 {
		t.Fatal("authorization failure published reminder")
	}
	access.fail = false
	if n, more, err := w.RunOnce(context.Background()); err != nil || n != 2 || more {
		t.Fatalf("delivery %d %v %v", n, more, err)
	}
	if len(q.published) != 2 || q.published[0].ParticipantID != "owner" || len(q.published[0].AuthorizedPathIDs) != 1 || q.published[0].AuthorizedPathIDs[0] != "allowed" {
		t.Fatalf("unsafe bundle %+v", q.published)
	}
	if q.published[1].ParticipantID != "second-owner" || len(q.published[1].AuthorizedPathIDs) != 1 || q.published[1].AuthorizedPathIDs[0] != "second-path" {
		t.Fatalf("cross-participant bundle %+v", q.published[1])
	}
	if len(events.events) != 1 || events.events[0].Outcome != audit.Denied {
		t.Fatalf("missing denial audit %+v", events.events)
	}
}

func (q *reminderQueue) GoalReminderCandidateForParticipant(_ context.Context, _ time.Time, owner string) (GoalReminderCandidate, error) {
	return GoalReminderCandidate{ParticipantID: owner, PathIDs: []string{"allowed", "denied"}}, nil
}

func TestGoalReminderParticipantRecalculationDoesNotAdvanceScanOrPublishOtherOwners(t *testing.T) {
	q := &reminderQueue{}
	w := GoalReminderWorker{Repository: q, Authorizer: &reminderAccess{}, Clock: longTimerClock{time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)}, Audits: &longTimerAudit{}, RateLimiter: longTimerLimiter{}, NewID: func() string { return "reminder-audit" }, BatchSize: 10, afterParticipantID: "scan-position"}
	n, err := w.RunParticipant(context.Background(), "owner")
	if err != nil || n != 1 || w.afterParticipantID != "scan-position" || len(q.published) != 1 || q.published[0].ParticipantID != "owner" || len(q.published[0].AuthorizedPathIDs) != 1 {
		t.Fatalf("participant recalculation: n=%d err=%v queue=%+v cursor=%s", n, err, q.published, w.afterParticipantID)
	}
}
