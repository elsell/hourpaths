package gormstore

import (
	"bytes"
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresGoalReminderPushRechecksPreferencesTimerAndAccessAtHandoff(t *testing.T) {
	f := newNudgeFixture(t, "reminderpush", false)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	recipient := f.recipient.ID
	installationID, id := "deadline-install-"+newTestID(), "deadline-push-"+newTestID()
	repository, err := NewPushRepository(f.runtime.DB, bytes.Repeat([]byte{0x63}, 32), pushTestClock{at})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertPushInstallation(ctx, ports.PushInstallation{ID: installationID, OwnerUserID: recipient, Provider: "expo", Platform: "ios", Locale: "en", Token: "ExponentPushToken[goal-deadline-test]", CreatedAt: at, UpdatedAt: at}, audit.Event{ID: newTestID(), OwnerUserID: recipient, ActorUserID: recipient, Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}); err != nil {
		t.Fatal(err)
	}
	var installation pushInstallationModel
	if err := f.migration.DB.Where("id = ?", installationID).Take(&installation).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("notification_models").Create(map[string]any{"id": id, "recipient_user_id": recipient, "actor_user_id": recipient, "kind": "goal_practice_reminder", "presentation_class": "informational", "channel": "goal_reminders", "created_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("goal_reminder_receipt_models").Create(map[string]any{"participant_id": recipient, "path_id": f.pathID, "interval_started_at": time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC), "interval_ended_at": time.Date(at.Year(), at.Month(), at.Day()+1, 0, 0, 0, 0, time.UTC), "notification_id": id, "scheduled_at": at, "created_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("notification_push_outbox_models").Create(map[string]any{"notification_id": id, "created_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("notification_push_delivery_models").Create(map[string]any{"notification_id": id, "installation_id": installationID, "recipient_user_id": recipient, "provider": installation.Provider, "platform": installation.Platform, "locale": installation.Locale, "token_ciphertext": installation.TokenCiphertext, "token_nonce": installation.TokenNonce, "token_hash": installation.TokenHash, "available_at": at, "created_at": at, "locked_by": "deadline-worker", "locked_until": at.Add(time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	check := func(want bool) {
		t.Helper()
		called := false
		_, sent, err := repository.HandoffPushDelivery(ctx, "deadline-worker", id, installationID, func() ports.PushTicket { called = true; return ports.PushTicket{State: ports.PushDelivered} })
		if err != nil || sent != want || called != want {
			t.Fatalf("handoff want=%v sent=%v called=%v err=%v", want, sent, called, err)
		}
	}
	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).Updates(map[string]any{"interval_goal_target_seconds": 60, "interval_goal_recurrence": "daily", "interval_goal_start_hour": 0}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Exec("INSERT INTO user_preference_models (user_id, first_day_of_week, current_time_zone, created_at, updated_at) VALUES (?,1,'UTC',?,?) ON CONFLICT (user_id) DO UPDATE SET current_time_zone='UTC'", recipient, at, at).Error; err != nil {
		t.Fatal(err)
	}
	check(true)
	// A new quiet period can make a queued reminder no longer actionable before
	// quiet hours are active. The provider must not receive it in that gap.
	quietStart := at.Add(2 * time.Minute)
	quietMinute := quietStart.Hour()*60 + quietStart.Minute()
	if err := f.migration.DB.Table("user_unavailable_period_models").Create(map[string]any{"user_id": recipient, "enabled": true, "start_minute": quietMinute, "end_minute": (quietMinute + 60) % 1440, "revision": 1, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).UpdateColumn("interval_goal_target_seconds", 180).Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("user_unavailable_period_models").Where("user_id = ?", recipient).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).UpdateColumn("interval_goal_target_seconds", 60).Error; err != nil {
		t.Fatal(err)
	}
	check(true)
	// Simulate a delayed handoff: the goal remains incomplete and current,
	// but its remaining practice now exceeds the time left in the interval.
	if err := f.migration.DB.Exec(`UPDATE path_models SET interval_goal_target_seconds =
	  CEIL(EXTRACT(EPOCH FROM (((CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::date + INTERVAL '1 day') AT TIME ZONE 'UTC' - CURRENT_TIMESTAMP))) + 60
	  WHERE id = ?`, f.pathID).Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).UpdateColumn("interval_goal_target_seconds", 60).Error; err != nil {
		t.Fatal(err)
	}
	check(true)
	if err := f.migration.DB.Table("user_preference_models").Where("user_id = ?", recipient).UpdateColumn("current_time_zone", "Asia/Tokyo").Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("user_preference_models").Where("user_id = ?", recipient).UpdateColumn("current_time_zone", "UTC").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).UpdateColumn("interval_goal_start_hour", 1).Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).UpdateColumn("interval_goal_start_hour", 0).Error; err != nil {
		t.Fatal(err)
	}
	check(true)

	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).Updates(map[string]any{"interval_goal_target_seconds": nil, "interval_goal_recurrence": nil, "interval_goal_start_hour": nil}).Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).Updates(map[string]any{"interval_goal_target_seconds": 60, "interval_goal_recurrence": "daily", "interval_goal_start_hour": 0}).Error; err != nil {
		t.Fatal(err)
	}
	activityID := "reminder-completed-" + newTestID()
	if err := f.migration.DB.Table("recorded_activity_models").Create(map[string]any{"id": activityID, "path_id": f.pathID, "participant_id": recipient, "started_at": at.Add(-time.Minute), "ended_at": at, "occurrence_time_zone": "UTC", "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("recorded_activity_models").Where("id = ?", activityID).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	check(true)
	if err := f.migration.DB.Table("goal_reminder_preference_models").Create(map[string]any{"participant_id": recipient, "path_id": f.pathID, "enabled": false, "revision": 1, "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("goal_reminder_preference_models").Where("participant_id = ? AND path_id = ?", recipient, f.pathID).UpdateColumn("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	timerID := "deadline-active-" + newTestID()
	if err := f.migration.DB.Table("running_timer_models").Create(map[string]any{"id": timerID, "participant_id": recipient, "path_id": f.pathID, "started_at": at, "occurrence_time_zone": "UTC"}).Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("running_timer_models").Where("id = ?", timerID).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	check(true)
	if err := f.migration.DB.Table("notification_channel_preference_models").Create(map[string]any{"user_id": recipient, "channel": "goal_reminders", "enabled": false, "revision": 1, "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("notification_channel_preference_models").Where("user_id = ? AND channel = 'goal_reminders'", recipient).UpdateColumn("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("block_models").Create(map[string]any{"blocker_user_id": recipient, "blocked_user_id": f.sender.ID, "created_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.migration.DB.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", recipient, f.sender.ID).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", f.pathID, recipient).UpdateColumn("role", "supporter").Error; err != nil {
		t.Fatal(err)
	}
	check(false)
}
