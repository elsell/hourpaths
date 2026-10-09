package gormstore

import (
	"bytes"
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresGoalDeadlinePushRechecksPreferencesTimerAndAccessAtHandoff(t *testing.T) {
	f := newNudgeFixture(t, "deadlinepush", false)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	recipient := f.recipient.ID
	installationID, id := "deadline-install-"+newTestID(), "deadline-push-"+newTestID()
	repository, err := NewPushRepository(f.runtime.DB, bytes.Repeat([]byte{0x63}, 32))
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
	if err := f.migration.DB.Table("notification_models").Create(map[string]any{"id": id, "recipient_user_id": recipient, "actor_user_id": recipient, "path_id": f.pathID, "kind": "goal_no_longer_achievable", "presentation_class": "informational", "channel": "goal_reminders", "created_at": at, "goal_interval_started_at": at.Add(-time.Hour), "goal_interval_ended_at": at.Add(time.Minute)}).Error; err != nil {
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
