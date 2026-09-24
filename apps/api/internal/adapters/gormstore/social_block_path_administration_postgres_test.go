package gormstore

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPostgresBlockedSharedPathMemberRemainsManageableWithoutUnblocking(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeStore, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, runtimeStore)
	closeSocialProfileTestStore(t, migrationStore)

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	manager := socialRelationshipTestUser(t, migrationStore.DB, "blockedmanager", identity.ProfileVisibilityPublic, now)
	member := socialRelationshipTestUser(t, migrationStore.DB, "blockedmember", identity.ProfileVisibilityPublic, now)
	ordinary := socialRelationshipTestUser(t, migrationStore.DB, "ordinarymember", identity.ProfileVisibilityPublic, now)
	pathID, otherPathID := "blocked-admin-path-"+newTestID(), "blocked-other-path-"+newTestID()
	activityID, otherActivityID := "blocked-admin-activity-"+newTestID(), "blocked-other-activity-"+newTestID()
	t.Cleanup(func() {
		users := []string{manager.ID, member.ID, ordinary.ID}
		paths := []string{pathID, otherPathID}
		_ = migrationStore.DB.Table("authorization_outbox_models").Where("resource_type = ? AND resource_id IN ?", "path", paths).Delete(map[string]any{}).Error
		_ = migrationStore.DB.Table("social_block_replay_models").Where("actor_user_id IN ?", users).Delete(map[string]any{}).Error
		_ = migrationStore.DB.Table("idempotency_models").Where("principal_id IN ?", users).Delete(map[string]any{}).Error
		_ = migrationStore.DB.Table("audit_event_models").Where("owner_user_id IN ?", users).Delete(map[string]any{}).Error
		_ = migrationStore.DB.Table("block_models").Where("blocker_user_id IN ? OR blocked_user_id IN ?", users, users).Delete(map[string]any{}).Error
		_ = migrationStore.DB.Table("path_models").Where("id IN ?", paths).Delete(map[string]any{}).Error
		_ = migrationStore.DB.Table("user_models").Where("id IN ?", users).Delete(map[string]any{}).Error
	})

	for _, path := range []map[string]any{
		{"id": pathID, "owner_user_id": manager.ID, "name": "Shared Piano", "visibility": "private", "created_at": now.Add(-time.Hour), "updated_at": now},
		{"id": otherPathID, "owner_user_id": ordinary.ID, "name": "Unrelated Reading", "visibility": "private", "created_at": now.Add(-time.Hour), "updated_at": now},
	} {
		if err := migrationStore.DB.Table("path_models").Create(path).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, membership := range []map[string]any{
		{"path_id": pathID, "user_id": manager.ID, "role": "participant", "joined_at": now.Add(-time.Hour)},
		{"path_id": pathID, "user_id": member.ID, "role": "participant", "joined_at": now.Add(-time.Hour)},
		{"path_id": pathID, "user_id": ordinary.ID, "role": "participant", "joined_at": now.Add(-time.Hour)},
		{"path_id": otherPathID, "user_id": ordinary.ID, "role": "participant", "joined_at": now.Add(-time.Hour)},
		{"path_id": otherPathID, "user_id": member.ID, "role": "participant", "joined_at": now.Add(-time.Hour)},
	} {
		if err := migrationStore.DB.Table("path_membership_models").Create(membership).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, activity := range []map[string]any{
		{"id": activityID, "path_id": pathID, "participant_id": member.ID, "started_at": now.Add(-30 * time.Minute), "ended_at": now.Add(-20 * time.Minute), "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now},
		{"id": otherActivityID, "path_id": otherPathID, "participant_id": member.ID, "started_at": now.Add(-15 * time.Minute), "ended_at": now.Add(-5 * time.Minute), "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now},
	} {
		if err := migrationStore.DB.Table("recorded_activity_models").Create(activity).Error; err != nil {
			t.Fatal(err)
		}
	}

	socialRepository := NewSocialRelationshipRepository(runtimeStore.DB, newTestID, "blocked-admin-social", time.Minute)
	block := socialBlockTestCommand(manager.ID, socialapp.BlockOperation, *member.Username, member.ID, "blocked-admin-block-key", now.Add(time.Second))
	block.ExpectedSharedPathIDs = []string{pathID}
	if result, err := socialRepository.Block(ctx, block); err != nil || !result.Blocked {
		t.Fatalf("Block()=%+v err=%v", result, err)
	}
	if _, err := NewSocialProfileRepository(runtimeStore.DB).GetByUsername(ctx, manager.ID, *member.Username); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("blocked profile error=%v, want opaque not found", err)
	}

	pathRepository := pathstore.New(runtimeStore.DB)
	page, err := pathRepository.ListMembers(ctx, pathapp.MemberListQuery{ActorUserID: manager.ID, PathID: pathdomain.ID(pathID)}, pathapp.MemberPageRequest{Limit: 25, Snapshot: now.Add(2 * time.Second)})
	if err != nil || !memberPageContains(page, member.ID, "participant") {
		t.Fatalf("blocked member roster=%+v err=%v", page, err)
	}
	review, err := pathRepository.ReviewMemberRemoval(ctx, pathapp.MemberRemovalQuery{ActorUserID: manager.ID, TargetUserID: member.ID, PathID: pathdomain.ID(pathID)})
	if err != nil || review.UserID != member.ID || review.SessionCount != 1 || review.TotalTrackedSeconds != 600 {
		t.Fatalf("blocked member review=%+v err=%v", review, err)
	}

	denied := blockedMemberRemovalCommand(ordinary.ID, member.ID, pathID, now.Add(2*time.Second), "blocked-admin-denied-key")
	if _, err := pathRepository.RemoveMember(ctx, denied); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("ordinary member removal error=%v, want opaque not found", err)
	}
	assertBlockedMemberPathRows(t, migrationStore.DB, pathID, member.ID, 1, 1)

	command := blockedMemberRemovalCommand(manager.ID, member.ID, pathID, now.Add(3*time.Second), "blocked-admin-remove-key")
	result, err := pathRepository.RemoveMember(ctx, command)
	if err != nil || !result.Removed || !result.ActivityDeleted {
		t.Fatalf("RemoveMember()=%+v err=%v", result, err)
	}
	assertBlockedMemberPathRows(t, migrationStore.DB, pathID, member.ID, 0, 0)
	assertBlockedMemberPathRows(t, migrationStore.DB, otherPathID, member.ID, 1, 1)
	var removalNotices, removalPushes, unrelatedNotices int64
	if err := migrationStore.DB.Table("notification_models").Where("id = ? AND recipient_user_id = ? AND actor_user_id = ? AND path_id = ? AND kind = ? AND presentation_class = ? AND channel = ? AND offered_role = ?", command.Notification.ID, member.ID, manager.ID, pathID, pathapp.NotificationPathMemberRemoved, pathapp.NotificationInformational, "path_access", pathdomain.RoleParticipant).Count(&removalNotices).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("notification_push_outbox_models").Where("notification_id = ?", command.Notification.ID).Count(&removalPushes).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("notification_models").Where("recipient_user_id = ? AND path_id = ? AND kind = ?", member.ID, otherPathID, pathapp.NotificationPathMemberRemoved).Count(&unrelatedNotices).Error; err != nil {
		t.Fatal(err)
	}
	if removalNotices != 1 || removalPushes != 1 || unrelatedNotices != 0 {
		t.Fatalf("blocked removal notices=%d pushes=%d unrelated=%d", removalNotices, removalPushes, unrelatedNotices)
	}

	var blocks int64
	if err := migrationStore.DB.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", manager.ID, member.ID).Count(&blocks).Error; err != nil || blocks != 1 {
		t.Fatalf("persisted block count=%d err=%v", blocks, err)
	}
	if _, err := socialRepository.Review(ctx, manager.ID, *member.Username); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("review after final shared Path removal error=%v, want opaque not found", err)
	}
	if _, err := NewSocialProfileRepository(runtimeStore.DB).GetByUsername(ctx, manager.ID, *member.Username); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("profile after Path removal error=%v, want opaque not found", err)
	}
}

func blockedMemberRemovalCommand(actor, member, pathID string, at time.Time, key string) pathapp.RemoveMemberCommand {
	notificationID := newTestID()
	return pathapp.RemoveMemberCommand{
		ActorUserID: actor, TargetUserID: member, PathID: pathdomain.ID(pathID), ExpectedRole: pathdomain.RoleParticipant, RemovedAt: at,
		Idempotency:  ports.Idempotency{PrincipalID: actor, Operation: pathapp.RemoveMemberOperation, Key: key, RequestHash: bytes.Repeat([]byte{7}, 32)},
		Audit:        audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: audit.ResourceUpdated, TargetType: "path_member", TargetID: pathID + ":" + member, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at},
		Notification: pathapp.MemberAccessNotification{ID: notificationID, RecipientUserID: member, ActorUserID: actor, PathID: pathdomain.ID(pathID), Kind: pathapp.NotificationPathMemberRemoved, Role: pathdomain.RoleParticipant, CreatedAt: at},
		NewID:        newTestID, AuthorizationWorker: "blocked-admin-path", AuthorizationLease: time.Minute,
	}
}

func memberPageContains(page pathapp.MemberPage, userID, role string) bool {
	for _, item := range page.Items {
		if item.UserID == userID && item.Role == role {
			return true
		}
	}
	return false
}

func assertBlockedMemberPathRows(t *testing.T, db *gorm.DB, pathID, memberID string, wantMemberships, wantActivities int64) {
	t.Helper()
	var memberships, activities int64
	if err := db.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, memberID).Count(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("recorded_activity_models").Where("path_id = ? AND participant_id = ?", pathID, memberID).Count(&activities).Error; err != nil {
		t.Fatal(err)
	}
	if memberships != wantMemberships || activities != wantActivities {
		t.Fatalf("Path %s member rows: memberships=%d activities=%d, want %d/%d", pathID, memberships, activities, wantMemberships, wantActivities)
	}
}
