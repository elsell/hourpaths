package liveactivityapp

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/liveactivity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type workerClock struct{ now time.Time }

func (c workerClock) Now() time.Time { return c.now }

type deliveryProvider struct {
	sent    []ports.LiveActivityRefresh
	outcome ports.LiveActivityDelivery
}

func (p *deliveryProvider) Refresh(_ context.Context, r ports.LiveActivityRefresh) (ports.LiveActivityDelivery, error) {
	p.sent = append(p.sent, r)
	return p.outcome, nil
}

type deliveryRepository struct {
	claim      ports.LiveActivityClaim
	eligible   bool
	transition ports.LiveActivityTransition
}

func (r *deliveryRepository) ClaimLiveActivities(context.Context, string, time.Time, time.Duration, int) ([]ports.LiveActivityClaim, error) {
	return []ports.LiveActivityClaim{r.claim}, nil
}
func (r *deliveryRepository) HandoffLiveActivity(_ context.Context, _ string, _ ports.LiveActivityClaim, _ time.Time, callback func(bool) ports.LiveActivityTransition) error {
	r.transition = callback(r.eligible)
	return nil
}

func TestRefreshWorkerUsesCurrentDeadlineAndRechecksEligibilityAtHandoff(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	for _, eligible := range []bool{true, false} {
		repository := &deliveryRepository{eligible: eligible, claim: ports.LiveActivityClaim{Version: 2, Registration: liveactivity.Registration{ID: "activity", OwnerUserID: "alice", AccountLease: "lease", Token: "abab", ExpiresAt: now.Add(2 * time.Hour), Boundaries: []time.Time{now.Add(-time.Minute), now.Add(time.Hour)}}}}
		provider := &deliveryProvider{outcome: ports.LiveActivityAccepted}
		worker := Worker{Repository: repository, Provider: provider, Clock: workerClock{now}, WorkerID: "worker", NewID: func() string { return "event" }}
		if count, err := worker.RunOnce(context.Background()); err != nil || count != 1 {
			t.Fatalf("count=%d err=%v", count, err)
		}
		if !repository.transition.Audit.Valid() {
			t.Fatal("worker transition not audited")
		}
		if eligible {
			if len(provider.sent) != 1 || !provider.sent[0].StaleAt.Equal(now.Add(time.Hour)) || !repository.transition.AvailableAt.Equal(now.Add(time.Hour)) {
				t.Fatal("late refresh replayed an old boundary")
			}
		} else if len(provider.sent) != 0 || !repository.transition.Retired {
			t.Fatal("revoked registration was delivered")
		}
	}
}

func TestRefreshWorkerRetriesTransportButRetiresInvalidTokens(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	for _, outcome := range []ports.LiveActivityDelivery{ports.LiveActivityRetry, ports.LiveActivityConfigurationFailure, ports.LiveActivityTokenInvalid} {
		repository := &deliveryRepository{eligible: true, claim: ports.LiveActivityClaim{Version: 1, Failures: 8, Registration: liveactivity.Registration{ID: "activity", OwnerUserID: "alice", AccountLease: "lease", Token: "abab", ExpiresAt: now.Add(time.Minute)}}}
		worker := Worker{Repository: repository, Provider: &deliveryProvider{outcome: outcome}, Clock: workerClock{now}, WorkerID: "worker", NewID: func() string { return "event" }}
		if _, err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if repository.transition.Retired != (outcome == ports.LiveActivityTokenInvalid) {
			t.Fatal("wrong token retirement")
		}
		if repository.transition.AvailableAt.After(repository.claim.Registration.ExpiresAt) {
			t.Fatal("retry extended native lifetime")
		}
	}
}
