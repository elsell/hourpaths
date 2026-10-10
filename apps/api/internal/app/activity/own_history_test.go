package activity

import (
	"context"
	"errors"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type ownHistoryRepository struct {
	testRepository
	page     ActivityPage
	requests []OwnHistoryPageRequest
	owners   []string
}

func (r *ownHistoryRepository) ListOwnActivityCandidates(_ context.Context, owner string, page OwnHistoryPageRequest) (ActivityPage, error) {
	r.owners = append(r.owners, owner)
	r.requests = append(r.requests, page)
	return r.page, nil
}
func (testRepository) ListOwnActivityCandidates(context.Context, string, OwnHistoryPageRequest) (ActivityPage, error) {
	return ActivityPage{}, nil
}

type ownHistoryAuthorizer struct{ testAuthorizer }

func (a ownHistoryAuthorizer) Check(_ context.Context, kind, id, permission, owner string) (bool, error) {
	if a.err != nil {
		return false, a.err
	}
	return kind == "path" && id != "hidden" && permission == "view" && owner == "user-1", nil
}
func historyEntry(t *testing.T, id, path string, ago time.Duration) ActivityListRecord {
	t.Helper()
	entry, err := domain.RecordManualActivity(domain.ManualActivity{ID: id, PathID: path, ParticipantID: "user-1", StartedAt: testNow.Add(-ago), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC", Note: "Private note"}, testNow.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return ActivityListRecord{Activity: entry, Version: 1, PathName: path}
}
func TestOwnHistoryPaginationScopesOwnerWindowAndAuthorizesPaths(t *testing.T) {
	repo := &ownHistoryRepository{page: ActivityPage{Items: []ActivityListRecord{historyEntry(t, "a", "hidden", time.Hour), historyEntry(t, "b", "visible", 2*time.Hour)}, HasMore: true}}
	service := testService(repo)
	service.Authorizer = ownHistoryAuthorizer{}
	var events []audit.Event
	service.Audits = testAudits{events: &events}
	items, cursor, err := service.ListOwnHistory(context.Background(), "Bearer valid", "retained", "", 2)
	if err != nil || len(items) != 1 || items[0].Activity.ID != "b" || cursor == "" {
		t.Fatalf("items=%+v cursor=%q err=%v", items, cursor, err)
	}
	if len(repo.requests) != 1 || repo.owners[0] != "user-1" || !repo.requests[0].EndedSince.Equal(testNow.Add(-90*24*time.Hour)) {
		t.Fatalf("unscoped retention: %+v %v", repo.requests, repo.owners)
	}
	if len(events) != 2 || events[0].Outcome != audit.Denied || events[1].Action != audit.ResourceListed {
		t.Fatalf("audit=%+v", events)
	}
	if _, _, err = service.ListOwnHistory(context.Background(), "Bearer valid", "all", cursor, 2); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("changed window=%v", err)
	}
	service.Auth = testAuth{principal: ports.Principal{UserID: "other", Scopes: []string{"api:user"}}}
	if _, _, err = service.ListOwnHistory(context.Background(), "Bearer valid", "retained", cursor, 2); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("foreign cursor=%v", err)
	}
	if len(repo.requests) != 1 {
		t.Fatal("invalid cursor reached repository")
	}
}

func TestOwnHistoryFailsClosedAtSecurityBoundaries(t *testing.T) {
	failure := errors.New("dependency unavailable")
	cases := []struct {
		name   string
		change func(*Service, *ownHistoryRepository)
	}{
		{"rejected credential", func(s *Service, _ *ownHistoryRepository) { s.Auth = testAuth{err: ports.ErrInvalidCredential} }},
		{"missing scope", func(s *Service, _ *ownHistoryRepository) {
			s.Auth = testAuth{principal: ports.Principal{UserID: "user-1"}}
		}},
		{"authorization unavailable", func(s *Service, _ *ownHistoryRepository) { s.Authorizer = testAuthorizer{err: failure} }},
		{"audit unavailable", func(s *Service, _ *ownHistoryRepository) { s.Audits = testAudits{err: failure} }},
		{"limited principal", func(s *Service, _ *ownHistoryRepository) { s.AuditRateLimiter = testLimiter{allowed: false} }},
		{"foreign record", func(_ *Service, r *ownHistoryRepository) { r.page.Items[0].Activity.ParticipantID = "other" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &ownHistoryRepository{page: ActivityPage{Items: []ActivityListRecord{historyEntry(t, "a", "visible", time.Hour)}}}
			service := testService(repo)
			service.Authorizer = ownHistoryAuthorizer{}
			tc.change(service, repo)
			items, next, err := service.ListOwnHistory(context.Background(), "Bearer valid", "all", "", 10)
			if err == nil || len(items) != 0 || next != "" {
				t.Fatalf("failed open: items=%+v next=%q err=%v", items, next, err)
			}
		})
	}
}
