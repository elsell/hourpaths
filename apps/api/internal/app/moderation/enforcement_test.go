package moderation

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type noticeFixture struct {
	reportFixture
	notice    domain.Enforcement
	submitted *AppealCommand
	owner     string
	now       time.Time
	tick      time.Duration
}

func (f *noticeFixture) Now() time.Time {
	if !f.now.IsZero() {
		result := f.now
		f.now = f.now.Add(f.tick)
		return result
	}
	return f.reportFixture.Now()
}
func (f *noticeFixture) ReadNotice(_ context.Context, owner, id string, event audit.Event) (Notice, error) {
	f.owner = owner
	if owner != f.notice.SubjectUserID || id != f.notice.ID {
		return Notice{}, ports.ErrNotFound
	}
	f.events = append(f.events, event)
	n := Notice{Decision: f.notice}
	if f.submitted != nil {
		a := f.submitted.Appeal
		n.Appeal = &a
	}
	return n, nil
}
func (f *noticeFixture) SubmitAppeal(_ context.Context, c AppealCommand) (domain.Appeal, error) {
	if c.OwnerID != f.notice.SubjectUserID {
		return domain.Appeal{}, ports.ErrNotFound
	}
	if f.submitted != nil {
		if c.Appeal.ID == f.submitted.Appeal.ID && c.Appeal.Explanation == f.submitted.Appeal.Explanation {
			return f.submitted.Appeal, nil
		}
		return domain.Appeal{}, ports.ErrConflict
	}
	f.submitted = &c
	f.events = append(f.events, c.Audit)
	return c.Appeal, nil
}
func TestAppealIsOwnedByAuthenticatedSubject(t *testing.T) {
	f := &noticeFixture{reportFixture: reportFixture{principal: ports.Principal{UserID: "subject", Scopes: []string{"api:user"}}}}
	f.notice = domain.Enforcement{ID: "notice", SubjectUserID: "subject", Action: domain.Warning, PolicyReason: "Policy", IssuedAt: f.Now().Add(-time.Hour)}
	s := NewEnforcements(EnforcementDependencies{Auth: f, Repository: f, Audits: f, RateLimiter: f, Clock: f})
	a, err := s.Appeal(context.Background(), "session", "notice", "0123456789abcdef", "  context  ")
	if err != nil || a.Explanation != "context" || f.owner != "subject" || f.submitted == nil || !f.submitted.Audit.Valid() {
		t.Fatalf("appeal=%+v err=%v command=%+v", a, err, f.submitted)
	}
	f.now = f.Now().Add(31 * 24 * time.Hour)
	if _, err = s.Appeal(context.Background(), "session", "notice", "0123456789abcdef", "context"); err != nil {
		t.Fatal("retry failed", err)
	}
	if _, err = s.Appeal(context.Background(), "session", "notice", "other-submission-id", "different"); !errors.Is(err, ports.ErrConflict) {
		t.Fatal("second appeal accepted", err)
	}
	f.principal.UserID = "other"
	if _, err = s.Get(context.Background(), "session", "notice"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("cross-account notice exposed", err)
	}
	if _, err = s.Appeal(context.Background(), "session", "notice", "other-submission-id", "context"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("cross-account appeal accepted", err)
	}
}

func (f *noticeFixture) ListNotices(_ context.Context, owner string, page ports.PageRequest, event audit.Event) (NoticePage, error) {
	f.owner = owner
	if owner != f.notice.SubjectUserID {
		return NoticePage{}, nil
	}
	f.events = append(f.events, event)
	return NoticePage{Items: []Notice{{Decision: f.notice}}, HasMore: true}, nil
}
func TestNoticeCursorCannotCrossAccounts(t *testing.T) {
	f := &noticeFixture{reportFixture: reportFixture{principal: ports.Principal{UserID: "subject", Scopes: []string{"api:user"}}}}
	f.notice = domain.Enforcement{ID: "notice", SubjectUserID: "subject", Action: domain.Warning, PolicyReason: "Policy", IssuedAt: f.Now().Add(-time.Hour)}
	s := NewEnforcements(EnforcementDependencies{Auth: f, Repository: f, Audits: f, RateLimiter: f, Clock: f, CursorSigningKey: bytes.Repeat([]byte{7}, 32)})
	items, cursor, err := s.List(context.Background(), "session", "", 1)
	if err != nil || len(items) != 1 || cursor == "" {
		t.Fatal("missing notice page", err)
	}
	f.principal.UserID = "other"
	f.owner = ""
	if _, _, err = s.List(context.Background(), "session", cursor, 1); !errors.Is(err, ports.ErrInvalidArgument) || f.owner != "" {
		t.Fatal("cross-account cursor reached repository", err)
	}
}

func TestAppealAuditUsesSubmissionInstantWithAdvancingClock(t *testing.T) {
	f := &noticeFixture{reportFixture: reportFixture{principal: ports.Principal{UserID: "subject", Scopes: []string{"api:user"}}}, now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC), tick: time.Millisecond}
	f.notice = domain.Enforcement{ID: "notice", SubjectUserID: "subject", Action: domain.Warning, PolicyReason: "Policy", IssuedAt: f.now.Add(-time.Hour)}
	s := NewEnforcements(EnforcementDependencies{Auth: f, Repository: f, Audits: f, RateLimiter: f, Clock: f})
	if _, err := s.Appeal(context.Background(), "session", "notice", "0123456789abcdef", "context"); err != nil {
		t.Fatal(err)
	}
	if f.submitted == nil || !f.submitted.Audit.OccurredAt.Equal(f.submitted.Appeal.SubmittedAt) {
		t.Fatal("audit timestamp differs from persisted submission")
	}
}
