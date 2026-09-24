package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestSocialRelationshipRepositoryPublicFollowReplayAndUnfollowAreAtomic(t *testing.T) {
	repository, migrationDB := socialRelationshipTestRepository(t)
	now := time.Date(2026, 7, 27, 18, 0, 0, 0, time.UTC)
	actor, target := socialRelationshipTestUsers(t, migrationDB, identity.ProfileVisibilityPublic, now)

	follow := socialRelationshipTestCommand(actor.ID, socialapp.FollowOperation, *target.Username, "public-follow-key", now)
	created, err := repository.Follow(context.Background(), follow)
	if err != nil || created.Replayed || !created.Changed || created.Target.Relationship != domain.RelationshipFollowing || created.Target.FollowerCount != 1 || created.RequestID != "" ||
		created.AuthorizationChange.ID == "" || created.AuthorizationChange.Operation != ports.AuthorizationTouch {
		t.Fatalf("Follow() = %+v, %v", created, err)
	}
	assertSocialRelationshipEffects(t, migrationDB, actor.ID, target.ID, 1, 0, 1, 1, 1)

	replayed, err := repository.Follow(context.Background(), follow)
	if err != nil || !replayed.Replayed || !replayed.Changed || replayed.Target.Relationship != domain.RelationshipFollowing || replayed.Target.FollowerCount != 1 ||
		replayed.AuthorizationChange.ID != created.AuthorizationChange.ID {
		t.Fatalf("replayed Follow() = %+v, %v", replayed, err)
	}
	if state, err := repository.AuthorizationChangeState(context.Background(), created.AuthorizationChange.ID); err != nil ||
		(state != socialapp.AuthorizationChangeLocked && state != socialapp.AuthorizationChangePending && state != socialapp.AuthorizationChangeCompleted) {
		t.Fatalf("authorization state = %q, %v", state, err)
	}
	assertSocialRelationshipEffects(t, migrationDB, actor.ID, target.ID, 1, 0, 1, 1, 1)
	conflict := follow
	conflict.Idempotency.RequestHash = append([]byte(nil), follow.Idempotency.RequestHash...)
	conflict.Idempotency.RequestHash[0] ^= 0xff
	if _, err := repository.Follow(context.Background(), conflict); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict error = %v", err)
	}

	unfollow := socialRelationshipTestCommand(actor.ID, socialapp.UnfollowOperation, *target.Username, "public-unfollow-key", now.Add(time.Second))
	removed, err := repository.Unfollow(context.Background(), unfollow)
	if err != nil || !removed.Changed || removed.Target.Relationship != domain.RelationshipNone || removed.Target.FollowerCount != 0 ||
		removed.AuthorizationChange.ID == "" || removed.AuthorizationChange.Operation != ports.AuthorizationDelete {
		t.Fatalf("Unfollow() = %+v, %v", removed, err)
	}
	assertSocialRelationshipEffects(t, migrationDB, actor.ID, target.ID, 0, 0, 1, 2, 2)
}

func TestSocialRelationshipRepositoryPrivateRequestListCancelAcceptAndReject(t *testing.T) {
	repository, migrationDB := socialRelationshipTestRepository(t)
	now := time.Date(2026, 7, 27, 19, 0, 0, 0, time.UTC)
	first, target := socialRelationshipTestUsers(t, migrationDB, identity.ProfileVisibilityPrivate, now)
	second := socialRelationshipTestUser(t, migrationDB, "second", identity.ProfileVisibilityPublic, now)

	firstFollow := socialRelationshipTestCommand(first.ID, socialapp.FollowOperation, *target.Username, "private-follow-one", now)
	firstResult, err := repository.Follow(context.Background(), firstFollow)
	if err != nil || !firstResult.Changed || firstResult.Target.Relationship != domain.RelationshipRequested || firstResult.RequestID == "" || firstResult.AuthorizationChange != (ports.AuthorizationChange{}) {
		t.Fatalf("private Follow() = %+v, %v", firstResult, err)
	}
	secondFollow := socialRelationshipTestCommand(second.ID, socialapp.FollowOperation, *target.Username, "private-follow-two", now.Add(time.Second))
	secondResult, err := repository.Follow(context.Background(), secondFollow)
	if err != nil || secondResult.Target.Relationship != domain.RelationshipRequested || secondResult.RequestID == "" {
		t.Fatalf("second private Follow() = %+v, %v", secondResult, err)
	}

	page, err := repository.ListIncoming(context.Background(), target.ID, ports.PageRequest{Snapshot: now.Add(2 * time.Second), Limit: 1})
	if err != nil || len(page.Requests) != 1 || !page.HasMore || page.Requests[0].ID != secondResult.RequestID {
		t.Fatalf("first incoming page = %+v, %v", page, err)
	}
	next, err := repository.ListIncoming(context.Background(), target.ID, ports.PageRequest{
		Snapshot: now.Add(2 * time.Second), Limit: 1,
		AfterCreated: page.Requests[0].CreatedAt, AfterID: page.Requests[0].ID,
	})
	if err != nil || len(next.Requests) != 1 || next.HasMore || next.Requests[0].ID != firstResult.RequestID {
		t.Fatalf("second incoming page = %+v, %v", next, err)
	}

	accept := socialRelationshipReviewCommand(target.ID, socialapp.AcceptFollowRequestOperation, secondResult.RequestID, "private-accept-key", now.Add(3*time.Second))
	accepted, err := repository.AcceptRequest(context.Background(), accept)
	if err != nil || accepted.Decision != socialapp.FollowRequestAccepted || accepted.Request.ID != secondResult.RequestID ||
		accepted.AuthorizationChange.ID == "" || accepted.AuthorizationChange.Operation != ports.AuthorizationTouch {
		t.Fatalf("AcceptRequest() = %+v, %v", accepted, err)
	}
	acceptedReplay, err := repository.AcceptRequest(context.Background(), accept)
	if err != nil || !acceptedReplay.Replayed || acceptedReplay.Decision != socialapp.FollowRequestAccepted || acceptedReplay.AuthorizationChange.ID != accepted.AuthorizationChange.ID {
		t.Fatalf("replayed AcceptRequest() = %+v, %v", acceptedReplay, err)
	}

	cancel := socialRelationshipTestCommand(first.ID, socialapp.CancelFollowRequestOperation, *target.Username, "private-cancel-key", now.Add(4*time.Second))
	canceled, err := repository.CancelRequest(context.Background(), cancel)
	if err != nil || canceled.Target.Relationship != domain.RelationshipNone || canceled.RequestID != firstResult.RequestID {
		t.Fatalf("CancelRequest() = %+v, %v", canceled, err)
	}
	if _, err := repository.CancelRequest(context.Background(), socialRelationshipTestCommand(first.ID, socialapp.CancelFollowRequestOperation, *target.Username, "another-cancel-key", now.Add(5*time.Second))); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("second cancellation error = %v", err)
	}

	third := socialRelationshipTestUser(t, migrationDB, "third", identity.ProfileVisibilityPublic, now)
	thirdResult, err := repository.Follow(context.Background(), socialRelationshipTestCommand(third.ID, socialapp.FollowOperation, *target.Username, "private-follow-three", now.Add(6*time.Second)))
	if err != nil {
		t.Fatal(err)
	}
	reject := socialRelationshipReviewCommand(target.ID, socialapp.RejectFollowRequestOperation, thirdResult.RequestID, "private-reject-key", now.Add(7*time.Second))
	rejected, err := repository.RejectRequest(context.Background(), reject)
	if err != nil || rejected.Decision != socialapp.FollowRequestRejected || rejected.Request.ID != thirdResult.RequestID || rejected.AuthorizationChange != (ports.AuthorizationChange{}) {
		t.Fatalf("RejectRequest() = %+v, %v", rejected, err)
	}

	var follows, pending, received, acceptedNotices int64
	if err := migrationDB.Model(&socialFollowModel{}).Where("follower_user_id IN ? AND following_user_id = ?", []string{first.ID, second.ID, third.ID}, target.ID).Count(&follows).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&socialFollowRequestModel{}).Where("target_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL", target.ID).Count(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_models").Where("kind = ? AND recipient_user_id = ?", "follow_request_received", target.ID).Count(&received).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_models").Where("kind = ? AND recipient_user_id = ?", "follow_request_accepted", second.ID).Count(&acceptedNotices).Error; err != nil {
		t.Fatal(err)
	}
	if follows != 1 || pending != 0 || received != 3 || acceptedNotices != 1 {
		t.Fatalf("effects follows=%d pending=%d received=%d accepted=%d", follows, pending, received, acceptedNotices)
	}
}

func TestSocialRelationshipRepositoryMissingSelfAndBlockedTargetsAreOpaqueWithoutEffects(t *testing.T) {
	repository, migrationDB := socialRelationshipTestRepository(t)
	now := time.Date(2026, 7, 27, 20, 0, 0, 0, time.UTC)
	actor, target := socialRelationshipTestUsers(t, migrationDB, identity.ProfileVisibilityPublic, now)
	if err := migrationDB.Table("block_models").Create(map[string]any{
		"blocker_user_id": target.ID, "blocked_user_id": actor.ID, "created_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	for index, username := range []string{*target.Username, *actor.Username, "missing.profile"} {
		command := socialRelationshipTestCommand(actor.ID, socialapp.FollowOperation, username, "opaque-follow-key-"+string(rune('a'+index)), now)
		if result, err := repository.Follow(context.Background(), command); !errors.Is(err, ports.ErrNotFound) || result != (socialapp.RelationshipResult{}) {
			t.Fatalf("target %q result=%+v err=%v", username, result, err)
		}
	}
	assertSocialRelationshipEffects(t, migrationDB, actor.ID, target.ID, 0, 0, 0, 0, 0)
}

func TestConcurrentPublicFollowsConvergeWithoutDuplicateRelationshipNotificationOrAuthorization(t *testing.T) {
	repository, migrationDB := socialRelationshipTestRepository(t)
	now := time.Date(2026, 7, 27, 21, 0, 0, 0, time.UTC)
	actor, target := socialRelationshipTestUsers(t, migrationDB, identity.ProfileVisibilityPublic, now)
	commands := []socialapp.RelationshipCommand{
		socialRelationshipTestCommand(actor.ID, socialapp.FollowOperation, *target.Username, "concurrent-follow-a", now),
		socialRelationshipTestCommand(actor.ID, socialapp.FollowOperation, *target.Username, "concurrent-follow-b", now),
	}
	errorsByWorker := make([]error, len(commands))
	results := make([]socialapp.RelationshipResult, len(commands))
	var wait sync.WaitGroup
	for index := range commands {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results[index], errorsByWorker[index] = repository.Follow(context.Background(), commands[index])
		}()
	}
	wait.Wait()
	for index, err := range errorsByWorker {
		if err != nil || results[index].Target.Relationship != domain.RelationshipFollowing {
			t.Fatalf("worker %d result=%+v err=%v", index, results[index], err)
		}
	}
	changed := 0
	for _, result := range results {
		if result.Changed {
			changed++
		}
	}
	if changed != 1 {
		t.Fatalf("concurrent results changed=%d want exactly one", changed)
	}
	assertSocialRelationshipEffects(t, migrationDB, actor.ID, target.ID, 1, 0, 1, 1, 2)
}

func socialRelationshipTestRepository(t *testing.T) (*SocialRelationshipRepository, *gorm.DB) {
	t.Helper()
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
	return NewSocialRelationshipRepository(runtimeStore.DB, newTestID, "social-test-worker", time.Minute), migrationStore.DB
}

func socialRelationshipTestUsers(t *testing.T, db *gorm.DB, targetVisibility identity.ProfileVisibility, now time.Time) (userModel, userModel) {
	t.Helper()
	actor := socialRelationshipTestUser(t, db, "actor", identity.ProfileVisibilityPublic, now)
	target := socialRelationshipTestUser(t, db, "target", targetVisibility, now)
	return actor, target
}

func socialRelationshipTestUser(t *testing.T, db *gorm.DB, prefix string, visibility identity.ProfileVisibility, now time.Time) userModel {
	t.Helper()
	id := "social-relationship-" + newTestID()
	username := prefix + strings.ReplaceAll(newTestID(), "-", "")[:16]
	user := userModel{
		ID: id, Username: &username, DisplayName: strings.ToUpper(prefix[:1]) + prefix[1:],
		ProfileVisibility: &visibility, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func socialRelationshipTestCommand(actor, operation, username, key string, now time.Time) socialapp.RelationshipCommand {
	digest := sha256.Sum256([]byte(operation + "\x00" + username))
	action, targetType := audit.ResourceCreated, "profile_follow"
	if operation == socialapp.CancelFollowRequestOperation {
		action, targetType = audit.ResourceDeleted, "follow_request"
	} else if operation == socialapp.UnfollowOperation {
		action = audit.ResourceDeleted
	}
	return socialapp.RelationshipCommand{
		ActorUserID: actor, TargetUsername: username, OccurredAt: now,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: digest[:]},
		Audit: audit.Event{
			ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: action,
			TargetType: targetType, TargetID: username, Outcome: audit.Succeeded,
			CorrelationID: newTestID(), OccurredAt: now,
		},
	}
}

func socialRelationshipReviewCommand(actor, operation, requestID, key string, now time.Time) socialapp.RelationshipCommand {
	digest := sha256.Sum256([]byte(operation + "\x00" + requestID))
	action := audit.ResourceUpdated
	if operation == socialapp.RejectFollowRequestOperation {
		action = audit.ResourceDeleted
	}
	return socialapp.RelationshipCommand{
		ActorUserID: actor, RequestID: requestID, OccurredAt: now,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: digest[:]},
		Audit: audit.Event{
			ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: action,
			TargetType: "follow_request", TargetID: requestID, Outcome: audit.Succeeded,
			CorrelationID: newTestID(), OccurredAt: now,
		},
	}
}

func assertSocialRelationshipEffects(t *testing.T, db *gorm.DB, actor, target string, follows, requests, notifications, authorization, audits int64) {
	t.Helper()
	checks := []struct {
		table, predicate string
		want             int64
		values           []any
	}{
		{"follow_models", "follower_user_id = ? AND following_user_id = ?", follows, []any{actor, target}},
		{"follow_request_models", "requester_user_id = ? AND target_user_id = ?", requests, []any{actor, target}},
		{"notification_models", "actor_user_id = ? AND follow_subject_user_id = ?", notifications, []any{actor, actor}},
		{"authorization_outbox_models", "resource_type = 'user' AND resource_id = ? AND subject_id = ?", authorization, []any{target, actor}},
		{"audit_event_models", "actor_user_id = ? AND target_id = ?", audits, []any{actor, *socialRelationshipTargetUsername(t, db, target)}},
	}
	for _, check := range checks {
		var count int64
		if err := db.Table(check.table).Where(check.predicate, check.values...).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != check.want {
			t.Fatalf("%s count=%d want=%d", check.table, count, check.want)
		}
	}
}

func socialRelationshipTargetUsername(t *testing.T, db *gorm.DB, target string) *string {
	t.Helper()
	var user userModel
	if err := db.Where("id = ?", target).First(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user.Username
}
