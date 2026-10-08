package gormstore

import (
	"context"
	"errors"
	achievementstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationachievement"
	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresAchievementHistoryNegotiatesVocabularyAndCurrentMembership(t *testing.T) {
	f := newNudgeFixture(t, "achievementhistory", false)
	noticeID, eventID := seedAchievementNotice(t, f)
	repository := pathstore.New(f.runtime.DB)
	request := pathapp.NotificationPageRequest{Limit: 25, Snapshot: f.now.Add(time.Minute), EmojiReactions: true, TimerStarts: true}
	old, err := repository.ListNotifications(context.Background(), f.recipient.ID, request)
	if err != nil || len(old.Items) != 0 || old.UnreadCount != 0 {
		t.Fatalf("legacy page=%+v err=%v", old, err)
	}
	if eligible, err := achievementstore.Eligible(f.runtime.DB, noticeID, f.recipient.ID); err != nil || !eligible {
		t.Fatalf("current handoff=%t err=%v", eligible, err)
	}
	request.Achievements = true
	page, err := repository.ListNotifications(context.Background(), f.recipient.ID, request)
	if err != nil || len(page.Items) != 1 || page.UnreadCount != 1 {
		t.Fatalf("achievement page=%+v err=%v", page, err)
	}
	notice := page.Items[0]
	if notice.Kind != pathapp.NotificationOverallTargetAchieved || notice.Actor.UserID != f.recipient.ID || notice.SocialFeedEventID != eventID {
		t.Fatalf("notice=%+v", notice)
	}
	if _, err := repository.GetNotification(context.Background(), f.sender.ID, noticeID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("cross-user notice=%v", err)
	}
	if err := f.migration.DB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", f.pathID, f.recipient.ID).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	page, err = repository.ListNotifications(context.Background(), f.recipient.ID, request)
	if err != nil || len(page.Items) != 0 || page.UnreadCount != 0 {
		t.Fatalf("revoked page=%+v err=%v", page, err)
	}
	if _, err := repository.GetNotification(context.Background(), f.recipient.ID, noticeID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("revoked notice=%v", err)
	}
	if eligible, err := achievementstore.Eligible(f.runtime.DB, noticeID, f.recipient.ID); err != nil || eligible {
		t.Fatalf("revoked handoff=%t err=%v", eligible, err)
	}
	if err := f.migration.DB.Table("path_membership_models").Create(map[string]any{"path_id": f.pathID, "user_id": f.recipient.ID, "role": "participant", "joined_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := suppressBlockedPairPush(f.runtime.DB, f.sender.ID, f.recipient.ID, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetNotification(context.Background(), f.recipient.ID, noticeID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("owner block did not retire own achievement: %v", err)
	}
	if eligible, err := achievementstore.Eligible(f.runtime.DB, noticeID, f.recipient.ID); err != nil || eligible {
		t.Fatalf("blocked owner handoff=%t err=%v", eligible, err)
	}

}

func seedAchievementNotice(t *testing.T, f nudgeFixture) (string, string) {
	t.Helper()
	achievementID := "achievement-" + newTestID()
	eventID := "achievement:" + achievementID
	noticeID := "achievement-notice:" + achievementID
	if err := f.runtime.DB.Table("social_goal_achievement_models").Create(map[string]any{"id": achievementID, "participant_user_id": f.recipient.ID, "path_id": f.pathID, "kind": "overall", "target_seconds": 60, "published_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.DB.Table("social_feed_event_models").Create(map[string]any{"id": eventID, "achievement_id": achievementID, "participant_user_id": f.recipient.ID, "path_id": f.pathID, "published_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.DB.Table("notification_models").Create(map[string]any{"id": noticeID, "recipient_user_id": f.recipient.ID, "actor_user_id": f.recipient.ID, "path_id": f.pathID, "social_feed_event_id": eventID, "kind": "overall_target_achieved", "presentation_class": "informational", "channel": "achievements", "created_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	return noticeID, eventID
}

func TestPostgresSelfAchievementCanBeReadAndDeleted(t *testing.T) {
	f := newNudgeFixture(t, "achievementmutation", false)
	noticeID, _ := seedAchievementNotice(t, f)
	repository := pathstore.New(f.runtime.DB)
	command := pathapp.NotificationMutationCommand{Achievements: true, EmojiReactions: true, TimerStarts: true, RecipientUserID: f.recipient.ID, NotificationID: noticeID, ChangedAt: f.now, Audit: audit.Event{ID: "achievement-read-" + newTestID(), OwnerUserID: f.recipient.ID, ActorUserID: f.recipient.ID, Action: audit.ResourceUpdated, TargetType: "notification", TargetID: noticeID, Outcome: audit.Succeeded, CorrelationID: "achievement-read", OccurredAt: f.now}}
	result, err := repository.MarkNotificationRead(context.Background(), command)
	if err != nil || result.UnreadCount != 0 {
		t.Fatalf("self read=%+v err=%v", result, err)
	}
	notice, err := repository.GetNotification(context.Background(), f.recipient.ID, noticeID)
	if err != nil || !notice.Read {
		t.Fatalf("self read not retained=%+v err=%v", notice, err)
	}
	command.Audit.ID = "achievement-delete-" + newTestID()
	command.Audit.Action = audit.ResourceDeleted
	result, err = repository.DeleteNotification(context.Background(), command)
	if err != nil || result.UnreadCount != 0 {
		t.Fatalf("self delete=%+v err=%v", result, err)
	}
	if _, err := repository.GetNotification(context.Background(), f.recipient.ID, noticeID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("deleted notice=%v", err)
	}
}
