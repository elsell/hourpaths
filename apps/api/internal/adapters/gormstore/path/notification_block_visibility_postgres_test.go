package pathstore

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresBlockHidesRetainedNotificationAndUnreadCountUntilUnblocked(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	recipient, actor := "block-notification-recipient-"+suffix, "block-notification-actor-"+suffix
	notificationID := "block-notification-" + suffix
	seedInvitationUser(t, migrationDB, recipient, "BlockRecipient."+suffix, identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, migrationDB, actor, "BlockActor."+suffix, identity.ProfileVisibilityPublic, now)
	t.Cleanup(func() {
		migrationDB.Table("notification_models").Where("id = ?", notificationID).Delete(&notificationModel{})
		migrationDB.Table("user_models").Where("id IN ?", []string{recipient, actor}).Delete(&invitationUserModel{})
	})
	if err := migrationDB.Table("notification_models").Create(map[string]any{
		"id": notificationID, "recipient_user_id": recipient, "actor_user_id": actor,
		"follow_subject_user_id": actor, "kind": "new_follower", "presentation_class": "informational",
		"channel": "following", "created_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository := New(runtimeDB)
	visible, err := repository.ListNotifications(ctx, recipient, application.NotificationPageRequest{Snapshot: now.Add(time.Minute), Limit: 10})
	if err != nil || len(visible.Items) != 1 || visible.UnreadCount != 1 {
		t.Fatalf("visible notification=%+v err=%v", visible, err)
	}
	if err := migrationDB.Table("block_models").Create(map[string]any{"blocker_user_id": actor, "blocked_user_id": recipient, "created_at": now.Add(time.Second)}).Error; err != nil {
		t.Fatal(err)
	}

	hidden, err := repository.ListNotifications(ctx, recipient, application.NotificationPageRequest{Snapshot: now.Add(time.Minute), Limit: 10})
	if err != nil || len(hidden.Items) != 0 || hidden.UnreadCount != 0 {
		t.Fatalf("blocked notification=%+v err=%v", hidden, err)
	}
	if _, err := repository.GetNotification(ctx, recipient, notificationID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("blocked notification resolution err=%v, want opaque not found", err)
	}

	if err := migrationDB.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", actor, recipient).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	restored, err := repository.ListNotifications(ctx, recipient, application.NotificationPageRequest{Snapshot: now.Add(time.Minute), Limit: 10})
	if err != nil || len(restored.Items) != 1 || restored.Items[0].ID != notificationID || restored.UnreadCount != 1 {
		t.Fatalf("restored notification=%+v err=%v", restored, err)
	}
}
