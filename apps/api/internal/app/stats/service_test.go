package stats

import (
	"context"
	"errors"
	platform "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/stats"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type controlled struct {
	snapshot                         Snapshot
	actor                            string
	events                           []audit.Event
	denied                           bool
	authErr, errorPolicy, errorAudit error
}

func (f *controlled) Authenticate(context.Context, string) (ports.Principal, error) {
	return ports.Principal{UserID: "viewer", Scopes: []string{"api:user"}}, f.authErr
}
func (f *controlled) Read(_ context.Context, actor string) (Snapshot, error) {
	f.actor = actor
	return f.snapshot, nil
}
func (f *controlled) Check(_ context.Context, kind, id, permission, actor string) (bool, error) {
	if kind != "path" || permission != "view" || actor != "viewer" {
		panic("incorrect policy scope")
	}
	return !f.denied, f.errorPolicy
}
func (f *controlled) WriteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *controlled) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *controlled) AppendAuditEvent(_ context.Context, e audit.Event) error {
	f.events = append(f.events, e)
	return f.errorAudit
}
func (f *controlled) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}
func (f *controlled) Allow(string, time.Time) bool { return true }
func (f *controlled) Now() time.Time               { return time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC) }
func TestStatsScopedAuthorizationAndAuditFailClosed(t *testing.T) {
	for _, name := range []string{"success", "unauthenticated", "policy denied", "policy outage", "audit outage", "foreign filter"} {
		t.Run(name, func(t *testing.T) {
			f := &controlled{}
			f.snapshot = Snapshot{TimeZone: "Etc/UTC", FirstDayOfWeek: 1, Paths: []domain.StatsPath{{ID: "p", Name: "Piano"}}, Records: []domain.Record{{PathID: "p", StartedAt: f.Now().Add(-time.Hour), EndedAt: f.Now(), TimeZone: "Etc/UTC"}}}
			filter := ""
			switch name {
			case "unauthenticated":
				f.authErr = platform.ErrUnauthenticated
			case "policy denied":
				f.denied = true
			case "policy outage":
				f.errorPolicy = errors.New("offline")
			case "audit outage":
				f.errorAudit = errors.New("offline")
			case "foreign filter":
				filter = "someone-elses-path"
			}
			s := Service{Auth: f, Repository: f, Authorizer: f, Audits: f, AuditRateLimiter: f, Clock: f}
			result, err := s.Get(context.Background(), "session", "day", "", filter)
			if name == "success" {
				if err != nil || result.TotalSeconds != 3600 || f.actor != "viewer" || len(f.events) != 1 {
					t.Fatalf("%+v %v", result, err)
				}
			} else if err == nil || result.TotalSeconds != 0 {
				t.Fatalf("leaked data %+v %v", result, err)
			}
			if name == "unauthenticated" && f.actor != "" {
				t.Fatal("unauthenticated repository access")
			}
		})
	}
}
