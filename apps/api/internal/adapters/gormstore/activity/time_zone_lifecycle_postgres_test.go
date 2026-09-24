package activitystore

import (
	"bytes"
	"context"
	"testing"
	"time"

	rootstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	accountapp "github.com/elsell/hour-paths/apps/api/internal/app"
	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	activitydomain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresConfiguredTimeZoneChangeRetainsRunningTimerZoneAndAppliesToNextStart(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "time-zone-timer-participant", "time-zone-timer-path"
	seedParticipantAndPath(t, migrationDB, participantID, pathID, now)
	if err := migrationDB.Table("user_preference_models").Create(map[string]any{"user_id": participantID, "first_day_of_week": 1, "current_time_zone": "America/New_York", "created_at": now.Add(-time.Hour), "updated_at": now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("user_time_zone_history_models").Create(map[string]any{"user_id": participantID, "effective_at": now.Add(-time.Hour), "time_zone": "America/New_York"}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationDB.Table("user_models").Where("id = ?", participantID).Delete(&struct{ ID string }{})
	})

	preferences := &rootstore.Store{DB: runtimeDB}
	activities := New(runtimeDB)
	oldPreference, err := preferences.GetTimeZonePreference(context.Background(), participantID)
	if err != nil {
		t.Fatal(err)
	}
	oldTimer, err := activitydomain.StartTimer("time-zone-old-timer", pathID, participantID, now, oldPreference.TimeZone, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := activities.StartTimer(context.Background(), activityapp.StartTimerCommand{Timer: oldTimer, Idempotency: idempotency(participantID, activityapp.StartTimerOperation, "time-zone-old-start", 80), Audit: timerAudit("time-zone-old-start-audit", participantID, oldTimer.ID, audit.ActivityTimerStarted, now)}); err != nil {
		t.Fatal(err)
	}

	changeAt := now.Add(time.Minute)
	change := accountapp.TimeZonePreferenceCommand{
		ActorUserID: participantID, ReviewedTimeZone: "America/New_York", ProposedTimeZone: "Europe/Paris", ChangedAt: changeAt,
		Idempotency: ports.Idempotency{PrincipalID: participantID, Operation: accountapp.UpdateTimeZonePreferenceOperation, Key: "time-zone-change-01", RequestHash: bytes.Repeat([]byte{6}, 32)},
		Audit:       audit.Event{ID: "time-zone-change-audit", OwnerUserID: participantID, ActorUserID: participantID, Action: audit.ResourceUpdated, TargetType: "time_zone_preference", TargetID: participantID, Outcome: audit.Succeeded, CorrelationID: "time-zone-correlation", OccurredAt: changeAt},
	}
	if result, err := preferences.UpdateTimeZonePreference(context.Background(), change); err != nil || !result.Changed || result.Preference.TimeZone != "Europe/Paris" {
		t.Fatalf("change = %+v, %v", result, err)
	}
	stopped, err := activities.StopTimer(context.Background(), activityapp.StopTimerCommand{TimerID: oldTimer.ID, PathID: pathID, ParticipantID: participantID, ActivityID: "time-zone-old-activity", StoppedAt: changeAt.Add(time.Minute), RecordedAt: changeAt.Add(time.Minute), Idempotency: idempotency(participantID, activityapp.StopTimerOperation, "time-zone-old-stop1", 81), Audit: timerAudit("time-zone-old-stop-audit", participantID, oldTimer.ID, audit.ActivityTimerStopped, changeAt.Add(time.Minute))})
	if err != nil || !stopped.Saved || stopped.Activity.OccurrenceTimeZone != "America/New_York" {
		t.Fatalf("old stop = %+v, %v", stopped, err)
	}

	newPreference, err := preferences.GetTimeZonePreference(context.Background(), participantID)
	if err != nil || newPreference.TimeZone != "Europe/Paris" {
		t.Fatalf("new preference = %+v, %v", newPreference, err)
	}
	newTimer, err := activitydomain.StartTimer("time-zone-new-timer", pathID, participantID, changeAt.Add(2*time.Minute), newPreference.TimeZone, changeAt.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	started, err := activities.StartTimer(context.Background(), activityapp.StartTimerCommand{Timer: newTimer, Idempotency: idempotency(participantID, activityapp.StartTimerOperation, "time-zone-new-start", 82), Audit: timerAudit("time-zone-new-start-audit", participantID, newTimer.ID, audit.ActivityTimerStarted, changeAt.Add(2*time.Minute))})
	if err != nil || started.Timer.OccurrenceTimeZone != "Europe/Paris" {
		t.Fatalf("new start = %+v, %v", started, err)
	}
}
