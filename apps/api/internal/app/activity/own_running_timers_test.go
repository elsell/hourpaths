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

type ownTimerRepository struct {
	testRepository
	candidates []RunningTimerCandidate
	owners     []string
}

func (r *ownTimerRepository) ListRunningTimerCandidates(_ context.Context, owner string, page RunningTimerPageRequest) ([]RunningTimerCandidate, error) {
	r.owners = append(r.owners, owner)
	return r.candidates, nil
}

type ownTimerAuthorizer struct{ testAuthorizer }

func (a ownTimerAuthorizer) Check(_ context.Context, kind, id, permission, owner string) (bool, error) {
	if a.err != nil {
		return false, a.err
	}
	return kind == "path" && id != "hidden" && permission == "track" && owner == "user-1", nil
}
func TestOwnRunningTimersAuthorizeEachPathAndBindPaginationToOwner(t *testing.T) {
	timer := func(id, path string, ago time.Duration) RunningTimerCandidate {
		v, _ := domain.StartTimer(id, path, "user-1", testNow.Add(-ago), "Etc/UTC", testNow)
		return RunningTimerCandidate{Timer: v, PathName: path}
	}
	repo := &ownTimerRepository{candidates: []RunningTimerCandidate{timer("timer-hidden", "hidden", 3*time.Hour), timer("timer-visible", "visible", 2*time.Hour), timer("timer-next", "next", time.Hour)}}
	service := testService(repo)
	service.Authorizer = ownTimerAuthorizer{}
	var events []audit.Event
	service.Audits = testAudits{events: &events}
	items, cursor, err := service.ListOwnRunningTimers(context.Background(), "Bearer session", "", 2)
	if err != nil || len(items) != 1 || items[0].Timer.PathID != "visible" || cursor == "" {
		t.Fatalf("list=%+v cursor=%q err=%v", items, cursor, err)
	}
	if len(repo.owners) != 1 || repo.owners[0] != "user-1" || len(events) != 2 || events[0].Outcome != audit.Denied || events[1].Action != audit.ResourceListed {
		t.Fatalf("scope/audit=%v %+v", repo.owners, events)
	}
	service.Auth = testAuth{principal: ports.Principal{UserID: "someone-else", Scopes: []string{"api:user"}}}
	if _, _, err = service.ListOwnRunningTimers(context.Background(), "Bearer other", cursor, 2); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("foreign cursor=%v", err)
	}
	if len(repo.owners) != 1 {
		t.Fatal("foreign cursor reached store")
	}
}
func TestOwnRunningTimersFailClosedForForeignRowsAndAuthorizationFailure(t *testing.T) {
	v, _ := domain.StartTimer("timer", "path", "user-1", testNow.Add(-time.Hour), "Etc/UTC", testNow)
	repo := &ownTimerRepository{candidates: []RunningTimerCandidate{{Timer: v, PathName: "Path"}}}
	service := testService(repo)
	service.Authorizer = ownTimerAuthorizer{testAuthorizer{err: ports.ErrUnavailable}}
	if _, _, err := service.ListOwnRunningTimers(context.Background(), "Bearer session", "", 50); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("dependency=%v", err)
	}
	service.Authorizer = ownTimerAuthorizer{}
	repo.candidates[0].Timer.ParticipantID = "another-user"
	if items, _, err := service.ListOwnRunningTimers(context.Background(), "Bearer session", "", 50); err == nil || len(items) > 0 {
		t.Fatalf("foreign row=%+v %v", items, err)
	}
}

func (testRepository) ListRunningTimerCandidates(context.Context, string, RunningTimerPageRequest) ([]RunningTimerCandidate, error) {
	return nil, nil
}
