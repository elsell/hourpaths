package activitystore

import (
	"context"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

func TestPostgresGoalReminderBundleRollsBackReceiptsWithAuditAndDoesNotChain(t *testing.T) {
	db := postgresDB(t, false)
	r := New(db)
	ctx := context.Background()
	at := time.Date(2026, 10, 9, 22, 0, 0, 0, time.UTC)
	owner := "ordinary-reminder-owner"
	seedParticipantAndPath(t, db, owner, "reminder-a", at.Add(-time.Hour))
	seedPath(t, db, owner, "reminder-b", at.Add(-time.Hour))
	seedPath(t, db, owner, "reminder-c", at.Add(-time.Hour))
	if err := db.Table("user_preference_models").Create(map[string]any{"user_id": owner, "first_day_of_week": 1, "current_time_zone": "UTC", "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	for path, target := range map[string]int64{"reminder-a": 5400, "reminder-b": 5100, "reminder-c": 5040} {
		if err := db.Table("path_models").Where("id = ?", path).Updates(map[string]any{"interval_goal_target_seconds": target, "interval_goal_recurrence": "daily", "interval_goal_start_hour": 0}).Error; err != nil {
			t.Fatal(err)
		}
	}
	c := application.GoalReminderDeliveryCommand{ParticipantID: owner, AuthorizedPathIDs: []string{"reminder-a", "reminder-b", "reminder-c"}, At: at, Audit: timerAudit("reminder-audit", owner, owner, audit.ResourceCreated, at)}
	c.Audit.TargetType = "goal_reminder_delivery"
	if err := db.Create(fromAudit(c.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if n, err := r.PublishGoalReminders(ctx, c); err == nil || n != 0 {
		t.Fatalf("unaudited bundle committed: %d %v", n, err)
	}
	var receipts int64
	if err := db.Table("goal_reminder_receipt_models").Where("participant_id = ?", owner).Count(&receipts).Error; err != nil || receipts != 0 {
		t.Fatalf("rollback receipts %d %v", receipts, err)
	}
	c.Audit.ID = "reminder-success-audit"
	if n, err := r.PublishGoalReminders(ctx, c); err != nil || n != 1 {
		t.Fatalf("bundle %d %v", n, err)
	}
	if err := db.Table("goal_reminder_receipt_models").Where("participant_id = ?", owner).Count(&receipts).Error; err != nil || receipts != 2 {
		t.Fatalf("five-minute receipts %d %v", receipts, err)
	}
	if n, err := r.PublishGoalReminders(ctx, c); err != nil || n != 0 {
		t.Fatalf("repeated bundle %d %v", n, err)
	}
	c.At = at.Add(6 * time.Minute)
	c.Audit.OccurredAt = c.At
	c.Audit.ID = "reminder-next-audit"
	if n, err := r.PublishGoalReminders(ctx, c); err != nil || n != 1 {
		t.Fatalf("nonchained third path %d %v", n, err)
	}
}

func TestPostgresGoalReminderUsesSavedUnavailablePeriodDeadline(t *testing.T) {
	db := postgresDB(t, false)
	at := time.Date(2026, 10, 9, 20, 50, 0, 0, time.UTC)
	owner, path := "quiet-reminder-owner", "quiet-reminder-path"
	seedParticipantAndPath(t, db, owner, path, at.Add(-time.Hour))
	if err := db.Table("user_preference_models").Create(map[string]any{"user_id": owner, "first_day_of_week": 1, "current_time_zone": "UTC", "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_models").Where("id = ?", path).Updates(map[string]any{"interval_goal_target_seconds": 2400, "interval_goal_recurrence": "daily", "interval_goal_start_hour": 0}).Error; err != nil {
		t.Fatal(err)
	}
	before, eligible, err := reminderPlan(db, owner, path, at)
	if err != nil || !eligible || !before.Plan.ScheduledAt.Equal(at.Add(2*time.Hour)) {
		t.Fatalf("ordinary deadline: %+v %v %v", before, eligible, err)
	}
	if err := db.Table("user_unavailable_period_models").Create(map[string]any{"user_id": owner, "enabled": true, "start_minute": 1320, "end_minute": 480, "revision": 1, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	after, eligible, err := reminderPlan(db, owner, path, at)
	if err != nil || !eligible || !after.Plan.ScheduledAt.Equal(at) {
		t.Fatalf("quiet deadline: %+v %v %v", after, eligible, err)
	}
}
