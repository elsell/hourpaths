package activitystore

import (
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresGoalReminderPreferenceIsScopedRetrySafeAndAtomic(t *testing.T) {
	db := postgresDB(t, false)
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, path := "reminder-pref-owner", "reminder-pref-path"
	seedParticipantAndPath(t, db, owner, path, now.Add(-time.Hour))
	r := New(db)
	ctx := context.Background()
	pref, err := r.GetGoalReminderPreference(ctx, owner, path)
	if err != nil || !pref.Enabled || pref.Revision != 0 {
		t.Fatalf("default: %+v %v", pref, err)
	}
	if _, err := r.GetGoalReminderPreference(ctx, "other-participant", path); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("foreign read: %v", err)
	}
	event := timerAudit("reminder-pref-audit", owner, path, audit.ResourceUpdated, now)
	event.TargetType = "goal_reminder_preference"
	c := application.GoalReminderPreferenceCommand{ParticipantID: owner, PathID: path, Enabled: false, ExpectedRevision: 0, At: now, Audit: event, Idempotency: idempotency(owner, application.UpdateGoalReminderPreferenceOperation, "reminder-pref-key", 91)}
	first, err := r.UpdateGoalReminderPreference(ctx, c)
	if err != nil || first.Preference.Enabled || first.Preference.Revision != 1 || first.Replayed {
		t.Fatalf("save: %+v %v", first, err)
	}
	replay, err := r.UpdateGoalReminderPreference(ctx, c)
	if err != nil || !replay.Replayed || replay.Preference != first.Preference {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	stale := c
	stale.Idempotency.Key = "reminder-stale-key"
	stale.Audit.ID = "reminder-stale-audit"
	if _, err := r.UpdateGoalReminderPreference(ctx, stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale save: %v", err)
	}
	next := c
	next.ExpectedRevision = 1
	next.Enabled = true
	next.Idempotency = idempotency(owner, application.UpdateGoalReminderPreferenceOperation, "reminder-next-key", 92)
	// Reusing the audit identity must roll back the preference and replay record.
	if _, err := r.UpdateGoalReminderPreference(ctx, next); err == nil {
		t.Fatal("unaudited save committed")
	}
	pref, err = r.GetGoalReminderPreference(ctx, owner, path)
	if err != nil || pref != first.Preference {
		t.Fatalf("rollback: %+v %v", pref, err)
	}
	next.Audit.ID = "reminder-next-audit"
	if _, err := r.UpdateGoalReminderPreference(ctx, next); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateGoalReminderPreference(ctx, c); err != nil {
		t.Fatal(err)
	}
	pref, err = r.GetGoalReminderPreference(ctx, owner, path)
	if err != nil || !pref.Enabled || pref.Revision != 2 {
		t.Fatalf("old retry overwrote newer preference: %+v %v", pref, err)
	}
}
