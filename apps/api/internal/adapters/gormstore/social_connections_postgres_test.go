package gormstore

import (
	"context"
	"errors"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestProfileConnectionListsEnforceCurrentOwnerOrFollower(t *testing.T) {
	repo, db := socialRelationshipTestRepository(t)
	now := time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC)
	owner := socialRelationshipTestUser(t, db, "owner", identity.ProfileVisibilityPublic, now)
	follower := socialRelationshipTestUser(t, db, "follower", identity.ProfileVisibilityPublic, now)
	other := socialRelationshipTestUser(t, db, "other", identity.ProfileVisibilityPublic, now)
	ctx := context.Background()
	for i, u := range []userModel{follower, other} {
		if _, err := repo.Follow(ctx, socialRelationshipTestCommand(u.ID, socialapp.FollowOperation, *owner.Username, "connect-follow", now.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	page := ports.PageRequest{Limit: 1, Snapshot: now.Add(time.Minute)}
	got, err := repo.ListConnections(ctx, owner.ID, owner.ID, socialapp.Followers, page)
	if err != nil || len(got.Profiles) != 1 || !got.HasMore || got.Profiles[0].ID != other.ID {
		t.Fatalf("first page: %+v %v", got, err)
	}
	page.AfterCreated, page.AfterID = got.LastCreated, got.LastID
	got, err = repo.ListConnections(ctx, follower.ID, owner.ID, socialapp.Followers, page)
	if err != nil || len(got.Profiles) != 1 || got.Profiles[0].ID != follower.ID || got.HasMore {
		t.Fatalf("follower next page: %+v %v", got, err)
	}
	if err := db.Create(&socialBlockTestModel{BlockerUserID: owner.ID, BlockedUserID: other.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	visible, err := repo.ListConnections(ctx, owner.ID, owner.ID, socialapp.Followers, ports.PageRequest{Limit: 25, Snapshot: page.Snapshot})
	if err != nil || len(visible.Profiles) != 1 || visible.Profiles[0].ID != follower.ID {
		t.Fatalf("blocked identity shown: %+v %v", visible, err)
	}
	stranger := socialRelationshipTestUser(t, db, "stranger", identity.ProfileVisibilityPublic, now)
	if _, err = repo.ListConnections(ctx, stranger.ID, owner.ID, socialapp.Followers, page); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("stranger access: %v", err)
	}
	if _, err = repo.Unfollow(ctx, socialRelationshipTestCommand(follower.ID, socialapp.UnfollowOperation, *owner.Username, "connect-unfollow", now.Add(2*time.Second))); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ListConnections(ctx, follower.ID, owner.ID, socialapp.Followers, page); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("stale follower access: %v", err)
	}
}

func TestRemoveFollowerPreservesReverseAndReplayDoesNotRemoveNewFollow(t *testing.T) {
	repo, db := socialRelationshipTestRepository(t)
	now := time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC)
	follower, owner := socialRelationshipTestUsers(t, db, identity.ProfileVisibilityPublic, now)
	pathID := "connections-path-" + newTestID()
	t.Cleanup(func() { _ = db.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{}).Error })
	if err := db.Table("path_models").Create(map[string]any{"id": pathID, "owner_user_id": owner.ID, "name": "Shared Practice", "visibility": "private", "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_membership_models").Create(map[string]any{"path_id": pathID, "user_id": follower.ID, "role": "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	inbound := socialRelationshipTestCommand(follower.ID, socialapp.FollowOperation, *owner.Username, "inbound-follow", now)
	if _, err := repo.Follow(ctx, inbound); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Follow(ctx, socialRelationshipTestCommand(owner.ID, socialapp.FollowOperation, *follower.Username, "reverse-follow", now)); err != nil {
		t.Fatal(err)
	}
	var before int64
	db.Table("notification_models").Where("recipient_user_id IN ?", []string{owner.ID, follower.ID}).Count(&before)
	remove := socialRelationshipTestCommand(owner.ID, socialapp.RemoveFollowerOperation, *follower.Username, "remove-inbound", now.Add(time.Second))
	remove.TargetUserID = follower.ID
	remove.TargetUsername = ""
	if _, err := repo.RemoveFollower(ctx, remove); !errors.Is(err, ports.ErrAuthorizationPending) {
		t.Fatalf("removal overtook pending grant: %v", err)
	}
	if err := db.Model(&authorizationOutboxModel{}).Where("resource_type = 'user' AND resource_id = ?", owner.ID).Update("completed_at", now).Error; err != nil {
		t.Fatal(err)
	}
	invalidAudit := remove
	invalidAudit.Audit.ID = inbound.Audit.ID
	if _, err := repo.RemoveFollower(ctx, invalidAudit); err == nil {
		t.Fatal("duplicate audit should roll back removal")
	}
	var retained int64
	if err := db.Model(&socialFollowModel{}).Where("follower_user_id = ? AND following_user_id = ?", follower.ID, owner.ID).Count(&retained).Error; err != nil || retained != 1 {
		t.Fatalf("failed audit lost relationship: %d %v", retained, err)
	}
	result, err := repo.RemoveFollower(ctx, remove)
	if err != nil || result.Target.Relationship != domain.RelationshipFollowing || result.AuthorizationChange.ResourceID != owner.ID || result.AuthorizationChange.SubjectID != follower.ID || result.AuthorizationChange.Operation != ports.AuthorizationDelete {
		t.Fatalf("remove: %+v %v", result, err)
	}
	var inboundCount, reverseCount, after int64
	db.Model(&socialFollowModel{}).Where("follower_user_id = ? AND following_user_id = ?", follower.ID, owner.ID).Count(&inboundCount)
	db.Model(&socialFollowModel{}).Where("follower_user_id = ? AND following_user_id = ?", owner.ID, follower.ID).Count(&reverseCount)
	db.Table("notification_models").Where("recipient_user_id IN ?", []string{owner.ID, follower.ID}).Count(&after)
	if inboundCount != 0 || reverseCount != 1 || after != before {
		t.Fatalf("side effects: inbound=%d reverse=%d notices=%d/%d", inboundCount, reverseCount, before, after)
	}
	inbound.Idempotency.Key = "inbound-refollow"
	inbound.OccurredAt = now.Add(2 * time.Second)
	inbound.Audit.OccurredAt = inbound.OccurredAt
	inbound.Audit.ID = newTestID()
	if _, err = repo.Follow(ctx, inbound); !errors.Is(err, ports.ErrAuthorizationPending) {
		t.Fatalf("new follow overtook removal: %v", err)
	}
	if err := db.Model(&authorizationOutboxModel{}).Where("id = ?", result.AuthorizationChange.ID).Update("completed_at", now.Add(time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Follow(ctx, inbound); err != nil {
		t.Fatal(err)
	}
	replay, err := repo.RemoveFollower(ctx, remove)
	if err != nil || !replay.Replayed || replay.AuthorizationChange.ID != result.AuthorizationChange.ID {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	db.Model(&socialFollowModel{}).Where("follower_user_id = ? AND following_user_id = ?", follower.ID, owner.ID).Count(&inboundCount)
	if inboundCount != 1 {
		t.Fatal("old removal replay removed a new follow")
	}
	var memberships int64
	if err := db.Table("path_membership_models").Where("path_id = ? AND user_id = ? AND role = 'participant'", pathID, follower.ID).Count(&memberships).Error; err != nil || memberships != 1 {
		t.Fatalf("independent membership changed: %d %v", memberships, err)
	}
	if err := db.Model(&authorizationOutboxModel{}).Where("resource_type = 'user' AND resource_id = ?", owner.ID).Update("completed_at", now.Add(3*time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&userModel{}).Where("id = ?", owner.ID).Update("profile_visibility", identity.ProfileVisibilityPrivate).Error; err != nil {
		t.Fatal(err)
	}
	remove.Idempotency.Key = "remove-private-follow"
	remove.Audit.ID = newTestID()
	remove.OccurredAt = now.Add(4 * time.Second)
	remove.Audit.OccurredAt = remove.OccurredAt
	removedPrivate, err := repo.RemoveFollower(ctx, remove)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&authorizationOutboxModel{}).Where("id = ?", removedPrivate.AuthorizationChange.ID).Update("completed_at", now.Add(5*time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	inbound.Idempotency.Key = "private-request-after-removal"
	inbound.Audit.ID = newTestID()
	inbound.OccurredAt = now.Add(6 * time.Second)
	inbound.Audit.OccurredAt = inbound.OccurredAt
	request, err := repo.Follow(ctx, inbound)
	if err != nil || request.Target.Relationship != domain.RelationshipRequested || request.RequestID == "" {
		t.Fatalf("private follow after removal: %+v %v", request, err)
	}
}
