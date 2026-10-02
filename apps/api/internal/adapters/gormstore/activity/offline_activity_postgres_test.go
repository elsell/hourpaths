package activitystore

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"strings"
	"testing"
	"time"
)

func TestPostgresRecordedReplayRetainsLosingEdit(t *testing.T) {
	db := offlinePostgresDB(t)
	var present bool
	if err := db.Raw("SELECT to_regclass('public.activity_edit_order_models') IS NOT NULL").Scan(&present).Error; err != nil {
		t.Fatal(err)
	}
	if !present {
		sql, err := dbmigrations.Files.ReadFile("000070_offline_activity_replay.up.sql")
		if err != nil {
			t.Fatal(err)
		}
		statement := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(sql)), "BEGIN;"), "COMMIT;")
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	seedParticipantAndPath(t, db, "recorded-owner", "recorded-path", now)
	if err := db.Exec("SET LOCAL ROLE app").Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	makeCommand := func(kind, note, key string, offset time.Duration, hash byte) application.OfflineActivityCommand {
		entry, err := domain.RecordManualActivity(domain.ManualActivity{ID: "recorded-entry", PathID: "recorded-path", ParticipantID: "recorded-owner", StartedAt: now.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "UTC", Note: note}, now)
		if err != nil {
			t.Fatal(err)
		}
		order, err := domain.NewActivityEditOrder(now.Add(offset), 0, key)
		if err != nil {
			t.Fatal(err)
		}
		action := audit.ResourceCreated
		if kind == "edit" {
			action = audit.ResourceUpdated
		}
		event := timerAudit("audit-"+key, entry.ParticipantID, entry.ID, action, now)
		event.TargetType = "activity"
		return application.OfflineActivityCommand{Kind: kind, Activity: entry, Order: order, Idempotency: idempotency(entry.ParticipantID, application.OfflineActivityOperation, key, hash), Audit: event}
	}
	create := makeCommand("create", "initial", "recorded-create-0001", -3*time.Minute, 1)
	newer := makeCommand("edit", "newer", "recorded-newer-0001", -time.Minute, 2)
	older := makeCommand("edit", "older", "recorded-older-0001", -2*time.Minute, 3)
	for _, command := range []application.OfflineActivityCommand{create, newer, older} {
		result, err := repo.SynchronizeActivity(context.Background(), command)
		if err != nil || result.Activity == nil {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if command.Kind == "edit" && result.Activity.Note != "newer" {
			t.Fatalf("late edit replaced winner: %+v", result)
		}
	}
	retry, err := repo.SynchronizeActivity(context.Background(), create)
	if err != nil || !retry.Replayed || retry.Activity.Note != "newer" {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	var revisions int64
	if err := db.Model(&activityRevisionModel{}).Where("activity_id = ?", "recorded-entry").Count(&revisions).Error; err != nil || revisions != 2 {
		t.Fatalf("revisions=%d err=%v", revisions, err)
	}
}
