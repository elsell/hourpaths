package pathstore

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPathAccessAfterMemberRemovalUsesVisibilityAndAcceptedFollow(t *testing.T) {
	for _, test := range []struct {
		name, visibility string
		followsOwner     bool
		want             bool
	}{
		{name: "private", visibility: "private", followsOwner: true, want: false},
		{name: "followers without follow", visibility: "followers", followsOwner: false, want: false},
		{name: "followers with follow", visibility: "followers", followsOwner: true, want: true},
		{name: "public", visibility: "public", followsOwner: false, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := retainsPathAccessAfterMemberRemoval(test.visibility, test.followsOwner, false); got != test.want {
				t.Fatalf("retainsPathAccessAfterMemberRemoval(%q, %t, false)=%t want %t", test.visibility, test.followsOwner, got, test.want)
			}
		})
	}
}

func TestPostgresRemoveFollowersPathSupporterUsesCurrentFollowAccess(t *testing.T) {
	for _, followsOwner := range []bool{false, true} {
		t.Run(fmt.Sprintf("follows-owner-%t", followsOwner), func(t *testing.T) {
			runtimeDB, migrationDB := goalUpdateDatabases(t)
			suffix := fmt.Sprintf("%t-%d", followsOwner, time.Now().UnixNano())
			usernameSuffix := fmt.Sprintf("%t%d", followsOwner, time.Now().UnixNano())
			owner, supporter := "followers-owner-"+suffix, "followers-supporter-"+suffix
			pathID, nudgeID, noticeID := "followers-path-"+suffix, "followers-nudge-"+suffix, "followers-notice-"+suffix
			now := time.Now().UTC().Truncate(time.Microsecond)

			seedGoalUpdateUsers(t, migrationDB, now, owner, supporter)
			for userID, username := range map[string]string{owner: "fo" + usernameSuffix, supporter: "fs" + usernameSuffix} {
				if err := migrationDB.Table("user_models").Where("id = ?", userID).Updates(map[string]any{"username": username, "display_name": username}).Error; err != nil {
					t.Fatal(err)
				}
			}
			seedGoalUpdatePath(t, migrationDB, domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Followers path", Visibility: "followers"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}, owner)
			if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: supporter, Role: string(domain.RoleSupporter), JoinedAt: now.Add(-time.Hour)}).Error; err != nil {
				t.Fatal(err)
			}
			if followsOwner {
				if err := migrationDB.Table("follow_models").Create(map[string]any{"follower_user_id": supporter, "following_user_id": owner, "created_at": now.Add(-time.Hour)}).Error; err != nil {
					t.Fatal(err)
				}
			}
			t.Cleanup(func() { cleanupDeletionFixture(migrationDB, []string{owner, supporter}, []string{pathID}) })
			if err := migrationDB.Table("social_nudge_models").Create(map[string]any{"id": nudgeID, "sender_user_id": owner, "recipient_user_id": supporter, "path_id": pathID, "content_kind": "preset", "preset": "keep_it_going", "sent_at": now.Add(-time.Second)}).Error; err != nil {
				t.Fatal(err)
			}
			seedNudgeNotification(t, migrationDB, notificationModel{ID: noticeID, RecipientUserID: supporter, ActorUserID: owner, PathID: pathID, NudgeID: nudgeID, Kind: nudgeReceivedNotificationKind, PresentationClass: "informational", Channel: "nudges", CreatedAt: now.Add(-time.Second)})

			command := removalNotificationTestCommand(owner, supporter, pathID, suffix, now)
			if result, err := New(runtimeDB).RemoveMember(context.Background(), command); err != nil || !result.Removed {
				t.Fatalf("RemoveMember()=%+v err=%v", result, err)
			}
			_, err := New(runtimeDB).GetNotification(context.Background(), supporter, noticeID)
			if followsOwner && err != nil {
				t.Fatalf("retained follower notification error=%v", err)
			}
			if !followsOwner && !errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("inaccessible follower notification error=%v", err)
			}
			var active int64
			if err := migrationDB.Table("notification_models").Where("id = ? AND deleted_at IS NULL", noticeID).Count(&active).Error; err != nil {
				t.Fatal(err)
			}
			wantActive := int64(0)
			if followsOwner {
				wantActive = 1
			}
			if active != wantActive {
				t.Fatalf("active target notification=%d want %d", active, wantActive)
			}
			page, err := New(runtimeDB).ListNotifications(context.Background(), supporter, application.NotificationPageRequest{Limit: 25, Snapshot: now.Add(time.Second)})
			wantItems := 1
			if followsOwner {
				wantItems = 2
			}
			if err != nil || len(page.Items) != wantItems || page.UnreadCount != int64(wantItems) {
				t.Fatalf("ListNotifications()=%+v err=%v", page, err)
			}
		})
	}
}
