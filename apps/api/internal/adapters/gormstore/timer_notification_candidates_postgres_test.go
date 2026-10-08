package gormstore

import (
	"context"
	"slices"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

func TestPostgresTimerCandidatesCombineSubscriptionsWithoutBypassingPrivacy(t *testing.T) {
	f := newNudgeFixture(t, "timercandidates", false)
	r := NewTimerSubscriptionRepository(f.runtime.DB)
	outsider := socialRelationshipTestUser(t, f.migration.DB, "timerfollower", identity.ProfileVisibilityPublic, f.now)
	expect := func(want ...string) {
		t.Helper()
		got, err := r.TimerNotificationCandidates(context.Background(), f.sender.ID, f.pathID)
		slices.Sort(want)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("recipients=%v expected=%v err=%v", got, want, err)
		}
	}
	expect(f.recipient.ID)
	for _, follower := range []string{outsider.ID, f.recipient.ID} {
		if err := f.migration.DB.Create(&socialFollowModel{FollowerUserID: follower, FollowingUserID: f.sender.ID, ActivityNotificationsEnabled: true, CreatedAt: f.now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	expect(f.recipient.ID, outsider.ID) // The member is also a subscriber: one result.
	if err := f.migration.DB.Create(&pathTimerSubscriptionModel{UserID: f.recipient.ID, PathID: f.pathID, Enabled: false, Revision: 1, CreatedAt: f.now, UpdatedAt: f.now}).Error; err != nil {
		t.Fatal(err)
	}
	expect(f.recipient.ID, outsider.ID) // Follow subscription independently remains.
	if err := f.migration.DB.Table("notification_channel_preference_models").Create(map[string]any{"user_id": f.recipient.ID, "channel": "tracking_activity", "enabled": false, "revision": 1, "created_at": f.now, "updated_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	expect(outsider.ID)
	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).Update("visibility", "private").Error; err != nil {
		t.Fatal(err)
	}
	expect() // Following does not grant access to a private Path.
	if err := f.migration.DB.Table("path_models").Where("id = ?", f.pathID).Update("visibility", "public").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("block_models").Create(map[string]any{"blocker_user_id": outsider.ID, "blocked_user_id": f.sender.ID, "created_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	expect()
}
