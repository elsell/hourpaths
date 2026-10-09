package gormstore

import (
	"bytes"
	"context"
	"errors"
	timerstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationtimer"
	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"testing"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPostgresTimerStartNotificationDeduplicatesAndRechecksDisabledChannel(t *testing.T) {
	f := newNudgeFixture(t, "timerproducer", false)
	push, err := NewPushRepository(f.runtime.DB, bytes.Repeat([]byte{0x61}, 32), pushTestClock{f.now})
	if err != nil {
		t.Fatal(err)
	}
	installationID := "timer-installation-" + newTestID()
	if err := push.UpsertPushInstallation(context.Background(), ports.PushInstallation{ID: installationID, OwnerUserID: f.recipient.ID, Provider: "expo", Platform: "ios", Locale: "en", Token: "ExponentPushToken[timer-subscription-test]", CreatedAt: f.now, UpdatedAt: f.now}, audit.Event{ID: newTestID(), OwnerUserID: f.recipient.ID, ActorUserID: f.recipient.ID, Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: f.now}); err != nil {
		t.Fatal(err)
	}
	timer, err := domain.StartTimer("timer-"+newTestID(), f.pathID, f.sender.ID, f.now, "Etc/UTC", f.now)
	if err != nil {
		t.Fatal(err)
	}
	write := func(rollback bool) error {
		return f.runtime.DB.Transaction(func(tx *gorm.DB) error {
			if err := timerstore.Create(tx, timer, []string{f.recipient.ID}, f.now); err != nil {
				return err
			}
			if rollback {
				return ports.ErrUnavailable
			}
			return nil
		})
	}
	count := func(want int64) {
		t.Helper()
		var found int64
		if err := f.migration.DB.Table("notification_models").Where("timer_id = ? AND recipient_user_id = ?", timer.ID, f.recipient.ID).Count(&found).Error; err != nil || found != want {
			t.Fatalf("notifications=%d want=%d err=%v", found, want, err)
		}
	}
	if err := write(true); err == nil {
		t.Fatal("rollback not propagated")
	}
	count(0)
	if err := write(false); err != nil {
		t.Fatal(err)
	}
	count(1)
	if err := write(false); err != nil {
		t.Fatal(err)
	}
	count(1)
	var queued int64
	if err := f.migration.DB.Table("notification_push_outbox_models queue").Joins("JOIN notification_models n ON n.id = queue.notification_id").Where("n.timer_id = ?", timer.ID).Count(&queued).Error; err != nil || queued != 1 {
		t.Fatalf("push jobs=%d err=%v", queued, err)
	}
	subscriptions := NewTimerSubscriptionRepository(f.runtime.DB)
	for index, enabled := range []bool{false, true} {
		command := timerSubscriptionTestCommand(f.recipient.ID, f.pathID, socialapp.TimerSubscriptionPath, "timer-delivery-setting-"+newTestID(), int64(index), enabled, f.now.Add(time.Duration(index+1)*time.Second))
		if _, err := subscriptions.UpdateTimerSubscription(context.Background(), command); err != nil {
			t.Fatal(err)
		}
	}
	count(1) // Preference changes preserve already received history.
	if err := write(false); err != nil {
		t.Fatal(err)
	}
	var delivery struct {
		SuppressedAt    *time.Time
		TokenCiphertext []byte
	}
	if err := f.migration.DB.Table("notification_push_delivery_models delivery").Select("delivery.suppressed_at, delivery.token_ciphertext").Joins("JOIN notification_models notice ON notice.id = delivery.notification_id").Where("notice.timer_id = ?", timer.ID).Take(&delivery).Error; err != nil || delivery.SuppressedAt == nil || delivery.TokenCiphertext != nil {
		t.Fatalf("old delivery resurrected: %+v %v", delivery, err)
	}
	if err := f.migration.DB.Table("notification_channel_preference_models").Create(map[string]any{"user_id": f.recipient.ID, "channel": "tracking_activity", "enabled": false, "revision": 1, "created_at": f.now, "updated_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	timer.ID = "timer-" + newTestID()
	if err := write(false); err != nil {
		t.Fatal(err)
	}
	count(0)
}

func TestPostgresTimerNotificationHistoryNegotiatesVocabularyAndVisibility(t *testing.T) {
	f := newNudgeFixture(t, "timerhistory", false)
	timer, err := domain.StartTimer("timer-"+newTestID(), f.pathID, f.sender.ID, f.now, "Etc/UTC", f.now)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.DB.Transaction(func(tx *gorm.DB) error { return timerstore.Create(tx, timer, []string{f.recipient.ID}, f.now) }); err != nil {
		t.Fatal(err)
	}
	repository := pathstore.New(f.runtime.DB)
	request := pathapp.NotificationPageRequest{Limit: 25, Snapshot: f.now.Add(time.Minute), EmojiReactions: true}
	old, err := repository.ListNotifications(context.Background(), f.recipient.ID, request)
	if err != nil || len(old.Items) != 0 || old.UnreadCount != 0 {
		t.Fatalf("old client: %+v %v", old, err)
	}
	request.TimerStarts = true
	page, err := repository.ListNotifications(context.Background(), f.recipient.ID, request)
	if err != nil || len(page.Items) != 1 || page.UnreadCount != 1 {
		t.Fatalf("timer history: %+v %v", page, err)
	}
	notice := page.Items[0]
	if notice.Kind != pathapp.NotificationTimerStarted || string(notice.PathID) != f.pathID || notice.Actor.UserID != f.sender.ID {
		t.Fatalf("projection: %+v", notice)
	}
	if _, err := repository.GetNotification(context.Background(), f.recipient.ID, notice.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetNotification(context.Background(), f.sender.ID, notice.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("cross user: %v", err)
	}
	// Losing both membership and following removes the history target as well as its count.
	if err := f.migration.DB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", f.pathID, f.recipient.ID).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("follow_models").Where("follower_user_id = ? AND following_user_id = ?", f.recipient.ID, f.sender.ID).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	page, err = repository.ListNotifications(context.Background(), f.recipient.ID, request)
	if err != nil || len(page.Items) != 0 || page.UnreadCount != 0 {
		t.Fatalf("inaccessible history: %+v %v", page, err)
	}
	if _, err := repository.GetNotification(context.Background(), f.recipient.ID, notice.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("inaccessible target: %v", err)
	}
}
