package pathstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	sociallock "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPostgresVisibilityNarrowingRetiresOnlyInaccessibleTargetNotificationsAtomically(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, member := "narrow-owner-"+suffix, "narrow-member-"+suffix
	follower, formerFollower := "narrow-follower-"+suffix, "narrow-former-"+suffix
	pathID, activityID := "narrow-path-"+suffix, "narrow-activity-"+suffix
	eventID, commentID := "practice:"+activityID, "narrow-comment-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	for index, user := range []string{owner, member, follower, formerFollower} {
		seedInvitationUser(t, migrationDB, user, fmt.Sprintf("narrow%d.%s", index, suffix), identity.ProfileVisibilityPublic, now)
	}
	existing := domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Narrow", Visibility: "public"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	seedGoalUpdatePath(t, migrationDB, existing, owner)
	if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: member, Role: "participant", JoinedAt: now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("follow_models").Create(map[string]any{"follower_user_id": follower, "following_user_id": owner, "created_at": now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{"id": activityID, "path_id": pathID, "participant_id": owner, "started_at": now.Add(-time.Hour), "ended_at": now.Add(-time.Minute), "occurrence_time_zone": "Etc/UTC", "created_at": now.Add(-time.Minute), "updated_at": now.Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("social_practice_comment_models").Create(map[string]any{"id": commentID, "social_feed_event_id": eventID, "author_user_id": formerFollower, "body": "Earlier comment", "version": 1, "created_at": now.Add(-time.Minute), "updated_at": now.Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"lostNudge": "narrow-nudge-" + suffix, "lostHeart": "narrow-heart-" + suffix, "keptOwner": "narrow-owner-notice-" + suffix, "keptMember": "narrow-member-notice-" + suffix, "keptFollower": "narrow-follower-notice-" + suffix, "explanation": "narrow-explain-" + suffix}
	nudges := map[string]string{"lostNudge": "narrow-nudge-record-" + suffix, "keptMember": "narrow-member-record-" + suffix, "keptFollower": "narrow-follower-record-" + suffix}
	future := now.Add(5 * time.Second)
	disabledAt, disabledReason := now.Add(-500*time.Millisecond), "comments"
	for key, nudgeID := range nudges {
		recipient, sentAt := formerFollower, now.Add(-time.Second)
		if key == "lostNudge" {
			sentAt = future
		}
		if key == "keptMember" {
			recipient = member
		}
		if key == "keptFollower" {
			recipient = follower
		}
		if err := migrationDB.Table("social_nudge_models").Create(map[string]any{"id": nudgeID, "sender_user_id": owner, "recipient_user_id": recipient, "path_id": pathID, "content_kind": "preset", "preset": "keep_it_going", "sent_at": sentAt}).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows := []map[string]any{
		{"id": ids["lostNudge"], "recipient_user_id": formerFollower, "actor_user_id": owner, "path_id": pathID, "nudge_id": nudges["lostNudge"], "kind": "nudge_received", "presentation_class": "informational", "channel": "nudges", "created_at": future},
		{"id": ids["lostHeart"], "recipient_user_id": formerFollower, "actor_user_id": member, "path_id": pathID, "social_feed_event_id": eventID, "comment_id": commentID, "kind": "comment_heart", "presentation_class": "informational", "channel": "comment_hearts", "created_at": now.Add(-time.Second), "deleted_at": disabledAt, "interaction_disabled_reason": disabledReason},
		{"id": ids["keptOwner"], "recipient_user_id": owner, "actor_user_id": member, "path_id": pathID, "social_feed_event_id": eventID, "reaction_type": "heart", "kind": "practice_reaction", "presentation_class": "informational", "channel": "reactions", "created_at": now.Add(-time.Second)},
		{"id": ids["keptMember"], "recipient_user_id": member, "actor_user_id": owner, "path_id": pathID, "nudge_id": nudges["keptMember"], "kind": "nudge_received", "presentation_class": "informational", "channel": "nudges", "created_at": now.Add(-time.Second)},
		{"id": ids["keptFollower"], "recipient_user_id": follower, "actor_user_id": owner, "path_id": pathID, "nudge_id": nudges["keptFollower"], "kind": "nudge_received", "presentation_class": "informational", "channel": "nudges", "created_at": now.Add(-time.Second)},
		{"id": ids["explanation"], "recipient_user_id": formerFollower, "actor_user_id": owner, "path_id": pathID, "path_visibility": "followers", "kind": "path_visibility_changed", "presentation_class": "informational", "channel": "path_access", "created_at": now.Add(-time.Second)},
	}
	for _, row := range rows {
		if err := migrationDB.Table("notification_models").Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, user := range []string{member, follower, formerFollower} {
		hash := sha256.Sum256([]byte(user))
		if err := migrationDB.Table("push_installation_models").Create(map[string]any{"id": "narrow-install-" + user, "owner_user_id": user, "provider": "expo", "platform": "ios", "locale": "en", "token_ciphertext": bytes.Repeat([]byte{1}, 17), "token_nonce": bytes.Repeat([]byte{2}, 12), "token_hash": hash[:], "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for key, id := range ids {
		if key != "explanation" {
			if key == "keptOwner" {
				continue
			}
			recipient, createdAt := formerFollower, now
			if key == "lostNudge" {
				createdAt = future
			}
			if key == "keptMember" {
				recipient = member
			}
			if key == "keptFollower" {
				recipient = follower
			}
			if err := createNotificationPushDelivery(migrationDB, id, recipient, createdAt); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := migrationDB.Table("notification_push_delivery_models").Where("notification_id = ?", ids["lostHeart"]).Updates(map[string]any{"delivered_at": now, "token_ciphertext": nil, "token_nonce": nil, "token_hash": nil}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_outbox_models").Where("notification_id = ?", ids["lostHeart"]).Update("delivered_at", now).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupInvitationFixture(t, migrationDB, []string{owner, member, follower, formerFollower}, []string{pathID})
	})

	changed, _ := existing.SetVisibility("followers", now)
	sequence := 0
	newID := func() string { sequence++; return fmt.Sprintf("narrow-id-%s-%d", suffix, sequence) }
	command := visibilityCommand(owner, "public", changed, "narrow-key-0000001", bytes.Repeat([]byte{4}, 32), "narrow-audit-"+suffix, now, newID)
	rollback := command
	rollback.Idempotency.Key = "narrow-key-rollback"
	rollback.Audit.ID = "narrow-audit-rollback-" + suffix
	rollback.Audit.CorrelationID = rollback.Audit.ID
	if err := migrationDB.Create(fromAudit(rollback.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := New(runtimeDB).SetVisibility(context.Background(), rollback); err == nil {
		t.Fatal("duplicate audit rollback unexpectedly succeeded")
	}
	assertVisibilityNoticeState(t, migrationDB, ids, true)

	blocker := migrationDB.Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	defer blocker.Rollback()
	if err := progresslock.LockKey(blocker, sociallock.PathAudienceKey(pathID)); err != nil {
		t.Fatal(err)
	}
	completed := make(chan error, 1)
	go func() { _, err := New(runtimeDB).SetVisibility(context.Background(), command); completed <- err }()
	waitForVisibilityAudienceWaiter(t, migrationDB, pathID)
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	assertVisibilityNoticeState(t, migrationDB, ids, false)
	for _, id := range []string{ids["lostNudge"], ids["lostHeart"]} {
		if _, err := New(runtimeDB).GetNotification(context.Background(), formerFollower, id); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("GetNotification(%s)=%v", id, err)
		}
	}
	replay, err := New(runtimeDB).SetVisibility(context.Background(), command)
	if err != nil || !replay.Replayed {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
}

func assertVisibilityNoticeState(t *testing.T, db *gorm.DB, ids map[string]string, before bool) {
	t.Helper()
	var lost, disabled, kept, explanation, pendingSuppressed, outboxSuppressed, terminalRetained int64
	_ = db.Table("notification_models").Where("id IN ? AND (deleted_at IS NULL OR interaction_disabled_reason IS NOT NULL)", []string{ids["lostNudge"], ids["lostHeart"]}).Count(&lost).Error
	_ = db.Table("notification_models").Where("id = ? AND interaction_disabled_reason IS NOT NULL", ids["lostHeart"]).Count(&disabled).Error
	_ = db.Table("notification_models").Where("id IN ? AND deleted_at IS NULL", []string{ids["keptOwner"], ids["keptMember"], ids["keptFollower"]}).Count(&kept).Error
	_ = db.Table("notification_models").Where("id = ? AND deleted_at IS NULL", ids["explanation"]).Count(&explanation).Error
	pendingIDs := []string{ids["lostNudge"]}
	_ = db.Table("notification_push_delivery_models").Where("notification_id IN ? AND suppressed_at IS NOT NULL AND token_ciphertext IS NULL", pendingIDs).Count(&pendingSuppressed).Error
	_ = db.Table("notification_push_outbox_models").Where("notification_id IN ? AND suppressed_at IS NOT NULL AND failure_code = ?", pendingIDs, pathAccessRevokedPushFailureCode).Count(&outboxSuppressed).Error
	_ = db.Table("notification_push_delivery_models").Where("notification_id = ? AND delivered_at IS NOT NULL AND suppressed_at IS NULL", ids["lostHeart"]).Count(&terminalRetained).Error
	wantLost, wantDisabled, wantSuppressed := int64(0), int64(0), int64(1)
	if before {
		wantLost, wantDisabled, wantSuppressed = 2, 1, 0
	}
	if lost != wantLost || disabled != wantDisabled || kept != 3 || explanation != 1 || pendingSuppressed != wantSuppressed || outboxSuppressed != wantSuppressed || terminalRetained != 1 {
		t.Fatalf("state lost=%d disabled=%d kept=%d explain=%d pending=%d outbox=%d terminal=%d", lost, disabled, kept, explanation, pendingSuppressed, outboxSuppressed, terminalRetained)
	}
}

func waitForVisibilityAudienceWaiter(t *testing.T, db *gorm.DB, pathID string) {
	t.Helper()
	key := sociallock.PathAudienceKey(pathID)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int64
		if err := db.Table("pg_locks").Where(`locktype='advisory' AND NOT granted AND classid=((hashtextextended(?,0)>>32)&4294967295)::oid AND objid=(hashtextextended(?,0)&4294967295)::oid AND objsubid=1`, key, key).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("visibility change did not wait for path-audience lock")
}
