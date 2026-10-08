package activitystore

import (
	"context"
	"fmt"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

func TestPostgresAchievementNotifiesOnlyParticipantAndHonorsChannel(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			db := postgresDB(t, false)
			now := achievementTestInstant
			participant, path := "achievement-notice-participant", "achievement-notice-path"
			seedParticipantAndPath(t, db, participant, path, now)
			seedAchievementPreference(t, db, participant, "Etc/UTC", now)
			configureAchievementGoals(t, db, path, 60, 60)
			if !enabled {
				if err := db.Table("notification_channel_preference_models").Create(map[string]any{"user_id": participant, "channel": "achievements", "enabled": false, "revision": 1, "created_at": now, "updated_at": now}).Error; err != nil {
					t.Fatal(err)
				}
			}
			entry := manualAchievementActivity(t, "achievement-notice-entry", participant, path, now.Add(-time.Minute), 60, now)
			command := application.CreateManualActivityCommand{Activity: entry, Idempotency: idempotency(participant, application.CreateManualActivityOperation, "achievement-notice-create", 81), Audit: activityAudit("achievement-notice-audit", participant, entry.ID, audit.ResourceCreated, now)}
			repository := New(db)
			for replay := 0; replay < 2; replay++ {
				if _, err := repository.CreateManualActivity(context.Background(), command); err != nil {
					t.Fatal(err)
				}
			}
			var notices []struct{ ID, RecipientUserID, ActorUserID, SocialFeedEventID, Kind, Channel, PresentationClass string }
			if err := db.Table("notification_models").Where("path_id = ?", path).Find(&notices).Error; err != nil {
				t.Fatal(err)
			}
			want := 0
			if enabled {
				want = 2
			}
			if len(notices) != want {
				t.Fatalf("achievement notices=%+v, want %d", notices, want)
			}
			legacyCount, err := activityDeletionVisibleUnreadNotificationCount(db, participant, now)
			if err != nil || legacyCount != 0 {
				t.Fatalf("legacy deletion count=%d err=%v, want 0", legacyCount, err)
			}
			currentCount, err := activityDeletionVisibleUnreadNotificationCount(db, participant, now, true)
			if err != nil || currentCount != int64(want) {
				t.Fatalf("negotiated deletion count=%d err=%v, want %d", currentCount, err, want)
			}
			kinds := map[string]bool{}
			for _, notice := range notices {
				if notice.RecipientUserID != participant || notice.ActorUserID != participant || notice.SocialFeedEventID == "" || notice.Channel != "achievements" || notice.PresentationClass != "informational" {
					t.Fatalf("unsafe achievement notice: %+v", notice)
				}
				kinds[notice.Kind] = true
				var queued int64
				if err := db.Table("notification_push_outbox_models").Where("notification_id = ?", notice.ID).Count(&queued).Error; err != nil || queued != 1 {
					t.Fatalf("push work=%d err=%v", queued, err)
				}
			}
			if enabled && (!kinds["interval_goal_achieved"] || !kinds["overall_target_achieved"]) {
				t.Fatalf("achievement kinds=%v", kinds)
			}
			if _, err := repository.DeleteActivity(context.Background(), application.DeleteActivityCommand{ActivityID: entry.ID, PathID: path, ParticipantID: participant, Idempotency: idempotency(participant, application.DeleteActivityOperation, "achievement-notice-delete", 82), Audit: activityAudit("achievement-notice-delete-audit", participant, entry.ID, audit.ResourceDeleted, now.Add(time.Second))}); err != nil {
				t.Fatal(err)
			}
			var remaining int64
			if err := db.Table("notification_models").Where("path_id = ?", path).Count(&remaining).Error; err != nil || remaining != 0 {
				t.Fatalf("invalidated achievement notices=%d err=%v", remaining, err)
			}
		})
	}
}
