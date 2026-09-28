package social

import (
	"context"
	"errors"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type profilePathCandidates struct {
	controlledFeed
	items               []ProfilePathCandidate
	viewer, participant string
}

func (r *profilePathCandidates) ListProfilePathCandidates(_ context.Context, viewer, participant string, _ time.Time) ([]ProfilePathCandidate, error) {
	r.viewer, r.participant = viewer, participant
	return r.items, r.err
}
func TestProfilePathsOnlyCountAuthorizedParticipationAndFailClosed(t *testing.T) {
	ctx := context.Background()
	profiles := &controlledProfiles{profile: domain.PublicProfile{ID: "alice", Username: "alice", DisplayName: "Alice", Relationship: domain.RelationshipNone}}
	audits := &controlledAudits{}
	service := testService(profiles, audits)
	repository := &profilePathCandidates{items: []ProfilePathCandidate{{ID: "visible", Timer: &ActiveFollowingTimer{ID: "timer", PathID: "visible"}}, {ID: "hidden", Timer: &ActiveFollowingTimer{ID: "secret", PathID: "hidden"}}}}
	service.Feed = repository
	authorizer := &feedAuthorizer{allowed: map[string]bool{"visible": true}}
	service.Authorizer = authorizer
	result, err := service.GetProfilePaths(ctx, "Bearer session", "alice")
	if err != nil || result.Count != 1 || len(result.Active.Timers) != 1 || result.Active.Timers[0].ID != "timer" || repository.viewer != "viewer" || repository.participant != "alice" || len(audits.events) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	authorizer.err = ports.ErrUnavailable
	if result, err := service.GetProfilePaths(ctx, "Bearer session", "alice"); !errors.Is(err, ports.ErrUnavailable) || result.Count != 0 {
		t.Fatalf("authorization failure: %+v %v", result, err)
	}
	authorizer.err = nil
	profiles.err = ports.ErrNotFound
	if _, err := service.GetProfilePaths(ctx, "Bearer session", "alice"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal(err)
	}
	profiles.err = nil
	audits.err = ports.ErrUnavailable
	if _, err := service.GetProfilePaths(ctx, "Bearer session", "alice"); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatal(err)
	}
	for _, credential := range []error{ports.ErrInvalidCredential, ports.ErrUnavailable} {
		service.Auth = controlledAuth{err: credential}
		if _, err := service.GetProfilePaths(ctx, "Bearer bad", "alice"); !errors.Is(err, credential) {
			t.Fatal(err)
		}
	}
}
