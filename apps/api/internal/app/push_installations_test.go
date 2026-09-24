package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledPushInstallationRepository struct {
	upserts []ports.PushInstallation
	deletes []string
	events  []audit.Event
	err     error
}

func (repository *controlledPushInstallationRepository) UpsertPushInstallation(
	_ context.Context,
	installation ports.PushInstallation,
	event audit.Event,
) error {
	repository.upserts = append(repository.upserts, installation)
	repository.events = append(repository.events, event)
	return repository.err
}

func (repository *controlledPushInstallationRepository) DeletePushInstallation(
	_ context.Context,
	ownerUserID, installationID string,
	changedAt time.Time,
	event audit.Event,
) error {
	repository.deletes = append(repository.deletes, ownerUserID+"|"+installationID+"|"+changedAt.Format(time.RFC3339Nano))
	repository.events = append(repository.events, event)
	return repository.err
}

type controlledPushAuthenticator struct {
	principal ports.Principal
	err       error
}

func (auth controlledPushAuthenticator) Authenticate(context.Context, string) (ports.Principal, error) {
	return auth.principal, auth.err
}

type controlledPushAudits struct{ events []audit.Event }

func (audits *controlledPushAudits) AppendAuditEvent(_ context.Context, event audit.Event) error {
	audits.events = append(audits.events, event)
	return nil
}

func (*controlledPushAudits) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}

type controlledPushClock struct{ now time.Time }

func (clock controlledPushClock) Now() time.Time { return clock.now }

type controlledPushLimiter struct{ allowed bool }

func (limiter controlledPushLimiter) Allow(string, time.Time) bool { return limiter.allowed }

func TestPushInstallationRegistrationIsAuthenticatedValidatedAndAtomicallyAudited(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 30, 0, 0, time.UTC)
	repository := &controlledPushInstallationRepository{}
	application := App{
		Auth:              controlledPushAuthenticator{principal: ports.Principal{UserID: "user-1", Scopes: []string{"api:user"}}},
		PushInstallations: repository,
		AuditRateLimiter:  controlledPushLimiter{allowed: true},
		Clock:             controlledPushClock{now: now},
	}

	err := application.UpsertPushInstallation(
		WithCorrelationID(context.Background(), "request-1"),
		"Bearer application",
		"installation-1",
		"expo",
		"ios",
		"es",
		"ExponentPushToken[opaque-device-token]",
	)
	if err != nil {
		t.Fatalf("UpsertPushInstallation() error = %v", err)
	}
	if len(repository.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(repository.upserts))
	}
	installation := repository.upserts[0]
	if installation.ID != "installation-1" || installation.OwnerUserID != "user-1" ||
		installation.Provider != "expo" || installation.Platform != "ios" ||
		installation.Locale != "es" || installation.Token != "ExponentPushToken[opaque-device-token]" ||
		!installation.CreatedAt.Equal(now) || !installation.UpdatedAt.Equal(now) {
		t.Fatalf("installation = %+v", installation)
	}
	if len(repository.events) != 1 {
		t.Fatalf("repository audit events = %d, want 1", len(repository.events))
	}
	event := repository.events[0]
	if event.OwnerUserID != "user-1" || event.ActorUserID != "user-1" ||
		event.Action != audit.ResourceUpdated || event.TargetType != "push_installation" ||
		event.TargetID != "installation-1" || event.Outcome != audit.Succeeded ||
		event.CorrelationID != "request-1" || !event.OccurredAt.Equal(now) {
		t.Fatalf("audit event = %+v", event)
	}
}

func TestPushInstallationDeletionIsRecipientScopedAndAuditsOpaqueMiss(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 31, 0, 0, time.UTC)
	repository := &controlledPushInstallationRepository{err: ports.ErrNotFound}
	audits := &controlledPushAudits{}
	application := App{
		Auth:              controlledPushAuthenticator{principal: ports.Principal{UserID: "user-1", Scopes: []string{"api:user"}}},
		PushInstallations: repository,
		Audits:            audits,
		AuditRateLimiter:  controlledPushLimiter{allowed: true},
		Clock:             controlledPushClock{now: now},
	}

	err := application.DeletePushInstallation(
		WithCorrelationID(context.Background(), "request-2"),
		"Bearer application",
		"installation-foreign",
	)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeletePushInstallation() error = %v, want opaque not found", err)
	}
	if len(audits.events) != 1 {
		t.Fatalf("denied audit events = %d, want 1", len(audits.events))
	}
	event := audits.events[0]
	if event.Action != audit.ResourceAccessDenied || event.Outcome != audit.Denied ||
		event.OwnerUserID != "user-1" || event.ActorUserID != "user-1" ||
		event.TargetType != "push_installation" || event.TargetID != "installation-foreign" {
		t.Fatalf("denied audit event = %+v", event)
	}
}

func TestPushInstallationRegistrationRejectsInvalidOrNonUserInputsBeforePersistence(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 32, 0, 0, time.UTC)
	for name, test := range map[string]struct {
		principal                             ports.Principal
		id, provider, platform, locale, token string
		want                                  error
	}{
		"wrong scope": {
			principal: ports.Principal{UserID: "user-1", Scopes: []string{"api:onboarding"}},
			id:        "installation-1", provider: "expo", platform: "ios", locale: "en",
			token: "ExponentPushToken[opaque-device-token]", want: ErrUnauthenticated,
		},
		"unsupported provider": {
			principal: ports.Principal{UserID: "user-1", Scopes: []string{"api:user"}},
			id:        "installation-1", provider: "unknown", platform: "ios", locale: "en",
			token: "ExponentPushToken[opaque-device-token]", want: ports.ErrInvalidArgument,
		},
		"invalid platform": {
			principal: ports.Principal{UserID: "user-1", Scopes: []string{"api:user"}},
			id:        "installation-1", provider: "expo", platform: "web", locale: "en",
			token: "ExponentPushToken[opaque-device-token]", want: ports.ErrInvalidArgument,
		},
		"invalid locale": {
			principal: ports.Principal{UserID: "user-1", Scopes: []string{"api:user"}},
			id:        "installation-1", provider: "expo", platform: "ios", locale: "fr",
			token: "ExponentPushToken[opaque-device-token]", want: ports.ErrInvalidArgument,
		},
		"invalid token": {
			principal: ports.Principal{UserID: "user-1", Scopes: []string{"api:user"}},
			id:        "installation-1", provider: "expo", platform: "ios", locale: "en",
			token: "private-device-token", want: ports.ErrInvalidArgument,
		},
	} {
		t.Run(name, func(t *testing.T) {
			repository := &controlledPushInstallationRepository{}
			application := App{
				Auth:              controlledPushAuthenticator{principal: test.principal},
				PushInstallations: repository, AuditRateLimiter: controlledPushLimiter{allowed: true},
				Clock: controlledPushClock{now: now},
			}
			err := application.UpsertPushInstallation(
				context.Background(), "Bearer application",
				test.id, test.provider, test.platform, test.locale, test.token,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("UpsertPushInstallation() error = %v, want %v", err, test.want)
			}
			if len(repository.upserts) != 0 {
				t.Fatalf("invalid registration persisted %d times", len(repository.upserts))
			}
		})
	}
}

func TestPushInstallationRegistrationIsRateLimitedBeforePersistence(t *testing.T) {
	repository := &controlledPushInstallationRepository{}
	application := App{
		Auth: controlledPushAuthenticator{principal: ports.Principal{
			UserID: "user-1", Scopes: []string{"api:user"},
		}},
		PushInstallations: repository,
		AuditRateLimiter:  controlledPushLimiter{},
		Clock: controlledPushClock{
			now: time.Date(2026, 7, 24, 0, 33, 0, 0, time.UTC),
		},
	}
	err := application.UpsertPushInstallation(
		context.Background(), "Bearer application", "installation-1",
		"expo", "ios", "en", "ExpoPushToken[opaque-device-token]",
	)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("UpsertPushInstallation() error = %v, want rate limited", err)
	}
	if len(repository.upserts) != 0 {
		t.Fatalf("rate-limited registration persisted %d times", len(repository.upserts))
	}
}
