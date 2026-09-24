package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPostgresBlockAtomicallyEndsPairRelationshipsRetainsSharedPathAndReplays(t *testing.T) {
	repository, db := socialRelationshipTestRepository(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	actor, target := socialRelationshipTestUsers(t, db, identity.ProfileVisibilityPublic, now)
	pathID := "block-path-" + newTestID()
	t.Cleanup(func() {
		_ = db.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{}).Error
	})
	if err := db.Table("path_models").Create(map[string]any{"id": pathID, "owner_user_id": target.ID, "name": "Shared Piano", "visibility": "private", "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_membership_models").Create(map[string]any{
		"path_id": pathID, "user_id": actor.ID, "role": "participant",
	}).Error; err != nil {
		t.Fatal(err)
	}

	review, err := repository.Review(context.Background(), actor.ID, *target.Username)
	if err != nil || review.Target.UserID != target.ID || len(review.SharedPaths) != 1 || review.SharedPaths[0].ID != pathID {
		t.Fatalf("review=%+v err=%v", review, err)
	}
	first, err := repository.Follow(context.Background(), socialRelationshipTestCommand(actor.ID, socialapp.FollowOperation, *target.Username, "block-follow-one", now.Add(time.Second)))
	if err != nil || first.AuthorizationChange.ID == "" {
		t.Fatalf("first follow=%+v err=%v", first, err)
	}
	second, err := repository.Follow(context.Background(), socialRelationshipTestCommand(target.ID, socialapp.FollowOperation, *actor.Username, "block-follow-two", now.Add(2*time.Second)))
	if err != nil || second.AuthorizationChange.ID == "" {
		t.Fatalf("second follow=%+v err=%v", second, err)
	}
	if err := db.Create(&socialFollowRequestModel{ID: "block-request-" + newTestID(), RequesterUserID: actor.ID, TargetUserID: target.ID, CreatedAt: now.Add(3 * time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&socialFollowRequestModel{ID: "block-request-" + newTestID(), RequesterUserID: target.ID, TargetUserID: actor.ID, CreatedAt: now.Add(3 * time.Second)}).Error; err != nil {
		t.Fatal(err)
	}

	stale := socialBlockTestCommand(actor.ID, socialapp.BlockOperation, *target.Username, target.ID, "stale-block-mutation-key", now.Add(4*time.Second))
	if _, err := repository.Block(context.Background(), stale); !errors.Is(err, socialapp.ErrBlockReviewRequired) {
		t.Fatalf("block with stale shared-Path review err=%v", err)
	}
	assertSocialBlockEffects(t, db, actor.ID, target.ID, pathID, 0, 2, 2)

	command := socialBlockTestCommand(actor.ID, socialapp.BlockOperation, *target.Username, target.ID, "block-mutation-key", now.Add(4*time.Second))
	command.ExpectedSharedPathIDs = []string{pathID}
	blocked, err := repository.Block(context.Background(), command)
	if err != nil || !blocked.Blocked || !blocked.Changed || blocked.Replayed || len(blocked.AuthorizationChanges) != 2 {
		t.Fatalf("block=%+v err=%v", blocked, err)
	}
	for _, change := range blocked.AuthorizationChanges {
		if change.Operation != ports.AuthorizationDelete || change.Relation != "follower" || change.ActorUserID != actor.ID {
			t.Fatalf("change=%+v", change)
		}
	}
	assertSocialBlockEffects(t, db, actor.ID, target.ID, pathID, 1, 0, 0)

	replayed, err := repository.Block(context.Background(), command)
	if err != nil || !replayed.Replayed || len(replayed.AuthorizationChanges) != 2 || replayed.AuthorizationChanges[0].ID != blocked.AuthorizationChanges[0].ID {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	expiredReplay := command
	expiredReplay.OccurredAt = command.ReviewExpiresAt.Add(time.Second)
	expiredReplay.Audit.OccurredAt = expiredReplay.OccurredAt
	replayed, err = repository.Block(context.Background(), expiredReplay)
	if err != nil || !replayed.Replayed || replayed.Target.UserID != blocked.Target.UserID {
		t.Fatalf("expired acknowledgement exact replay=%+v err=%v", replayed, err)
	}
	page, err := repository.ListAuthored(context.Background(), actor.ID, ports.PageRequest{Snapshot: now.Add(time.Hour), Limit: 10})
	if err != nil || len(page.Accounts) != 1 || page.Accounts[0].Target.UserID != target.ID {
		t.Fatalf("page=%+v err=%v", page, err)
	}

	unblock := socialBlockTestCommand(actor.ID, socialapp.UnblockOperation, "", target.ID, "unblock-mutation-key", now.Add(5*time.Second))
	unblocked, err := repository.Unblock(context.Background(), unblock)
	if err != nil || unblocked.Blocked || !unblocked.Changed || len(unblocked.AuthorizationChanges) != 0 {
		t.Fatalf("unblock=%+v err=%v", unblocked, err)
	}
	assertSocialBlockEffects(t, db, actor.ID, target.ID, pathID, 0, 0, 0)
	unblockedReplay, err := repository.Unblock(context.Background(), unblock)
	if err != nil || !unblockedReplay.Replayed || unblockedReplay.Blocked {
		t.Fatalf("unblock replay=%+v err=%v", unblockedReplay, err)
	}
	if _, err := repository.Unblock(context.Background(), socialBlockTestCommand(actor.ID, socialapp.UnblockOperation, "", target.ID, "new-unblock-key", now.Add(6*time.Second))); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("new unblock after removal err=%v", err)
	}
}

func assertSocialBlockEffects(t *testing.T, db *gorm.DB, actor, target, pathID string, blocks, follows, pending int64) {
	t.Helper()
	var gotBlocks, gotFollows, gotPending, memberships int64
	if err := db.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", actor, target).Count(&gotBlocks).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("follow_models").Where("(follower_user_id = ? AND following_user_id = ?) OR (follower_user_id = ? AND following_user_id = ?)", actor, target, target, actor).Count(&gotFollows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("follow_request_models").Where("((requester_user_id = ? AND target_user_id = ?) OR (requester_user_id = ? AND target_user_id = ?)) AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL", actor, target, target, actor).Count(&gotPending).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_membership_models").Where("path_id = ? AND user_id IN ?", pathID, []string{actor, target}).Count(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if gotBlocks != blocks || gotFollows != follows || gotPending != pending || memberships != 1 {
		t.Fatalf("effects blocks=%d follows=%d pending=%d memberships=%d", gotBlocks, gotFollows, gotPending, memberships)
	}
}

func socialBlockTestCommand(actor, operation, username, targetID, key string, at time.Time) socialapp.BlockCommand {
	hash := sha256.Sum256([]byte(operation + "\x00" + username + "\x00" + targetID))
	action := audit.ResourceCreated
	if operation == socialapp.UnblockOperation {
		action = audit.ResourceDeleted
	}
	command := socialapp.BlockCommand{ActorUserID: actor, TargetUsername: username, TargetUserID: targetID, OccurredAt: at,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: hash[:]},
		Audit:       audit.Event{ID: "audit-" + newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: action, TargetType: "user_block", TargetID: username + targetID, Outcome: audit.Succeeded, CorrelationID: "correlation", OccurredAt: at}}
	if operation == socialapp.BlockOperation {
		command.ReviewExpiresAt = at.Add(time.Minute)
	}
	return command
}
