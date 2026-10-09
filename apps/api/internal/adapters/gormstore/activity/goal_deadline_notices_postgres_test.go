package activitystore

import (
	"context"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

func TestPostgresGoalDeadlineNoticeRechecksProgressPreferencesAndReceipt(t *testing.T) {
	db := postgresDB(t, false)
	ctx := context.Background()
	r := New(db)
	end := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	at := end.Add(-30 * time.Minute)
	owner, path := "deadline-owner", "deadline-path"
	seedParticipantAndPath(t, db, owner, path, at.Add(-24*time.Hour))
	if err := db.Table("user_preference_models").Create(map[string]any{"user_id": owner, "first_day_of_week": 1, "current_time_zone": "UTC", "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_models").Where("id = ?", path).Updates(map[string]any{"interval_goal_target_seconds": 3600, "interval_goal_recurrence": "daily", "interval_goal_start_hour": 0}).Error; err != nil {
		t.Fatal(err)
	}
	entry := domain.RecordedActivity{ID: "deadline-history", PathID: path, ParticipantID: owner, StartedAt: end.Add(-12 * time.Hour), EndedAt: end.Add(-12*time.Hour + 20*time.Minute), OccurrenceTimeZone: "UTC", CreatedAt: at, UpdatedAt: at}
	if err := db.Create(fromActivity(entry)).Error; err != nil {
		t.Fatal(err)
	}
	candidate := application.GoalDeadlineCandidate{ParticipantID: owner, PathID: path}
	c := application.GoalDeadlineNoticeCommand{Candidate: candidate, At: at, Audit: timerAudit("deadline-audit", owner, path, audit.ResourceCreated, at)}
	c.Audit.TargetType = "goal_deadline_notice"
	page, err := r.ListGoalDeadlineCandidates(ctx, end.Add(-40*time.Minute), application.GoalDeadlineCandidate{}, 10)
	if err != nil || len(page.Candidates) != 0 {
		t.Fatalf("exactly enough time: %+v %v", page, err)
	}
	page, err = r.ListGoalDeadlineCandidates(ctx, at, application.GoalDeadlineCandidate{}, 10)
	if err != nil || len(page.Candidates) != 1 || page.Candidates[0] != candidate {
		t.Fatalf("impossible goal missing: %+v %v", page, err)
	}
	timer, _ := domain.StartTimer("deadline-timer", path, owner, end.Add(-40*time.Minute), "UTC", at)
	if err := db.Create(fromTimer(timer)).Error; err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishGoalDeadlineNotice(ctx, c); err != nil || sent {
		t.Fatalf("active timer: %v %v", sent, err)
	}
	_, err = r.StopTimer(ctx, application.StopTimerCommand{TimerID: timer.ID, PathID: path, ParticipantID: owner, ActivityID: "deadline-stop", StoppedAt: at, RecordedAt: at, Idempotency: idempotency(owner, application.StopTimerOperation, "deadline-stop-key", 93), Audit: timerAudit("deadline-stop-audit", owner, timer.ID, audit.ActivityTimerStopped, at)})
	if err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishGoalDeadlineNotice(ctx, c); err != nil || sent {
		t.Fatalf("stopped progress not counted: %v %v", sent, err)
	}
	c.At = c.At.Add(time.Minute)
	c.Audit.OccurredAt = c.At
	pref := goalReminderPreferenceModel{ParticipantID: owner, PathID: path, Enabled: false, Revision: 1, CreatedAt: at, UpdatedAt: at}
	if err := db.Create(&pref).Error; err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishGoalDeadlineNotice(ctx, c); err != nil || sent {
		t.Fatalf("disabled Path preference: %v %v", sent, err)
	}
	if err := db.Model(&goalReminderPreferenceModel{}).Where("participant_id = ? AND path_id = ?", owner, path).UpdateColumn("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("notification_channel_preference_models").Create(map[string]any{"user_id": owner, "channel": "goal_reminders", "enabled": false, "revision": 1, "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishGoalDeadlineNotice(ctx, c); err != nil || sent {
		t.Fatalf("disabled channel: %v %v", sent, err)
	}
	if err := db.Table("notification_channel_preference_models").Where("user_id = ? AND channel = 'goal_reminders'", owner).UpdateColumn("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(fromAudit(c.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishGoalDeadlineNotice(ctx, c); err == nil || sent {
		t.Fatal("unaudited notice committed")
	}
	c.Audit.ID = "deadline-success-audit"
	for i := 0; i < 2; i++ {
		sent, err := r.PublishGoalDeadlineNotice(ctx, c)
		if err != nil || sent != (i == 0) {
			t.Fatalf("publish %d: %v %v", i, sent, err)
		}
	}
	if err := db.Table("notification_models").Where("recipient_user_id = ? AND kind = 'goal_no_longer_achievable'", owner).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishGoalDeadlineNotice(ctx, c); err != nil || sent {
		t.Fatalf("dismissal reset receipt: %v %v", sent, err)
	}
}
