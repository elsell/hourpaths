package moderation

import (
	"context"
	"errors"
	"testing"
	"time"

	app "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type reportFixture struct {
	principal          ports.Principal
	authErr, accessErr error
	allowed            bool
	reads, writes      int
	events             []audit.Event
	command            ReportCommand
}

func (f *reportFixture) Authenticate(context.Context, string) (ports.Principal, error) {
	return f.principal, f.authErr
}
func (f *reportFixture) Now() time.Time               { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) }
func (f *reportFixture) Allow(string, time.Time) bool { return true }
func (f *reportFixture) AppendAuditEvent(_ context.Context, e audit.Event) error {
	f.events = append(f.events, e)
	return nil
}
func (f *reportFixture) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}
func (f *reportFixture) WriteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *reportFixture) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *reportFixture) Check(context.Context, string, string, string, string) (bool, error) {
	return f.allowed, f.accessErr
}
func (f *reportFixture) FindReportReplay(context.Context, ports.Idempotency) (Receipt, bool, error) {
	return Receipt{}, false, nil
}
func (f *reportFixture) ResolveReportTarget(_ context.Context, viewer string, target domain.Target, at time.Time) (TargetAccess, error) {
	f.reads++
	if viewer != "reporter" {
		return TargetAccess{}, ports.ErrNotFound
	}
	return TargetAccess{Target: target, SubjectUserID: "subject", PathID: "path"}, nil
}
func (f *reportFixture) SubmitReport(_ context.Context, c ReportCommand) (Receipt, error) {
	f.writes++
	f.command = c
	return Receipt{ID: c.ID}, nil
}

func TestSubmissionFailsClosedBeforeCreatingCase(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		principal                     ports.Principal
		authErr, accessErr, errorWant error
		allowed                       bool
		reads                         int
	}{
		{name: "invalid session", authErr: app.ErrUnauthenticated, errorWant: app.ErrUnauthenticated},
		{name: "provisional", principal: ports.Principal{UserID: "reporter", Scopes: []string{"onboarding"}}, errorWant: app.ErrUnauthenticated},
		{name: "denied", principal: ports.Principal{UserID: "reporter", Scopes: []string{"api:user"}}, errorWant: ports.ErrNotFound, reads: 1},
		{name: "dependency failure", principal: ports.Principal{UserID: "reporter", Scopes: []string{"api:user"}}, accessErr: ports.ErrUnavailable, errorWant: ports.ErrUnavailable, reads: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &reportFixture{principal: tc.principal, authErr: tc.authErr, accessErr: tc.accessErr, allowed: tc.allowed}
			s := New(Dependencies{Auth: f, Repository: f, Authorizer: f, Audits: f, RateLimiter: f, Clock: f, NewID: func() string { return "case" }})
			_, err := s.Submit(context.Background(), "Bearer session", domain.Target{Kind: domain.Path, ID: "path"}, domain.SpamOrScam, "context", "0123456789abcdef")
			if !errors.Is(err, tc.errorWant) || f.writes != 0 || f.reads != tc.reads {
				t.Fatalf("err=%v writes=%d reads=%d", err, f.writes, f.reads)
			}
			if tc.name == "denied" && (len(f.events) != 1 || f.events[0].Outcome != audit.Denied) {
				t.Fatal("missing denial audit")
			}
		})
	}
}

func TestSubmissionBindsNormalizedEvidenceToAuthenticatedReporter(t *testing.T) {
	f := &reportFixture{principal: ports.Principal{UserID: "reporter", Scopes: []string{"api:user"}}, allowed: true}
	s := New(Dependencies{Auth: f, Repository: f, Authorizer: f, Audits: f, RateLimiter: f, Clock: f, NewID: func() string { return "case" }})
	receipt, err := s.Submit(context.Background(), "Bearer session", domain.Target{Kind: domain.Path, ID: "path"}, domain.SpamOrScam, "  context\n", "0123456789abcdef")
	if err != nil || receipt.ID != "case" || f.writes != 1 {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	c := f.command
	if c.ReporterID != "reporter" || c.Explanation != "context" || c.Idempotency.PrincipalID != "reporter" || len(c.Idempotency.RequestHash) != 32 || !c.Audit.Valid() || c.Audit.OwnerUserID != "reporter" {
		t.Fatalf("incorrect report binding: %+v", c)
	}
}
