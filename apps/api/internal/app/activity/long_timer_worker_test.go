package activity

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type longTimerQueue struct {
	candidates []LongTimerCandidate
	admitted   []LongTimerNoticeCommand
}

func (q *longTimerQueue) ListLongTimerCandidates(_ context.Context, _ time.Time, after string, limit int) ([]LongTimerCandidate, error) {
	var page []LongTimerCandidate
	for _, c := range q.candidates {
		if c.TimerID > after && len(page) < limit {
			page = append(page, c)
		}
	}
	return page, nil
}
func (q *longTimerQueue) PublishLongTimerNotice(_ context.Context, c LongTimerNoticeCommand) (bool, error) {
	q.admitted = append(q.admitted, c)
	return true, nil
}

type longTimerAccess struct{ fail bool }

func (a *longTimerAccess) Check(_ context.Context, kind, id, permission, user string) (bool, error) {
	if a.fail {
		return false, errors.New("authorization unavailable")
	}
	return kind == "path" && permission == "track" && user == "owner" && id == "allowed", nil
}

type longTimerClock struct{ at time.Time }

func (c longTimerClock) Now() time.Time { return c.at }

type longTimerAudit struct{ events []audit.Event }

func (a *longTimerAudit) AppendAuditEvent(_ context.Context, e audit.Event) error {
	a.events = append(a.events, e)
	return nil
}
func (a *longTimerAudit) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}

type longTimerLimiter struct{}

func (longTimerLimiter) Allow(string, time.Time) bool { return true }
func TestLongTimerWorkerFailsClosedAndContinuesAcrossCandidatePages(t *testing.T) {
	q := &longTimerQueue{candidates: []LongTimerCandidate{{"a", "denied", "owner"}, {"b", "allowed", "owner"}}}
	access := &longTimerAccess{fail: true}
	audits := &longTimerAudit{}
	w := LongTimerNoticeWorker{Repository: q, Authorizer: access, Clock: longTimerClock{time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}, Audits: audits, RateLimiter: longTimerLimiter{}, NewID: func() string { return "audit-id" }, BatchSize: 1}
	if _, err := w.RunOnce(context.Background()); err == nil || len(q.admitted) != 0 {
		t.Fatal("authorization failure admitted notification")
	}
	access.fail = false
	if count, err := w.RunOnce(context.Background()); err != nil || count != 0 || len(audits.events) != 1 || audits.events[0].Outcome != audit.Denied {
		t.Fatalf("denial: %d %v %+v", count, err, audits.events)
	}
	if count, err := w.RunOnce(context.Background()); err != nil || count != 1 || len(q.admitted) != 1 {
		t.Fatalf("second page starved: %d %v", count, err)
	}
	if c := q.admitted[0]; c.Candidate.TimerID != "b" || c.Audit.ActorUserID != "owner" || !c.Audit.OccurredAt.Equal(w.Clock.Now()) {
		t.Fatalf("unsafe command %+v", c)
	}
}

func (*longTimerAccess) WriteRelationship(context.Context, string, string, string, string, string) error {
	return errors.New("relationship writes unsupported")
}
func (*longTimerAccess) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return errors.New("relationship writes unsupported")
}
