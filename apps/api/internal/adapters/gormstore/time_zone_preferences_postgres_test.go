package gormstore

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresTimeZonePreferenceUpdateIsAtomicIdempotentAndConflictSafe(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeStore, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := "time-zone-preference-" + newTestID()
	seedTimeZonePreferenceUser(t, migrationStore, userID, "America/New_York", now.Add(-time.Hour))
	t.Cleanup(func() { migrationStore.DB.Table("user_models").Where("id = ?", userID).Delete(&struct{ ID string }{}) })

	initial, err := runtimeStore.GetTimeZonePreference(ctx, userID)
	if err != nil || initial.TimeZone != "America/New_York" || initial.EffectiveAt != now.Add(-time.Hour) {
		t.Fatalf("initial preference = %+v, %v", initial, err)
	}
	staleInstant := timeZonePreferenceCommand(userID, "America/New_York", "Europe/Paris", initial.EffectiveAt, "time-zone-stale-001", "time-zone-stale-audit")
	if _, err := runtimeStore.UpdateTimeZonePreference(ctx, staleInstant); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("non-advancing effective instant error = %v", err)
	}
	assertTimeZonePreferenceState(t, migrationStore, userID, "America/New_York", 1, 0, 0)
	command := timeZonePreferenceCommand(userID, "America/New_York", "Europe/Paris", now, "time-zone-key-0001", "time-zone-audit-1")
	changed, err := runtimeStore.UpdateTimeZonePreference(ctx, command)
	if err != nil || changed.Preference != (application.TimeZonePreference{TimeZone: "Europe/Paris", EffectiveAt: now}) || !changed.Changed || changed.Replayed {
		t.Fatalf("changed preference = %+v, %v", changed, err)
	}
	replayed, err := runtimeStore.UpdateTimeZonePreference(ctx, command)
	if err != nil || replayed.Preference != changed.Preference || !replayed.Changed || !replayed.Replayed {
		t.Fatalf("replayed preference = %+v, %v", replayed, err)
	}

	conflictingKey := command
	conflictingKey.Idempotency.RequestHash = bytes.Repeat([]byte{9}, 32)
	if _, err := runtimeStore.UpdateTimeZonePreference(ctx, conflictingKey); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict error = %v", err)
	}
	stale := timeZonePreferenceCommand(userID, "America/New_York", "Asia/Tokyo", now.Add(time.Minute), "time-zone-key-0002", "time-zone-audit-2")
	if _, err := runtimeStore.UpdateTimeZonePreference(ctx, stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale review error = %v", err)
	}
	assertTimeZonePreferenceState(t, migrationStore, userID, "Europe/Paris", 2, 1, 1)
	if err := migrationStore.DB.Table("user_time_zone_history_models").Create(map[string]any{"user_id": userID, "effective_at": now.Add(2 * time.Minute), "time_zone": "Asia/Tokyo"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeStore.GetTimeZonePreference(ctx, userID); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("inconsistent latest history error = %v", err)
	}
}

func TestPostgresTimeZonePreferenceNoOpAndAuditFailureRollback(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeStore, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := "time-zone-noop-" + newTestID()
	initialAt := now.Add(-time.Hour)
	seedTimeZonePreferenceUser(t, migrationStore, userID, "Etc/UTC", initialAt)
	t.Cleanup(func() { migrationStore.DB.Table("user_models").Where("id = ?", userID).Delete(&struct{ ID string }{}) })

	noop := timeZonePreferenceCommand(userID, "Etc/UTC", "Etc/UTC", now, "time-zone-noop-0001", "time-zone-noop-audit")
	result, err := runtimeStore.UpdateTimeZonePreference(context.Background(), noop)
	if err != nil || result.Preference != (application.TimeZonePreference{TimeZone: "Etc/UTC", EffectiveAt: initialAt}) || result.Changed {
		t.Fatalf("no-op result = %+v, %v", result, err)
	}
	assertTimeZonePreferenceState(t, migrationStore, userID, "Etc/UTC", 1, 1, 1)

	duplicateAudit := auditEventModel{ID: "time-zone-duplicate-audit-" + newTestID(), OwnerUserID: userID, ActorUserID: userID, Action: audit.ResourceViewed, TargetType: "user", TargetID: userID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
	if err := migrationStore.DB.Create(&duplicateAudit).Error; err != nil {
		t.Fatal(err)
	}
	failed := timeZonePreferenceCommand(userID, "Etc/UTC", "Asia/Tokyo", now.Add(time.Minute), "time-zone-rollback1", duplicateAudit.ID)
	if _, err := runtimeStore.UpdateTimeZonePreference(context.Background(), failed); err == nil {
		t.Fatal("duplicate audit identity did not fail update")
	}
	assertTimeZonePreferenceState(t, migrationStore, userID, "Etc/UTC", 1, 1, 1)
	failed.Audit.ID = "time-zone-retry-audit-" + newTestID()
	if result, err := runtimeStore.UpdateTimeZonePreference(context.Background(), failed); err != nil || !result.Changed || result.Preference.TimeZone != "Asia/Tokyo" {
		t.Fatalf("retry after rollback = %+v, %v", result, err)
	}
}

func seedTimeZonePreferenceUser(t *testing.T, store *Store, userID, zone string, at time.Time) {
	t.Helper()
	user := userModel{ID: userID, Email: userID + "@example.com", ProviderEmailVerified: true, DisplayName: "Zone User", Status: identity.StatusActive, CreatedAt: at, UpdatedAt: at}
	if err := store.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Table("user_preference_models").Create(map[string]any{"user_id": userID, "first_day_of_week": 1, "current_time_zone": zone, "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Table("user_time_zone_history_models").Create(map[string]any{"user_id": userID, "effective_at": at, "time_zone": zone}).Error; err != nil {
		t.Fatal(err)
	}
}

func timeZonePreferenceCommand(userID, reviewed, proposed string, at time.Time, key, auditID string) application.TimeZonePreferenceCommand {
	hash := bytes.Repeat([]byte{7}, 32)
	return application.TimeZonePreferenceCommand{
		ActorUserID: userID, ReviewedTimeZone: reviewed, ProposedTimeZone: proposed, ChangedAt: at,
		Idempotency: ports.Idempotency{PrincipalID: userID, Operation: application.UpdateTimeZonePreferenceOperation, Key: key, RequestHash: hash},
		Audit:       audit.Event{ID: auditID, OwnerUserID: userID, ActorUserID: userID, Action: audit.ResourceUpdated, TargetType: "time_zone_preference", TargetID: userID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at},
	}
}

func assertTimeZonePreferenceState(t *testing.T, store *Store, userID, zone string, history, mutations, audits int64) {
	t.Helper()
	var persisted struct{ CurrentTimeZone string }
	if err := store.DB.Table("user_preference_models").Where("user_id = ?", userID).Take(&persisted).Error; err != nil || persisted.CurrentTimeZone != zone {
		t.Fatalf("persisted preference = %+v, %v", persisted, err)
	}
	for table, want := range map[string]int64{"user_time_zone_history_models": history, "user_time_zone_preference_mutation_models": mutations} {
		var count int64
		if err := store.DB.Table(table).Where("user_id = ?", userID).Count(&count).Error; err != nil || count != want {
			t.Fatalf("%s count = %d, %v; want %d", table, count, err, want)
		}
	}
	var auditCount int64
	if err := store.DB.Table("audit_event_models").Where("owner_user_id = ? AND target_type = 'time_zone_preference'", userID).Count(&auditCount).Error; err != nil || auditCount != audits {
		t.Fatalf("audit count = %d, %v; want %d", auditCount, err, audits)
	}
}
