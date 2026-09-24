package pathstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPathTargetNotificationKindsAreAnExplicitAllowlist(t *testing.T) {
	for kind, want := range map[string]bool{
		"nudge_received":           true,
		"path_member_removed":      false,
		"path_member_role_changed": false,
		"path_visibility_changed":  false,
		"practice_comment":         false,
	} {
		if got := targetOpeningPathNotificationKind(kind); got != want {
			t.Fatalf("targetOpeningPathNotificationKind(%q)=%t want %t", kind, got, want)
		}
	}
}

func TestPostgresRemovePrivatePathSupporterRetiresOnlyTargetOpeningNotificationsAtomically(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, supporter := "notice-owner-"+suffix, "notice-supporter-"+suffix
	pathID, otherPathID := "notice-path-"+suffix, "notice-other-"+suffix
	targetNotice, otherNotice := "notice-target-"+suffix, "notice-retained-"+suffix
	targetNudge, otherNudge := "nudge-target-"+suffix, "nudge-retained-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	tokenHash := sha256.Sum256([]byte("notice-token-" + suffix))

	seedGoalUpdateUsers(t, migrationDB, now, owner, supporter)
	for userID, username := range map[string]string{owner: "no" + suffix, supporter: "ns" + suffix} {
		if err := migrationDB.Table("user_models").Where("id = ?", userID).Updates(map[string]any{"username": username, "display_name": username}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, entity := range []domain.Entity{
		{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Removed private path", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
		{ID: domain.ID(otherPathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Retained private path", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	} {
		seedGoalUpdatePath(t, migrationDB, entity, owner)
	}
	if err := migrationDB.Create(&[]membershipModel{
		{PathID: pathID, UserID: supporter, Role: string(domain.RoleSupporter), JoinedAt: now.Add(-time.Hour)},
		{PathID: otherPathID, UserID: supporter, Role: string(domain.RoleSupporter), JoinedAt: now.Add(-time.Hour)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("push_installation_models").Create(map[string]any{
		"id": "notice-installation-" + suffix, "owner_user_id": supporter, "provider": "expo", "platform": "ios", "locale": "en",
		"token_ciphertext": bytes.Repeat([]byte{7}, 17), "token_nonce": bytes.Repeat([]byte{8}, 12), "token_hash": tokenHash[:],
		"created_at": now.Add(-time.Minute), "updated_at": now.Add(-time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupDeletionFixture(migrationDB, []string{owner, supporter}, []string{pathID, otherPathID}) })

	for _, nudge := range []map[string]any{
		{"id": targetNudge, "sender_user_id": owner, "recipient_user_id": supporter, "path_id": pathID, "content_kind": "preset", "preset": "keep_it_going", "sent_at": now.Add(-20 * time.Second)},
		{"id": otherNudge, "sender_user_id": owner, "recipient_user_id": supporter, "path_id": otherPathID, "content_kind": "preset", "preset": "keep_it_going", "sent_at": now.Add(-10 * time.Second)},
	} {
		if err := migrationDB.Table("social_nudge_models").Create(nudge).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, notice := range []notificationModel{
		{ID: targetNotice, RecipientUserID: supporter, ActorUserID: owner, PathID: pathID, NudgeID: targetNudge, Kind: "nudge_received", PresentationClass: "informational", Channel: "nudges", CreatedAt: now.Add(-20 * time.Second)},
		{ID: otherNotice, RecipientUserID: supporter, ActorUserID: owner, PathID: otherPathID, NudgeID: otherNudge, Kind: "nudge_received", PresentationClass: "informational", Channel: "nudges", CreatedAt: now.Add(-10 * time.Second)},
	} {
		seedNudgeNotification(t, migrationDB, notice)
		if err := createNotificationPushDelivery(migrationDB, notice.ID, supporter, notice.CreatedAt); err != nil {
			t.Fatal(err)
		}
	}

	rollbackCommand := removalNotificationTestCommand(owner, supporter, pathID, suffix, now)
	if err := migrationDB.Create(fromAudit(rollbackCommand.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := New(runtimeDB).RemoveMember(context.Background(), rollbackCommand); err == nil {
		t.Fatal("RemoveMember() with duplicate audit unexpectedly succeeded")
	}
	assertRemovalNotificationState(t, migrationDB, pathID, supporter, targetNotice, otherNotice, now, false)

	command := removalNotificationTestCommand(owner, supporter, pathID, suffix+"-success", now)
	repository := New(runtimeDB)
	result, err := repository.RemoveMember(context.Background(), command)
	if err != nil || !result.Removed || result.Replayed {
		t.Fatalf("RemoveMember()=%+v err=%v", result, err)
	}
	assertRemovalNotificationState(t, migrationDB, pathID, supporter, targetNotice, otherNotice, now, true)
	if _, err := repository.GetNotification(context.Background(), supporter, targetNotice); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetNotification(retired) error=%v", err)
	}
	if _, err := repository.GetNotification(context.Background(), supporter, otherNotice); err != nil {
		t.Fatalf("GetNotification(retained) error=%v", err)
	}
	page, err := repository.ListNotifications(context.Background(), supporter, application.NotificationPageRequest{Limit: 25, Snapshot: now.Add(time.Second)})
	if err != nil || page.UnreadCount != 2 || len(page.Items) != 2 {
		t.Fatalf("ListNotifications()=%+v err=%v", page, err)
	}
	listed := make(map[string]bool, len(page.Items))
	for _, item := range page.Items {
		listed[item.ID] = true
		if item.ID == targetNotice {
			t.Fatalf("retired notification remained in history: %+v", item)
		}
	}
	if !listed[otherNotice] || !listed[command.Notification.ID] {
		t.Fatalf("history did not retain unrelated nudge and removal explanation: %+v", page.Items)
	}
	replay, err := repository.RemoveMember(context.Background(), command)
	if err != nil || !replay.Replayed {
		t.Fatalf("RemoveMember(replay)=%+v err=%v", replay, err)
	}
	assertRemovalNotificationState(t, migrationDB, pathID, supporter, targetNotice, otherNotice, now, true)
}

func seedNudgeNotification(t *testing.T, db *gorm.DB, notice notificationModel) {
	t.Helper()
	if err := db.Table("notification_models").Create(map[string]any{
		"id": notice.ID, "recipient_user_id": notice.RecipientUserID, "actor_user_id": notice.ActorUserID,
		"path_id": notice.PathID, "nudge_id": notice.NudgeID, "kind": notice.Kind,
		"presentation_class": notice.PresentationClass, "channel": notice.Channel, "created_at": notice.CreatedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func removalNotificationTestCommand(owner, supporter, pathID, suffix string, now time.Time) application.RemoveMemberCommand {
	sequence := 0
	return application.RemoveMemberCommand{
		ActorUserID: owner, TargetUserID: supporter, PathID: domain.ID(pathID), ExpectedRole: domain.RoleSupporter, RemovedAt: now,
		Idempotency:  ports.Idempotency{PrincipalID: owner, Operation: application.RemoveMemberOperation, Key: "notice-remove-key-" + suffix, RequestHash: bytes.Repeat([]byte{6}, 32)},
		Audit:        audit.Event{ID: "notice-remove-audit-" + suffix, OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceUpdated, TargetType: "path_member", TargetID: pathID + ":" + supporter, Outcome: audit.Succeeded, CorrelationID: "notice-remove-" + suffix, OccurredAt: now},
		Notification: application.MemberAccessNotification{ID: "notice-removal-" + suffix, RecipientUserID: supporter, ActorUserID: owner, PathID: domain.ID(pathID), Kind: application.NotificationPathMemberRemoved, Role: domain.RoleSupporter, CreatedAt: now},
		NewID:        func() string { sequence++; return fmt.Sprintf("notice-change-%s-%d", suffix, sequence) }, AuthorizationWorker: "notice-removal-test", AuthorizationLease: time.Minute,
	}
}

func assertRemovalNotificationState(t *testing.T, db *gorm.DB, pathID, supporter, targetNotice, otherNotice string, removedAt time.Time, removed bool) {
	t.Helper()
	var membershipCount, targetActive, otherActive, targetPending, otherPending int64
	checks := []struct {
		destination  *int64
		table, where string
		args         []any
	}{
		{&membershipCount, "path_membership_models", "path_id = ? AND user_id = ?", []any{pathID, supporter}},
		{&targetActive, "notification_models", "id = ? AND deleted_at IS NULL", []any{targetNotice}},
		{&otherActive, "notification_models", "id = ? AND deleted_at IS NULL", []any{otherNotice}},
		{&targetPending, "notification_push_outbox_models", "notification_id = ? AND suppressed_at IS NULL", []any{targetNotice}},
		{&otherPending, "notification_push_outbox_models", "notification_id = ? AND suppressed_at IS NULL", []any{otherNotice}},
	}
	for _, check := range checks {
		if err := db.Table(check.table).Where(check.where, check.args...).Count(check.destination).Error; err != nil {
			t.Fatal(err)
		}
	}
	wantMembership, wantTargetActive, wantTargetPending := int64(1), int64(1), int64(1)
	if removed {
		wantMembership, wantTargetActive, wantTargetPending = 0, 0, 0
	}
	if membershipCount != wantMembership || targetActive != wantTargetActive || targetPending != wantTargetPending || otherActive != 1 || otherPending != 1 {
		t.Fatalf("state membership=%d targetActive=%d targetPending=%d otherActive=%d otherPending=%d", membershipCount, targetActive, targetPending, otherActive, otherPending)
	}
	if removed {
		var deleted notificationModel
		if err := db.Where("id = ?", targetNotice).Take(&deleted).Error; err != nil || deleted.DeletedAt == nil || !deleted.DeletedAt.Equal(removedAt) {
			t.Fatalf("retired notification=%+v err=%v", deleted, err)
		}
		var suppressedDeliveries, suppressedOutboxes int64
		if err := db.Table("notification_push_delivery_models").Where("notification_id = ? AND suppressed_at = ? AND failure_code = ? AND token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL", targetNotice, removedAt, "path_access_revoked").Count(&suppressedDeliveries).Error; err != nil || suppressedDeliveries != 1 {
			t.Fatalf("suppressed deliveries=%d err=%v", suppressedDeliveries, err)
		}
		if err := db.Table("notification_push_outbox_models").Where("notification_id = ? AND suppressed_at = ? AND failure_code = ?", targetNotice, removedAt, "path_access_revoked").Count(&suppressedOutboxes).Error; err != nil || suppressedOutboxes != 1 {
			t.Fatalf("suppressed outboxes=%d err=%v", suppressedOutboxes, err)
		}
	}
}
