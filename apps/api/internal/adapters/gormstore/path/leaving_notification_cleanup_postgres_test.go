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

func TestPostgresLeavePathRetiresNotificationsOnlyWhenPostLeaveAccessIsLost(t *testing.T) {
	for _, test := range []struct {
		name, visibility string
		followsOwner     bool
		blockedByOwner   bool
		retainActivity   bool
		losesAccess      bool
		proveRollback    bool
	}{
		{name: "private retained activity", visibility: "private", retainActivity: true, losesAccess: true, proveRollback: true},
		{name: "followers without follow deleted activity", visibility: "followers", retainActivity: false, losesAccess: true},
		{name: "followers with follow retained activity", visibility: "followers", followsOwner: true, retainActivity: true},
		{name: "public deleted activity", visibility: "public", retainActivity: false},
		{name: "public blocked by owner", visibility: "public", blockedByOwner: true, retainActivity: true, losesAccess: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newLeaveNotificationFixture(t, test.visibility, test.followsOwner, test.blockedByOwner)
			command := leaveNotificationTestCommand(fixture.participant, fixture.pathID, fixture.suffix, fixture.now, test.retainActivity)
			if test.proveRollback {
				rollback := command
				rollback.Idempotency.Key = "leave-notification-rollback-" + fixture.suffix
				rollback.Idempotency.RequestHash = bytes.Repeat([]byte{9}, sha256.Size)
				rollback.Audit.ID = "leave-notification-rollback-audit-" + fixture.suffix
				if err := fixture.migrationDB.Create(fromAudit(rollback.Audit)).Error; err != nil {
					t.Fatal(err)
				}
				if _, err := fixture.repository.LeavePath(context.Background(), rollback); err == nil {
					t.Fatal("LeavePath() with duplicate audit unexpectedly succeeded")
				}
				fixture.assertState(t, false, true, 0)
			}

			result, err := fixture.repository.LeavePath(context.Background(), command)
			if err != nil || !result.Left || result.Replayed || result.ActivityRetained != test.retainActivity {
				t.Fatalf("LeavePath()=%+v err=%v", result, err)
			}
			wantActivity := test.retainActivity
			fixture.assertState(t, test.losesAccess, wantActivity, 2)
			_, err = fixture.repository.GetNotification(context.Background(), fixture.participant, fixture.targetNotice)
			if test.losesAccess && !errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("GetNotification(retired) error=%v", err)
			}
			if !test.losesAccess && err != nil {
				t.Fatalf("GetNotification(retained) error=%v", err)
			}
			if _, err := fixture.repository.GetNotification(context.Background(), fixture.participant, fixture.otherNotice); err != nil {
				t.Fatalf("GetNotification(unrelated) error=%v", err)
			}
			page, err := fixture.repository.ListNotifications(context.Background(), fixture.participant, application.NotificationPageRequest{Limit: 25, Snapshot: fixture.now.Add(time.Second)})
			wantUnread := int64(2)
			if test.losesAccess {
				wantUnread = 1
			}
			if err != nil || page.UnreadCount != wantUnread || int64(len(page.Items)) != wantUnread {
				t.Fatalf("ListNotifications()=%+v err=%v", page, err)
			}

			replay, err := fixture.repository.LeavePath(context.Background(), command)
			if err != nil || !replay.Replayed || replay.ActivityRetained != test.retainActivity {
				t.Fatalf("LeavePath(replay)=%+v err=%v", replay, err)
			}
			fixture.assertState(t, test.losesAccess, wantActivity, 2)
		})
	}
}

type leaveNotificationFixture struct {
	repository                        *Repository
	migrationDB                       *gorm.DB
	suffix, owner, administrator      string
	participant, pathID, targetNotice string
	otherPathID, otherNotice          string
	now                               time.Time
}

func newLeaveNotificationFixture(t *testing.T, visibility string, followsOwner, blockedByOwner bool) leaveNotificationFixture {
	t.Helper()
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%s-%t-%d", visibility, followsOwner, time.Now().UnixNano())
	fixture := leaveNotificationFixture{
		repository: New(runtimeDB), migrationDB: migrationDB, suffix: suffix,
		owner: "leave-notice-owner-" + suffix, administrator: "leave-notice-admin-" + suffix,
		participant: "leave-notice-participant-" + suffix, pathID: "leave-notice-path-" + suffix,
		otherPathID: "leave-notice-other-" + suffix, targetNotice: "leave-notice-target-" + suffix,
		otherNotice: "leave-notice-retained-" + suffix, now: time.Now().UTC().Truncate(time.Microsecond),
	}
	users := []string{fixture.owner, fixture.administrator, fixture.participant}
	paths := []string{fixture.pathID, fixture.otherPathID}
	seedGoalUpdateUsers(t, migrationDB, fixture.now, users...)
	for index, userID := range users {
		username := fmt.Sprintf("ln%d%d", index, time.Now().UnixNano())
		if err := migrationDB.Table("user_models").Where("id = ?", userID).Updates(map[string]any{"username": username, "display_name": username}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, entity := range []domain.Entity{
		{ID: domain.ID(fixture.pathID), OwnerUserID: fixture.owner, Attributes: domain.Attributes{Name: "Leave notification", Visibility: visibility}, CreatedAt: fixture.now.Add(-time.Hour), UpdatedAt: fixture.now.Add(-time.Hour)},
		{ID: domain.ID(fixture.otherPathID), OwnerUserID: fixture.owner, Attributes: domain.Attributes{Name: "Unrelated notification", Visibility: "private"}, CreatedAt: fixture.now.Add(-time.Hour), UpdatedAt: fixture.now.Add(-time.Hour)},
	} {
		seedGoalUpdatePath(t, migrationDB, entity, fixture.owner)
	}
	if err := migrationDB.Create(&[]membershipModel{
		{PathID: fixture.pathID, UserID: fixture.administrator, Role: string(domain.RoleAdministrator), JoinedAt: fixture.now.Add(-time.Hour)},
		{PathID: fixture.pathID, UserID: fixture.participant, Role: string(domain.RoleParticipant), JoinedAt: fixture.now.Add(-time.Hour)},
		{PathID: fixture.otherPathID, UserID: fixture.participant, Role: string(domain.RoleParticipant), JoinedAt: fixture.now.Add(-time.Hour)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if followsOwner {
		if err := migrationDB.Table("follow_models").Create(map[string]any{"follower_user_id": fixture.participant, "following_user_id": fixture.owner, "created_at": fixture.now.Add(-time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if blockedByOwner {
		if err := migrationDB.Table("block_models").Create(map[string]any{"blocker_user_id": fixture.owner, "blocked_user_id": fixture.participant, "created_at": fixture.now.Add(-time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	tokenHash := sha256.Sum256([]byte("leave-notice-token-" + suffix))
	if err := migrationDB.Table("push_installation_models").Create(map[string]any{
		"id": "leave-notice-installation-" + suffix, "owner_user_id": fixture.participant, "provider": "expo", "platform": "ios", "locale": "en",
		"token_ciphertext": bytes.Repeat([]byte{7}, 17), "token_nonce": bytes.Repeat([]byte{8}, 12), "token_hash": tokenHash[:],
		"created_at": fixture.now.Add(-time.Minute), "updated_at": fixture.now.Add(-time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupDeletionFixture(migrationDB, users, paths) })
	if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{"id": "leave-notice-activity-" + suffix, "path_id": fixture.pathID, "participant_id": fixture.participant, "started_at": fixture.now.Add(-time.Minute), "ended_at": fixture.now, "occurrence_time_zone": "Etc/UTC", "created_at": fixture.now, "updated_at": fixture.now}).Error; err != nil {
		t.Fatal(err)
	}
	for _, nudge := range []struct{ id, pathID, noticeID, sender string }{
		{"leave-notice-nudge-" + suffix, fixture.pathID, fixture.targetNotice, fixture.owner},
		{"leave-notice-other-nudge-" + suffix, fixture.otherPathID, fixture.otherNotice, fixture.administrator},
	} {
		createdAt := fixture.now.Add(-10 * time.Second)
		if err := migrationDB.Table("social_nudge_models").Create(map[string]any{"id": nudge.id, "sender_user_id": nudge.sender, "recipient_user_id": fixture.participant, "path_id": nudge.pathID, "content_kind": "preset", "preset": "keep_it_going", "sent_at": createdAt}).Error; err != nil {
			t.Fatal(err)
		}
		seedNudgeNotification(t, migrationDB, notificationModel{ID: nudge.noticeID, RecipientUserID: fixture.participant, ActorUserID: nudge.sender, PathID: nudge.pathID, NudgeID: nudge.id, Kind: nudgeReceivedNotificationKind, PresentationClass: "informational", Channel: "nudges", CreatedAt: createdAt})
		if err := createNotificationPushDelivery(migrationDB, nudge.noticeID, fixture.participant, createdAt); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func leaveNotificationTestCommand(participant, pathID, suffix string, now time.Time, retainActivity bool) application.LeavePathCommand {
	sequence := 0
	return application.LeavePathCommand{
		ActorUserID: participant, PathID: domain.ID(pathID), LeftAt: now, RetainActivity: retainActivity,
		Idempotency: ports.Idempotency{PrincipalID: participant, Operation: application.LeavePathOperation, Key: "leave-notice-key-" + suffix, RequestHash: bytes.Repeat([]byte{4}, 32)},
		Audit:       audit.Event{ID: "leave-notice-audit-" + suffix, OwnerUserID: participant, ActorUserID: participant, Action: audit.ResourceUpdated, TargetType: "path", TargetID: pathID, Outcome: audit.Succeeded, CorrelationID: "leave-notice-" + suffix, OccurredAt: now},
		NewID:       func() string { sequence++; return fmt.Sprintf("leave-notice-id-%s-%d", suffix, sequence) }, AuthorizationWorker: "leave-notice-test", AuthorizationLease: time.Minute,
	}
}

func (fixture leaveNotificationFixture) assertState(t *testing.T, targetRetired, activityRetained bool, managerNotices int64) {
	t.Helper()
	var membership, activity, targetActive, targetPending, otherActive, otherPending, notices int64
	checks := []struct {
		to           *int64
		table, where string
		args         []any
	}{
		{&membership, "path_membership_models", "path_id = ? AND user_id = ?", []any{fixture.pathID, fixture.participant}},
		{&activity, "recorded_activity_models", "path_id = ? AND participant_id = ?", []any{fixture.pathID, fixture.participant}},
		{&targetActive, "notification_models", "id = ? AND deleted_at IS NULL", []any{fixture.targetNotice}},
		{&targetPending, "notification_push_outbox_models", "notification_id = ? AND suppressed_at IS NULL", []any{fixture.targetNotice}},
		{&otherActive, "notification_models", "id = ? AND deleted_at IS NULL", []any{fixture.otherNotice}},
		{&otherPending, "notification_push_outbox_models", "notification_id = ? AND suppressed_at IS NULL", []any{fixture.otherNotice}},
		{&notices, "notification_models", "path_id = ? AND kind = ? AND recipient_user_id IN ?", []any{fixture.pathID, application.NotificationPathMemberLeft, []string{fixture.owner, fixture.administrator}}},
	}
	for _, check := range checks {
		if err := fixture.migrationDB.Table(check.table).Where(check.where, check.args...).Count(check.to).Error; err != nil {
			t.Fatal(err)
		}
	}
	wantMembership, wantActivity, wantTarget := int64(0), int64(0), int64(1)
	if managerNotices == 0 {
		wantMembership = 1
	}
	if activityRetained {
		wantActivity = 1
	}
	if targetRetired {
		wantTarget = 0
	}
	if membership != wantMembership || activity != wantActivity || targetActive != wantTarget || targetPending != wantTarget || otherActive != 1 || otherPending != 1 || notices != managerNotices {
		t.Fatalf("state membership=%d activity=%d target=%d targetPush=%d other=%d otherPush=%d notices=%d", membership, activity, targetActive, targetPending, otherActive, otherPending, notices)
	}
	if targetRetired {
		for table, extra := range map[string]string{
			"notification_push_delivery_models": " AND token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL",
			"notification_push_outbox_models":   "",
		} {
			var suppressed int64
			where := "notification_id = ? AND suppressed_at = ? AND failure_code = ?" + extra
			if err := fixture.migrationDB.Table(table).Where(where, fixture.targetNotice, fixture.now, pathAccessRevokedPushFailureCode).Count(&suppressed).Error; err != nil || suppressed != 1 {
				t.Fatalf("%s suppressed=%d err=%v", table, suppressed, err)
			}
		}
	}
}
