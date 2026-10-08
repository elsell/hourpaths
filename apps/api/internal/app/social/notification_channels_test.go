package social

import (
	"context"
	"errors"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/notification"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
)

type memoryNotificationChannels struct {
	values        map[string]NudgeNotificationChannelPreference
	reads, writes int
}

func (r *memoryNotificationChannels) GetNotificationChannel(_ context.Context, actor string, channel notification.Channel) (NudgeNotificationChannelPreference, bool, error) {
	r.reads++
	v, ok := r.values[actor+":"+string(channel)]
	return v, ok, nil
}
func (r *memoryNotificationChannels) UpdateNotificationChannel(_ context.Context, c NotificationChannelCommand) (NudgeNotificationChannelMutationResult, error) {
	key := c.ActorUserID + ":" + string(c.Channel)
	if r.values[key].Revision != c.ExpectedRevision {
		return NudgeNotificationChannelMutationResult{}, ports.ErrConflict
	}
	r.writes++
	v := NudgeNotificationChannelPreference{Enabled: c.Enabled, Revision: c.ExpectedRevision + 1}
	r.values[key] = v
	return NudgeNotificationChannelMutationResult{Preference: v}, nil
}
func TestNotificationChannelsPersistIndependentlyForAuthenticatedOwner(t *testing.T) {
	repo := &memoryNotificationChannels{values: map[string]NudgeNotificationChannelPreference{}}
	audits := &controlledAudits{}
	service := testService(&controlledProfiles{}, audits)
	service.NotificationChannels = repo
	ctx := context.Background()
	initial, err := service.ListNotificationChannels(ctx, "Bearer viewer")
	if err != nil || len(initial) != 10 {
		t.Fatalf("list=%+v err=%v", initial, err)
	}
	for _, p := range initial {
		if !p.Enabled || p.Revision != 0 {
			t.Fatalf("default %+v", p)
		}
	}
	changed, err := service.UpdateNotificationChannel(ctx, "Bearer viewer", "comments", "notification-setting-one", 0, false)
	if err != nil || changed.Enabled || changed.Revision != 1 {
		t.Fatalf("change %+v %v", changed, err)
	}
	current, err := service.ListNotificationChannels(ctx, "Bearer viewer")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range current {
		if p.Enabled != (p.Channel != "comments") {
			t.Fatalf("coupled channels: %+v", current)
		}
	}
	service.Auth = controlledAuth{principal: ports.Principal{UserID: "other", Scopes: []string{"api:user"}}}
	other, err := service.ListNotificationChannels(ctx, "Bearer other")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range other {
		if !p.Enabled || p.Revision != 0 {
			t.Fatalf("cross-owner leak %+v", other)
		}
	}
	if len(audits.events) != 3 {
		t.Fatalf("read audit count %d", len(audits.events))
	}
}
func TestNotificationChannelsRejectInvalidAuthenticationBeforePersistence(t *testing.T) {
	for _, auth := range []controlledAuth{{err: platformapp.ErrUnauthenticated}, {principal: ports.Principal{UserID: "viewer", Scopes: []string{"api:onboarding"}}}, {principal: ports.Principal{UserID: "viewer", Scopes: []string{"api:user", "api:onboarding"}}}} {
		repo := &memoryNotificationChannels{values: map[string]NudgeNotificationChannelPreference{}}
		service := testService(&controlledProfiles{}, &controlledAudits{})
		service.NotificationChannels = repo
		service.Auth = auth
		if _, err := service.ListNotificationChannels(context.Background(), "bad"); !errors.Is(err, platformapp.ErrUnauthenticated) {
			t.Fatalf("list auth %v", err)
		}
		if _, err := service.UpdateNotificationChannel(context.Background(), "bad", "comments", "notification-setting-auth", 0, false); !errors.Is(err, platformapp.ErrUnauthenticated) {
			t.Fatalf("write auth %v", err)
		}
		if repo.reads != 0 || repo.writes != 0 {
			t.Fatal("unauthorized persistence")
		}
	}
}
func TestNotificationChannelsRateLimitAndAuditFailureFailClosed(t *testing.T) {
	repo := &memoryNotificationChannels{values: map[string]NudgeNotificationChannelPreference{}}
	audits := &controlledAudits{err: ports.ErrUnavailable}
	service := testService(&controlledProfiles{}, audits)
	service.NotificationChannels = repo
	if _, err := service.ListNotificationChannels(context.Background(), "viewer"); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("audit failure %v", err)
	}
	service.AuditRateLimiter = allowLimiter{allow: false}
	if _, err := service.UpdateNotificationChannel(context.Background(), "viewer", "comments", "notification-setting-limit", 0, false); !errors.Is(err, platformapp.ErrRateLimited) {
		t.Fatalf("rate limit %v", err)
	}
	if repo.writes != 0 {
		t.Fatal("rate-limited write persisted")
	}
}
