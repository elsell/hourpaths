package activitystore

import (
	"bytes"
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func offlineCommand(t *testing.T, timerID, kind string, start, end, now time.Time, hash byte) application.OfflineTimerCommand {
	t.Helper()
	timer, err := domain.StartTimer(timerID, "offline-path", "offline-owner", start, "America/New_York", now)
	if err != nil {
		t.Fatal(err)
	}
	action := audit.ActivityTimerStarted
	var ended *time.Time
	if kind == "stop" {
		action = audit.ActivityTimerStopped
		ended = &end
	}
	return application.OfflineTimerCommand{Timer: timer, Kind: kind, EndedAt: ended, RecordedAt: now, ActivityID: "entry-" + timerID,
		IdentityHash: application.OfflineTimerIdentityHash(timer), Idempotency: idempotency(timer.ParticipantID, application.OfflineTimerOperation, "offline-operation-"+timerID+"-"+kind, hash), Audit: timerAudit("audit-"+timerID+"-"+kind, timer.ParticipantID, timerID, action, now)}
}

func TestPostgresOfflineReplaySurvivesLostAcknowledgementAndNeverRestartsTerminalTimer(t *testing.T) {
	db := offlinePostgresDB(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	start := offlineCommand(t, "first", "start", now.Add(-time.Hour), time.Time{}, now, 1)
	first, err := repo.SynchronizeTimer(context.Background(), start)
	if err != nil || first.Timer == nil || first.Outcome != "accepted" || !first.Timer.StartedAt.Equal(start.Timer.StartedAt) {
		t.Fatalf("start=%+v err=%v", first, err)
	}
	stop := offlineCommand(t, "first", "stop", start.Timer.StartedAt, now.Add(-30*time.Minute), now, 2)
	saved, err := repo.SynchronizeTimer(context.Background(), stop)
	if err != nil || saved.Activity == nil || saved.SavedSeconds != 1800 {
		t.Fatalf("stop=%+v err=%v", saved, err)
	}
	for _, command := range []application.OfflineTimerCommand{stop, start, stop} {
		result, err := repo.SynchronizeTimer(context.Background(), command)
		if err != nil || !result.Replayed || result.Timer != nil {
			t.Fatalf("replay=%+v err=%v", result, err)
		}
	}
	var count int64
	if err := db.Model(&activityModel{}).Where("participant_id = ?", "offline-owner").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("activities=%d err=%v", count, err)
	}
	stop.Idempotency.RequestHash = bytes.Repeat([]byte{9}, 32)
	if _, err := repo.SynchronizeTimer(context.Background(), stop); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("changed payload=%v", err)
	}
}

func TestPostgresOfflineConflictLoserCannotReturnAfterWinnerStops(t *testing.T) {
	db := offlinePostgresDB(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	older := offlineCommand(t, "older", "start", now.Add(-time.Hour), time.Time{}, now, 1)
	newer := offlineCommand(t, "newer", "start", now.Add(-30*time.Minute), time.Time{}, now, 2)
	for _, command := range []application.OfflineTimerCommand{older, newer} {
		if _, err := repo.SynchronizeTimer(context.Background(), command); err != nil {
			t.Fatal(err)
		}
	}
	loser, err := repo.SynchronizeTimer(context.Background(), older)
	if err != nil || loser.Outcome != "conflict" {
		t.Fatalf("loser=%+v err=%v", loser, err)
	}
	stop := offlineCommand(t, "newer", "stop", newer.Timer.StartedAt, now, now, 3)
	if _, err := repo.SynchronizeTimer(context.Background(), stop); err != nil {
		t.Fatal(err)
	}
	late := offlineCommand(t, "older", "stop", older.Timer.StartedAt, now, now, 4)
	rejected, err := repo.SynchronizeTimer(context.Background(), late)
	if err != nil || rejected.Outcome != "conflict" || rejected.Activity != nil {
		t.Fatalf("late=%+v err=%v", rejected, err)
	}
	var count int64
	db.Model(&timerModel{}).Where("participant_id = ?", "offline-owner").Count(&count)
	if count != 0 {
		t.Fatal("discarded timer restarted")
	}
	db.Model(&activityModel{}).Where("participant_id = ?", "offline-owner").Count(&count)
	if count != 1 {
		t.Fatalf("saved losing time: entries=%d", count)
	}
}

// Install the pending schema transactionally so integration tests do not upgrade
// the shared development database or change its migration version.
func offlinePostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := postgresDB(t, true)
	var present bool
	if err := db.Raw("SELECT to_regclass('public.offline_timer_state_models') IS NOT NULL").Scan(&present).Error; err != nil {
		t.Fatal(err)
	}
	if !present {
		sql, err := dbmigrations.Files.ReadFile("000069_offline_timer_replay.up.sql")
		if err != nil {
			t.Fatal(err)
		}
		statement := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(sql)), "BEGIN;"), "COMMIT;")
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestPostgresOfflineArchiveSplitAndMembershipLoss(t *testing.T) {
	db := offlinePostgresDB(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
	archived := now.Add(-30 * time.Minute)
	if err := db.Table("path_models").Where("id = ?", "offline-path").Update("archived_at", archived).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	start := offlineCommand(t, "archived", "start", now.Add(-time.Hour), time.Time{}, now, 1)
	if result, err := repo.SynchronizeTimer(context.Background(), start); err != nil || result.Timer != nil {
		t.Fatalf("archived start=%+v err=%v", result, err)
	}
	stop := offlineCommand(t, "archived", "stop", start.Timer.StartedAt, now, now, 2)
	result, err := repo.SynchronizeTimer(context.Background(), stop)
	if err != nil || result.Outcome != "archived" || result.SavedSeconds != 1800 || result.DiscardedSeconds != 1800 || result.Activity == nil || !result.Activity.EndedAt.Equal(archived) {
		t.Fatalf("split=%+v err=%v", result, err)
	}
	if err := db.Exec("RESET ROLE").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_membership_models").Where("path_id = ? AND user_id = ?", "offline-path", "offline-owner").Delete(&struct{}{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SynchronizeTimer(context.Background(), stop); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("replay ignored current membership: %v", err)
	}
}

func TestPostgresOfflineAuditFailureRollsBackReplayAndTimer(t *testing.T) {
	db := offlinePostgresDB(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
	start := offlineCommand(t, "atomic", "start", now.Add(-time.Hour), time.Time{}, now, 1)
	if err := db.Create(fromAudit(start.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	if _, err := repo.SynchronizeTimer(context.Background(), start); err == nil {
		t.Fatal("audit failure accepted mutation")
	}
	for _, table := range []string{"running_timer_models", "offline_timer_state_models", "offline_timer_replay_models"} {
		var count int64
		if err := db.Table(table).Where("participant_id = ?", "offline-owner").Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s retained %d rows: %v", table, count, err)
		}
	}
	start.Audit.ID = "audit-retry-atomic"
	if _, err := repo.SynchronizeTimer(context.Background(), start); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresOfflineReplayAfterAnotherDeviceStopsDoesNotDuplicateActivity(t *testing.T) {
	db := offlinePostgresDB(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	start := offlineCommand(t, "remote", "start", now.Add(-time.Hour), time.Time{}, now, 1)
	running, err := repo.SynchronizeTimer(context.Background(), start)
	if err != nil || running.Timer == nil {
		t.Fatalf("start=%+v err=%v", running, err)
	}
	online := application.StopTimerCommand{TimerID: running.Timer.ID, PathID: "offline-path", ParticipantID: "offline-owner", ActivityID: "remote-stop-entry", StoppedAt: now.Add(-time.Minute), RecordedAt: now, Idempotency: idempotency("offline-owner", application.StopTimerOperation, "online-stop-key", 2), Audit: timerAudit("online-stop-audit", "offline-owner", running.Timer.ID, audit.ActivityTimerStopped, now)}
	if _, err := repo.StopTimer(context.Background(), online); err != nil {
		t.Fatal(err)
	}
	delayed := offlineCommand(t, "remote", "stop", start.Timer.StartedAt, now, now, 3)
	result, err := repo.SynchronizeTimer(context.Background(), delayed)
	if err != nil || result.Activity == nil || result.Activity.ID != online.ActivityID {
		t.Fatalf("delayed=%+v err=%v", result, err)
	}
	var count int64
	if err := db.Model(&activityModel{}).Where("participant_id = ?", "offline-owner").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("duplicate stop count=%d err=%v", count, err)
	}
}

func TestPostgresOfflineStopBeforeArchiveReconcilesServerArchiveEntry(t *testing.T) {
	db := offlinePostgresDB(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	start := offlineCommand(t, "archive-known", "start", now.Add(-time.Hour), time.Time{}, now.Add(-40*time.Minute), 1)
	if _, err := repo.SynchronizeTimer(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	archive := now.Add(-30 * time.Minute)
	if err := repo.StopPathTimersForArchive(context.Background(), "offline-path", archive, func() string { return "archive-entry" }); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_models").Where("id = ?", "offline-path").Update("archived_at", archive).Error; err != nil {
		t.Fatal(err)
	}
	stop := offlineCommand(t, "archive-known", "stop", start.Timer.StartedAt, now.Add(-45*time.Minute), now, 2)
	result, err := repo.SynchronizeTimer(context.Background(), stop)
	if err != nil || result.Activity == nil || result.Activity.ID != "archive-entry" || result.SavedSeconds != 900 || result.DiscardedSeconds != 0 {
		t.Fatalf("reconciled=%+v err=%v", result, err)
	}
	var count int64
	if err := db.Model(&activityModel{}).Where("participant_id = ?", "offline-owner").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("archive duplicate=%d err=%v", count, err)
	}
	assertPracticeFeedEvent(t, db, *result.Activity, 1)
	if err := db.Model(&activityRevisionModel{}).Where("activity_id = ?", "archive-entry").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("missing archive correction history=%d err=%v", count, err)
	}
}

func TestPostgresOnlineTimerCanStopOfflineUsingItsVisibleIdentity(t *testing.T) {
	db := offlinePostgresDB(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	timer, err := domain.StartTimer("online-visible-timer", "offline-path", "offline-owner", now.Add(-time.Hour), "America/New_York", now)
	if err != nil {
		t.Fatal(err)
	}
	start := application.StartTimerCommand{Timer: timer, Idempotency: idempotency(timer.ParticipantID, application.StartTimerOperation, "online-start-operation", 1), Audit: timerAudit("online-start-audit", timer.ParticipantID, timer.ID, audit.ActivityTimerStarted, now)}
	if _, err := repo.StartTimer(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	stop := offlineCommand(t, timer.ID, "stop", timer.StartedAt, now, now, 2)
	saved, err := repo.SynchronizeTimer(context.Background(), stop)
	if err != nil || saved.SavedSeconds != 3600 {
		t.Fatalf("offline stop=%+v err=%v", saved, err)
	}
	var count int64
	if err := db.Model(&timerModel{}).Where("participant_id = ?", timer.ParticipantID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("original online timer still running: %d %v", count, err)
	}
}

func TestPostgresOfflineCanonicalAliasPreservesIdentityAndRejectsChangedOccurrence(t *testing.T) {
	db := offlinePostgresDB(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	start := offlineCommand(t, "client-original", "start", now.Add(-time.Hour), time.Time{}, now, 1)
	result, err := repo.SynchronizeTimer(context.Background(), start)
	if err != nil || result.Timer == nil {
		t.Fatalf("start=%+v err=%v", result, err)
	}
	canonical := result.Timer.ID
	changed := offlineCommand(t, canonical, "stop", start.Timer.StartedAt.Add(time.Minute), now, now, 2)
	if _, err := repo.SynchronizeTimer(context.Background(), changed); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("alias permitted changed occurrence: %v", err)
	}
	stop := offlineCommand(t, canonical, "stop", start.Timer.StartedAt, now, now, 3)
	saved, err := repo.SynchronizeTimer(context.Background(), stop)
	if err != nil || saved.SavedSeconds != 3600 {
		t.Fatalf("canonical stop=%+v err=%v", saved, err)
	}
	replay, err := repo.SynchronizeTimer(context.Background(), stop)
	if err != nil || !replay.Replayed || replay.SavedSeconds != 3600 {
		t.Fatalf("alias replay=%+v err=%v", replay, err)
	}
	original := offlineCommand(t, "client-original", "stop", start.Timer.StartedAt, now, now, 4)
	same, err := repo.SynchronizeTimer(context.Background(), original)
	if err != nil || same.Activity == nil || same.Activity.ID != saved.Activity.ID {
		t.Fatalf("original identity duplicated: %+v %v", same, err)
	}
}

func TestPostgresOfflineRecognizesCompletedPreUpgradeTimer(t *testing.T) {
	for _, scenario := range []string{"saved", "deleted", "subsecond"} {
		t.Run(scenario, func(t *testing.T) {
			db := offlinePostgresDB(t)
			now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
			seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
			if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
				t.Fatal(err)
			}
			repo := New(db)
			timer, err := domain.StartTimer("pre-upgrade", "offline-path", "offline-owner", now.Add(-time.Hour), "America/New_York", now)
			if err != nil {
				t.Fatal(err)
			}
			_, err = repo.StartTimer(context.Background(), application.StartTimerCommand{Timer: timer,
				Idempotency: idempotency("offline-owner", application.StartTimerOperation, "legacy-start-key", 1),
				Audit:       timerAudit("legacy-start-audit", "offline-owner", timer.ID, audit.ActivityTimerStarted, now)})
			if err != nil {
				t.Fatal(err)
			}
			end := now.Add(-time.Minute)
			if scenario == "subsecond" {
				end = timer.StartedAt.Add(500 * time.Millisecond)
			}
			saved, err := repo.StopTimer(context.Background(), application.StopTimerCommand{TimerID: timer.ID, PathID: timer.PathID, ParticipantID: timer.ParticipantID,
				ActivityID: "legacy-entry", StoppedAt: end, RecordedAt: now,
				Idempotency: idempotency("offline-owner", application.StopTimerOperation, "legacy-stop-key", 2),
				Audit:       timerAudit("legacy-stop-audit", "offline-owner", timer.ID, audit.ActivityTimerStopped, now)})
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "deleted" {
				if err := db.Where("participant_id = ? AND id = ?", timer.ParticipantID, saved.Activity.ID).Delete(&activityModel{}).Error; err != nil {
					t.Fatal(err)
				}
			}
			// Before schema 69 these same authoritative mutation rows existed without
			// an offline registry. Remove only that registry as the migration owner.
			if err := db.Exec("RESET ROLE").Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Where("participant_id = ?", timer.ParticipantID).Delete(&offlineTimerState{}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
				t.Fatal(err)
			}
			for _, kind := range []string{"start", "stop"} {
				result, err := repo.SynchronizeTimer(context.Background(), offlineCommand(t, timer.ID, kind, timer.StartedAt, now, now, 3))
				if err != nil || !result.Terminal || result.Timer != nil {
					t.Fatalf("%s resurrected: %+v %v", kind, result, err)
				}
				if scenario == "saved" {
					if result.Activity == nil || result.Activity.ID != saved.Activity.ID {
						t.Fatalf("duplicated original entry: %+v", result)
					}
				} else if result.Activity != nil {
					t.Fatalf("recreated %s entry: %+v", scenario, result)
				}
			}
			changed := offlineCommand(t, timer.ID, "stop", timer.StartedAt.Add(time.Minute), now, now, 4)
			changed.Idempotency.Key = "changed-legacy-occurrence"
			if _, err := repo.SynchronizeTimer(context.Background(), changed); !errors.Is(err, ports.ErrIdempotencyConflict) {
				t.Fatalf("changed occurrence: %v", err)
			}
		})
	}
}

func TestPostgresOfflineArchiveDecisionSurvivesPathRestoration(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(map[bool]string{false: "split", true: "rejected"}[rejected], func(t *testing.T) {
			db := offlinePostgresDB(t)
			now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
			seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
			archived := now.Add(-30 * time.Minute)
			if err := db.Table("path_models").Where("id = ?", "offline-path").Update("archived_at", archived).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
				t.Fatal(err)
			}
			repo := New(db)
			started := now.Add(-time.Hour)
			if rejected {
				started = now.Add(-15 * time.Minute)
			}
			start := offlineCommand(t, "restore-case", "start", started, time.Time{}, now, 1)
			if _, err := repo.SynchronizeTimer(context.Background(), start); err != nil {
				t.Fatal(err)
			}
			if err := db.Table("path_models").Where("id = ?", "offline-path").Update("archived_at", nil).Error; err != nil {
				t.Fatal(err)
			}
			start.Idempotency.Key = "restore-repeated-start"
			start.Audit.ID = "restore-repeated-audit"
			repeated, err := repo.SynchronizeTimer(context.Background(), start)
			if err != nil || repeated.Timer != nil {
				t.Fatalf("restoration restarted old timer: %+v %v", repeated, err)
			}
			stop := offlineCommand(t, "restore-case", "stop", started, now, now, 2)
			result, err := repo.SynchronizeTimer(context.Background(), stop)
			if err != nil || result.Outcome != "archived" || !result.Terminal {
				t.Fatalf("archive lost: %+v %v", result, err)
			}
			if rejected {
				if result.Activity != nil || result.SavedSeconds != 0 {
					t.Fatalf("saved rejected time: %+v", result)
				}
			} else if result.Activity == nil || !result.Activity.EndedAt.Equal(archived) || result.SavedSeconds != 1800 || result.DiscardedSeconds != 1800 {
				t.Fatalf("lost split: %+v", result)
			}
		})
	}
}

func TestPostgresOfflineConflictPersistsForPreUpgradeRunningTimer(t *testing.T) {
	db := offlinePostgresDB(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
	older := offlineCommand(t, "legacy-running", "start", now.Add(-time.Hour), time.Time{}, now, 1)
	// A pre-upgrade running row has no entry in the new identity registry.
	if err := db.Create(fromTimer(older.Timer)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	newer := offlineCommand(t, "new-winner", "start", now.Add(-30*time.Minute), time.Time{}, now, 2)
	if _, err := repo.SynchronizeTimer(context.Background(), newer); err != nil {
		t.Fatal(err)
	}
	late := offlineCommand(t, older.Timer.ID, "stop", older.Timer.StartedAt, now, now, 3)
	result, err := repo.SynchronizeTimer(context.Background(), late)
	if err != nil || result.Outcome != "conflict" || result.Activity != nil {
		t.Fatalf("legacy loser saved time: %+v %v", result, err)
	}
}

func TestPostgresOfflineCorrectionRecordsReviewedTimingOnceAndPreservesUnrelatedTimer(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run(map[bool]string{false: "unacknowledged-future-clock", true: "acknowledged-original"}[known], func(t *testing.T) {
			db := offlinePostgresDB(t)
			now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
			seedParticipantAndPath(t, db, "offline-owner", "offline-path", now)
			if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
				t.Fatal(err)
			}
			repo := New(db)
			original := now.Add(-time.Minute)
			if known {
				start := offlineCommand(t, "clock", "start", original, time.Time{}, now, 1)
				if _, err := repo.SynchronizeTimer(context.Background(), start); err != nil {
					t.Fatal(err)
				}
			} else {
				original = now.Add(time.Hour)
				start := offlineCommand(t, "unrelated", "start", now.Add(-10*time.Minute), time.Time{}, now, 2)
				if _, err := repo.SynchronizeTimer(context.Background(), start); err != nil {
					t.Fatal(err)
				}
			}
			correction := offlineCommand(t, "clock", "stop", now.Add(-time.Minute), now, now, 3)
			correction.Kind = "correct"
			correction.Timer.StartedAt = original
			correction.IdentityHash = application.OfflineTimerIdentityHash(correction.Timer)
			reviewed := now.Add(-5 * time.Minute)
			ended := now.Add(-2 * time.Minute)
			correction.CorrectedStartedAt, correction.EndedAt = &reviewed, &ended
			result, err := repo.SynchronizeTimer(context.Background(), correction)
			if err != nil || result.Activity == nil || result.SavedSeconds != 180 || !result.Activity.StartedAt.Equal(reviewed) || !result.Activity.EndedAt.Equal(ended) {
				t.Fatalf("correction=%+v err=%v", result, err)
			}
			replay, err := repo.SynchronizeTimer(context.Background(), correction)
			if err != nil || !replay.Replayed || replay.Activity == nil || replay.Activity.ID != result.Activity.ID {
				t.Fatalf("replay=%+v err=%v", replay, err)
			}
			var activities, timers int64
			if err := db.Model(&activityModel{}).Where("participant_id = ?", "offline-owner").Count(&activities).Error; err != nil || activities != 1 {
				t.Fatalf("activities=%d err=%v", activities, err)
			}
			if err := db.Model(&timerModel{}).Where("participant_id = ?", "offline-owner").Count(&timers).Error; err != nil || (known && timers != 0) || (!known && timers != 1) {
				t.Fatalf("timers=%d err=%v", timers, err)
			}
			correction.Idempotency.RequestHash = bytes.Repeat([]byte{7}, 32)
			if _, err := repo.SynchronizeTimer(context.Background(), correction); !errors.Is(err, ports.ErrIdempotencyConflict) {
				t.Fatalf("changed correction=%v", err)
			}
		})
	}
}
