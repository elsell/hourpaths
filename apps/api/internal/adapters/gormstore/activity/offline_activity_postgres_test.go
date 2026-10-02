package activitystore

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
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
	invalid := makeCommand("create", "invalid", "recorded-prejoin-0001", -time.Minute, 9)
	invalid.Activity.StartedAt = now.AddDate(0, 0, -8)
	invalid.Activity.EndedAt = invalid.Activity.StartedAt.Add(time.Minute)
	if _, err := repo.SynchronizeActivity(context.Background(), invalid); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("pre-membership occurrence classified as lost access: %v", err)
	}
	create := makeCommand("create", "initial", "recorded-create-0001", -3*time.Minute, 1)
	newer := makeCommand("edit", "newer", "recorded-newer-0001", -time.Minute, 2)
	older := makeCommand("edit", "older", "recorded-older-0001", -2*time.Minute, 3)
	older.Activity.UpdatedAt = now.Add(2 * time.Second)
	older.Audit.OccurredAt = older.Activity.UpdatedAt
	for _, command := range []application.OfflineActivityCommand{create, newer, older} {
		result, err := repo.SynchronizeActivity(context.Background(), command)
		if err != nil || result.Activity == nil {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if command.Kind == "edit" && result.Activity.Note != "newer" {
			t.Fatalf("late edit replaced winner: %+v", result)
		}
	}
	fetched, _, err := repo.GetActivity(context.Background(), create.Activity.ParticipantID, create.Activity.PathID, create.Activity.ID)
	if err != nil || fetched.EditOrder != newer.Order {
		t.Fatalf("owner lost causal metadata: %+v err=%v", fetched, err)
	}
	retry, err := repo.SynchronizeActivity(context.Background(), create)
	if err != nil || !retry.Replayed || retry.Activity.Note != "newer" {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	var revisions int64
	if err := db.Model(&activityRevisionModel{}).Where("activity_id = ?", "recorded-entry").Count(&revisions).Error; err != nil || revisions != 2 {
		t.Fatalf("revisions=%d err=%v", revisions, err)
	}
	// A losing revision must not become canonical content in a historical page.
	snapshotPage, err := repo.ListActivities(context.Background(), create.Activity.ParticipantID, create.Activity.PathID, application.ActivityPageRequest{Limit: 25, Snapshot: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshotPage.Items) != 1 || snapshotPage.Items[0].Activity.Note != "newer" {
		t.Fatalf("losing revision altered snapshot: %+v", snapshotPage)
	}
	online := application.UpdateActivityCommand{ActivityID: create.Activity.ID, PathID: create.Activity.PathID, ParticipantID: create.Activity.ParticipantID, Edit: domain.ActivityEdit{StartedAt: create.Activity.StartedAt, DurationSeconds: 90, OccurrenceTimeZone: "UTC", Note: "online"}, UpdatedAt: now.Add(time.Minute), Idempotency: idempotency(create.Activity.ParticipantID, application.UpdateActivityOperation, "recorded-online-0001", 4), Audit: activityAudit("recorded-online-audit", create.Activity.ParticipantID, create.Activity.ID, audit.ResourceUpdated, now.Add(time.Minute))}
	if _, err := repo.UpdateActivity(context.Background(), online); err != nil {
		t.Fatal(err)
	}
	delayed := makeCommand("edit", "delayed", "recorded-delayed-0001", 0, 5)
	delayed.Activity.UpdatedAt = now.Add(2 * time.Minute)
	delayed.Audit.OccurredAt = delayed.Activity.UpdatedAt
	result, err := repo.SynchronizeActivity(context.Background(), delayed)
	if err != nil || result.Activity == nil || result.Activity.Note != "online" {
		t.Fatalf("delayed replaced online: %+v err=%v", result, err)
	}

	deletedAt := now.Add(3 * time.Minute)
	deletion := application.DeleteActivityCommand{ActivityID: create.Activity.ID, PathID: create.Activity.PathID, ParticipantID: create.Activity.ParticipantID, Idempotency: idempotency(create.Activity.ParticipantID, application.DeleteActivityOperation, "recorded-delete-0001", 6), Audit: activityAudit("recorded-delete-audit", create.Activity.ParticipantID, create.Activity.ID, audit.ResourceDeleted, deletedAt)}
	if _, err := repo.DeleteActivity(context.Background(), deletion); err != nil {
		t.Fatal(err)
	}
	for _, command := range []application.OfflineActivityCommand{create, newer, delayed, makeCommand("edit", "after delete", "recorded-deleted-0001", 0, 7), makeCommand("create", "resurrect", "recorded-recreate-0001", 0, 8)} {
		result, err := repo.SynchronizeActivity(context.Background(), command)
		if err != nil || result.Outcome != "deleted" || result.Activity != nil {
			t.Fatalf("deleted replay resurrected: %+v err=%v", result, err)
		}
	}

}
