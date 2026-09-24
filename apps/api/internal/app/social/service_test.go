package social

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledAuth struct {
	principal ports.Principal
	err       error
}

func (a controlledAuth) Authenticate(context.Context, string) (ports.Principal, error) {
	return a.principal, a.err
}

type controlledProfiles struct {
	search               ProfilePage
	profile              domain.PublicProfile
	err                  error
	viewer, query, after string
	afterRank            int
	limit                int
}

func (r *controlledProfiles) Search(_ context.Context, viewer, query string, afterRank int, after string, limit int) (ProfilePage, error) {
	r.viewer, r.query, r.afterRank, r.after, r.limit = viewer, query, afterRank, after, limit
	return r.search, r.err
}
func (r *controlledProfiles) GetByUsername(_ context.Context, viewer, username string) (domain.PublicProfile, error) {
	r.viewer, r.query = viewer, username
	return r.profile, r.err
}

type controlledAudits struct {
	events []audit.Event
	err    error
}

func (a *controlledAudits) AppendAuditEvent(_ context.Context, event audit.Event) error {
	a.events = append(a.events, event)
	return a.err
}
func (*controlledAudits) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}

type allowLimiter struct{ allow bool }

func (l allowLimiter) Allow(string, time.Time) bool { return l.allow }

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func testService(repository *controlledProfiles, audits *controlledAudits) *Service {
	return New(Dependencies{
		Auth:     controlledAuth{principal: ports.Principal{UserID: "viewer", Scopes: []string{"api:user"}}},
		Profiles: repository, Audits: audits, AuditRateLimiter: allowLimiter{allow: true},
		Clock:            fixedClock{now: time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)},
		CursorSigningKey: []byte("0123456789abcdef0123456789abcdef"),
	})
}

func TestSearchProfilesNormalizesRanksPagesAndAuditsViewerScopedRead(t *testing.T) {
	repository := &controlledProfiles{search: ProfilePage{Profiles: []domain.PublicProfile{{ID: "2", Username: "elise", DisplayName: "Élise", Relationship: domain.RelationshipNone}, {ID: "1", Username: "elan", DisplayName: "Élan", Relationship: domain.RelationshipFollowing}}, HasMore: true, LastRank: 2}}
	audits := &controlledAudits{}
	service := testService(repository, audits)
	profiles, cursor, err := service.Search(context.Background(), "Bearer session", " E\u0301L ", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || repository.viewer != "viewer" || repository.query != "él" || repository.limit != 2 || cursor == "" {
		t.Fatalf("profiles=%+v repository=%+v cursor=%q", profiles, repository, cursor)
	}
	if len(audits.events) != 1 || audits.events[0].Action != audit.ResourceListed || audits.events[0].OwnerUserID != "viewer" || audits.events[0].TargetType != "profile" {
		t.Fatalf("audit=%+v", audits.events)
	}
	_, _, err = service.Search(context.Background(), "Bearer session", "él", cursor, 2)
	if err != nil || repository.after != "elan" || repository.afterRank != 2 {
		t.Fatalf("paged search after=(%d,%q) err=%v", repository.afterRank, repository.after, err)
	}
}

func TestSearchProfilesFailsClosedBeforePersistenceForInvalidOrUnauthenticatedRequests(t *testing.T) {
	for _, test := range []struct {
		name, query string
		configure   func(*Service)
		want        error
	}{
		{name: "short", query: "a", want: ports.ErrInvalidArgument},
		{name: "unauthenticated", query: "al", configure: func(service *Service) { service.Auth = controlledAuth{err: ports.ErrInvalidCredential} }, want: ports.ErrInvalidCredential},
		{name: "wrong scope", query: "al", configure: func(service *Service) {
			service.Auth = controlledAuth{principal: ports.Principal{UserID: "viewer", Scopes: []string{"api:onboarding"}}}
		}, want: platformapp.ErrUnauthenticated},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository, audits := &controlledProfiles{}, &controlledAudits{}
			service := testService(repository, audits)
			if test.configure != nil {
				test.configure(service)
			}
			_, _, err := service.Search(context.Background(), "Bearer session", test.query, "", 25)
			if !errors.Is(err, test.want) || repository.viewer != "" || len(audits.events) != 0 {
				t.Fatalf("err=%v repository=%+v audits=%+v", err, repository, audits.events)
			}
		})
	}
}

func TestProfileReadsExposeOnlyPublicProjectionAndAuditOrFailClosed(t *testing.T) {
	want := domain.PublicProfile{ID: "target", Username: "alice", DisplayName: "Alice", Description: "Runs", FollowerCount: 4, FollowingCount: 5, Relationship: domain.RelationshipRequested}
	repository, audits := &controlledProfiles{profile: want}, &controlledAudits{}
	service := testService(repository, audits)
	got, err := service.Get(context.Background(), "Bearer session", "ALICE")
	if err != nil || got != want || repository.viewer != "viewer" || repository.query != "alice" {
		t.Fatalf("profile=%+v err=%v repository=%+v", got, err, repository)
	}
	if len(audits.events) != 1 || audits.events[0].Action != audit.ResourceViewed || audits.events[0].TargetID != "target" {
		t.Fatalf("audit=%+v", audits.events)
	}
	audits.err = errors.New("audit unavailable")
	if leaked, err := service.Get(context.Background(), "Bearer session", "alice"); err == nil || leaked != (domain.PublicProfile{}) {
		t.Fatalf("audit failure leaked profile=%+v err=%v", leaked, err)
	}
}

func TestHiddenProfileReadIsAuditedWithoutDisclosingTheRequestedIdentity(t *testing.T) {
	repository, audits := &controlledProfiles{err: ports.ErrNotFound}, &controlledAudits{}
	service := testService(repository, audits)
	if _, err := service.Get(context.Background(), "Bearer session", "hidden"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
	if len(audits.events) != 1 || audits.events[0].Action != audit.ResourceAccessDenied || audits.events[0].Outcome != audit.Denied || audits.events[0].TargetID != "hidden" {
		t.Fatalf("denial audit=%+v", audits.events)
	}
	audits.err = errors.New("audit unavailable")
	if _, err := service.Get(context.Background(), "Bearer session", "hidden"); err == nil || errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("audit failure did not fail closed: %v", err)
	}
}
