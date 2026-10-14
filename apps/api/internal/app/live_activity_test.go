package app

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/liveactivity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type activityRegistrationAuth struct{}

func (activityRegistrationAuth) Authenticate(_ context.Context, token string) (ports.Principal, error) {
	if token != "Bearer alice" {
		return ports.Principal{}, ErrUnauthenticated
	}
	return ports.Principal{UserID: "alice", Scopes: []string{"api:user"}}, nil
}

type activityRegistrationRepository struct {
	registrations map[string]liveactivity.Registration
	events        []audit.Event
}

func (r *activityRegistrationRepository) UpsertLiveActivity(_ context.Context, value liveactivity.Registration, event audit.Event) error {
	if !event.Valid() || event.OwnerUserID != value.OwnerUserID {
		panic("registration lacks owned audit")
	}
	r.registrations[value.ID] = value
	r.events = append(r.events, event)
	return nil
}
func (r *activityRegistrationRepository) RevokeLiveActivity(_ context.Context, owner, id, lease string, event audit.Event) error {
	if value, ok := r.registrations[id]; ok && value.OwnerUserID == owner && value.AccountLease == lease {
		delete(r.registrations, id)
	}
	r.events = append(r.events, event)
	return nil
}
func TestLiveActivityRegistrationUsesAuthenticatedOwnerAndAuditsWithoutToken(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	repository := &activityRegistrationRepository{registrations: map[string]liveactivity.Registration{}}
	application := App{Auth: activityRegistrationAuth{}, Clock: controlledPushClock{now}, AuditRateLimiter: controlledPushLimiter{allowed: true}, LiveActivities: repository, Audits: &controlledPushAudits{}}
	input := liveactivity.Registration{ID: "activity", OwnerUserID: "other-account", AccountLease: "lease", Token: "abab", ExpiresAt: now.Add(time.Hour), Boundaries: []time.Time{now.Add(30 * time.Minute)}}
	if err := application.UpsertLiveActivity(context.Background(), "Bearer alice", input); err != nil {
		t.Fatal(err)
	}
	if repository.registrations["activity"].OwnerUserID != "alice" {
		t.Fatal("request selected another owner")
	}
	if len(repository.events) != 1 || repository.events[0].TargetType != "live_activity_registration" {
		t.Fatal("missing registration audit")
	}
	if err := application.RevokeLiveActivity(context.Background(), "Bearer alice", "activity", "lease"); err != nil {
		t.Fatal(err)
	}
	if len(repository.registrations) != 0 {
		t.Fatal("registration remains after revoke")
	}
	before := len(repository.events)
	if err := application.UpsertLiveActivity(context.Background(), "missing", input); err == nil {
		t.Fatal("unauthenticated registration accepted")
	}
	if len(repository.events) != before {
		t.Fatal("unauthenticated mutation reached storage")
	}
}
