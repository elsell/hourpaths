package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledTimeZonePreferences struct {
	current TimeZonePreference
	result  TimeZonePreferenceResult
	err     error
	gets    *[]string
	updates *[]TimeZonePreferenceCommand
}

func (f controlledTimeZonePreferences) GetTimeZonePreference(_ context.Context, userID string) (TimeZonePreference, error) {
	if f.gets != nil {
		*f.gets = append(*f.gets, userID)
	}
	return f.current, f.err
}

func (f controlledTimeZonePreferences) UpdateTimeZonePreference(_ context.Context, command TimeZonePreferenceCommand) (TimeZonePreferenceResult, error) {
	if f.updates != nil {
		*f.updates = append(*f.updates, command)
	}
	return f.result, f.err
}

func timeZoneTestApp(repository TimeZonePreferenceRepository) App {
	now := time.Date(2026, 8, 3, 14, 30, 0, 123456789, time.UTC)
	return App{
		Auth:                fakeAuth{principal: ports.Principal{UserID: "owner", Scopes: []string{"api:user"}}},
		Users:               fakeUsers{user: identity.User{ID: "owner", Status: identity.StatusActive}},
		TimeZonePreferences: repository,
		Audits:              fakeAudits{}, AuditRateLimiter: fakeAuditRateLimiter{}, Clock: fakeClock{now: now},
	}
}

func TestConfiguredTimeZoneIsAuthenticatedOwnerScopedAndAudited(t *testing.T) {
	effectiveAt := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	var gets []string
	var events []audit.Event
	application := timeZoneTestApp(controlledTimeZonePreferences{current: TimeZonePreference{TimeZone: "America/New_York", EffectiveAt: effectiveAt}, gets: &gets})
	application.Audits = fakeAudits{events: &events}

	preference, err := application.ConfiguredTimeZone(context.Background(), "Bearer session")
	if err != nil || preference != (TimeZonePreference{TimeZone: "America/New_York", EffectiveAt: effectiveAt}) {
		t.Fatalf("ConfiguredTimeZone() = (%+v, %v)", preference, err)
	}
	if len(gets) != 1 || gets[0] != "owner" {
		t.Fatalf("owner-scoped reads = %v", gets)
	}
	if len(events) != 1 || events[0].OwnerUserID != "owner" || events[0].ActorUserID != "owner" || events[0].Action != audit.ResourceViewed || events[0].TargetType != "time_zone_preference" {
		t.Fatalf("audit events = %+v", events)
	}
}

func TestConfirmedTimeZoneUpdateCarriesReviewedValueAtomicEvidenceAndStableHash(t *testing.T) {
	now := time.Date(2026, 8, 3, 14, 30, 0, 123456000, time.UTC)
	want := TimeZonePreferenceResult{Preference: TimeZonePreference{TimeZone: "Europe/Paris", EffectiveAt: now}, Changed: true}
	var first, second []TimeZonePreferenceCommand
	firstApp := timeZoneTestApp(controlledTimeZonePreferences{result: want, updates: &first})
	secondApp := timeZoneTestApp(controlledTimeZonePreferences{result: want, updates: &second})
	secondApp.Clock = fakeClock{now: now.Add(time.Hour)}

	input := TimeZonePreferenceUpdate{ReviewedTimeZone: "America/New_York", ProposedTimeZone: "Europe/Paris", Confirmed: true}
	result, err := firstApp.UpdateConfiguredTimeZone(context.Background(), "Bearer session", "time-zone-key-0001", input)
	if err != nil || result != want {
		t.Fatalf("UpdateConfiguredTimeZone() = (%+v, %v)", result, err)
	}
	_, _ = secondApp.UpdateConfiguredTimeZone(context.Background(), "Bearer session", "time-zone-key-0001", input)
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("commands = %d, %d", len(first), len(second))
	}
	command := first[0]
	if command.ActorUserID != "owner" || command.ReviewedTimeZone != "America/New_York" || command.ProposedTimeZone != "Europe/Paris" || command.ChangedAt != now || command.Idempotency.PrincipalID != "owner" || command.Idempotency.Operation != UpdateTimeZonePreferenceOperation || command.Idempotency.Key != "time-zone-key-0001" || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("command = %+v", command)
	}
	if !command.Audit.Valid() || command.Audit.Action != audit.ResourceUpdated || command.Audit.TargetType != "time_zone_preference" || command.Audit.TargetID != "owner" {
		t.Fatalf("audit = %+v", command.Audit)
	}
	if string(first[0].Idempotency.RequestHash) != string(second[0].Idempotency.RequestHash) {
		t.Fatal("same reviewed change did not retain a stable request hash")
	}
}

func TestTimeZonePreferenceRejectsInvalidOrUnconfirmedInputBeforePersistence(t *testing.T) {
	tests := map[string]TimeZonePreferenceUpdate{
		"not confirmed":    {ReviewedTimeZone: "America/New_York", ProposedTimeZone: "Europe/Paris"},
		"invalid reviewed": {ReviewedTimeZone: "UTC-04:00", ProposedTimeZone: "Europe/Paris", Confirmed: true},
		"invalid proposed": {ReviewedTimeZone: "America/New_York", ProposedTimeZone: "Local", Confirmed: true},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			var updates []TimeZonePreferenceCommand
			application := timeZoneTestApp(controlledTimeZonePreferences{updates: &updates})
			if _, err := application.UpdateConfiguredTimeZone(context.Background(), "Bearer session", "time-zone-key-0001", input); !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("error = %v", err)
			}
			if len(updates) != 0 {
				t.Fatal("invalid input reached persistence")
			}
		})
	}
}

func TestTimeZonePreferenceFailsClosedForWrongUserOrRepositoryFailure(t *testing.T) {
	application := timeZoneTestApp(controlledTimeZonePreferences{})
	application.Auth = fakeAuth{principal: ports.Principal{UserID: "other", Scopes: []string{"api:user"}}}
	application.Users = fakeUsers{err: ports.ErrNotFound}
	if _, err := application.ConfiguredTimeZone(context.Background(), "Bearer session"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("cross-user inactive principal error = %v", err)
	}

	databaseErr := errors.New("database unavailable")
	application = timeZoneTestApp(controlledTimeZonePreferences{err: databaseErr})
	if _, err := application.ConfiguredTimeZone(context.Background(), "Bearer session"); !errors.Is(err, databaseErr) {
		t.Fatalf("repository error = %v", err)
	}
}
