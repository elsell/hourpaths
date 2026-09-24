package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresInteractionSettingsHideWithoutDeletingRestoreAndTombstoneEveryRecipient(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
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
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner := socialRelationshipTestUser(t, migrationStore.DB, "settingowner", identity.ProfileVisibilityPublic, now)
	commenter := socialRelationshipTestUser(t, migrationStore.DB, "settingcommenter", identity.ProfileVisibilityPublic, now)
	hearter := socialRelationshipTestUser(t, migrationStore.DB, "settinghearter", identity.ProfileVisibilityPublic, now)
	pathID, activityID := "setting-path-"+newTestID(), "setting-activity-"+newTestID()
	eventID := "practice:" + activityID
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{owner.ID, commenter.ID, hearter.ID}).Delete(&struct{ ID string }{})
	})
	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		CreatedAt, UpdatedAt              time.Time
	}
	type activityRow struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
	}
	if err := migrationStore.DB.Table("path_models").Create(&pathRow{ID: pathID, OwnerUserID: owner.ID, Name: "Piano", Visibility: "public", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("recorded_activity_models").Create(&activityRow{ID: activityID, PathID: pathID, ParticipantID: owner.ID, OccurrenceTimeZone: "Etc/UTC", StartedAt: now.Add(-time.Hour), EndedAt: now.Add(-30 * time.Minute), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	for _, follower := range []string{commenter.ID, hearter.ID} {
		if err := migrationStore.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: follower, FollowingUserID: owner.ID, CreatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := NewSocialFeedRepository(runtimeStore.DB)
	interactionAt := now.Add(-10 * time.Second)
	initial, err := repository.GetInteractionSettings(context.Background(), owner.ID)
	if err != nil || initial != (socialapp.InteractionSettings{CommentsEnabled: true, ReactionsEnabled: true}) {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	if _, err := repository.UpdateInteractionSettings(context.Background(), interactionSettingsTestCommand(hearter.ID, socialapp.InteractionSettings{CommentsEnabled: false, ReactionsEnabled: true}, "setting-other--01", now.Add(-time.Second))); err != nil {
		t.Fatal(err)
	}
	if stillOwner, err := repository.GetInteractionSettings(context.Background(), owner.ID); err != nil || stillOwner != initial {
		t.Fatalf("non-owner changed owner settings=%+v err=%v", stillOwner, err)
	}
	reactionTarget := socialapp.ReactionTarget{EventID: eventID, PathID: pathID, OwnerUserID: owner.ID}
	if _, err := repository.SetPracticeReaction(context.Background(), socialReactionTestCommand(commenter.ID, reactionTarget, socialdomain.ReactionFire, socialapp.SetPracticeReactionOperation, "setting-reaction-01", interactionAt)); err != nil {
		t.Fatal(err)
	}
	commentTarget := socialapp.PracticeCommentTarget(reactionTarget)
	comment, err := repository.CreatePracticeComment(context.Background(), socialCommentTestCommand(commenter.ID, commentTarget, "Keep going", "", 0, socialapp.CreatePracticeCommentOperation, "setting-comment-01", interactionAt.Add(time.Second)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SetPracticeCommentHeart(context.Background(), socialCommentHeartTestCommand(hearter.ID, commentTarget, comment.Comment, true, "setting-heart-0001", interactionAt.Add(2*time.Second))); err != nil {
		t.Fatal(err)
	}
	type notice struct{ ID, RecipientUserID, Kind string }
	var notices []notice
	if err := migrationStore.DB.Table("notification_models").Where("social_feed_event_id = ?", eventID).Order("kind").Find(&notices).Error; err != nil || len(notices) != 3 {
		t.Fatalf("notices=%+v err=%v", notices, err)
	}
	disabled := socialapp.InteractionSettings{CommentsEnabled: false, ReactionsEnabled: false}
	disableCommand := interactionSettingsTestCommand(owner.ID, disabled, "setting-disable-01", now.Add(3*time.Second))
	result, err := repository.UpdateInteractionSettings(context.Background(), disableCommand)
	if err != nil || result.Settings != disabled || result.Replayed {
		t.Fatalf("disabled=%+v err=%v", result, err)
	}
	replayed, err := repository.UpdateInteractionSettings(context.Background(), disableCommand)
	if err != nil || !replayed.Replayed || replayed.Settings != disabled {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	conflict := disableCommand
	conflict.Settings.CommentsEnabled = true
	conflictingHash := sha256.Sum256([]byte{1, 0})
	conflict.Idempotency.RequestHash = conflictingHash[:]
	if _, err := repository.UpdateInteractionSettings(context.Background(), conflict); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	var audits int64
	if err := migrationStore.DB.Table("audit_event_models").Where("target_type = 'social_interaction_settings' AND target_id = ?", owner.ID).Count(&audits).Error; err != nil || audits != 1 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
	page, err := repository.ListPracticeCandidates(context.Background(), commenter.ID, socialapp.FeedPageRequest{Snapshot: now.Add(4 * time.Second), Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].CommentsEnabled || page.Items[0].ReactionsEnabled || page.Items[0].Reactions.Counts != (socialdomain.ReactionCounts{}) {
		t.Fatalf("feed=%+v err=%v", page, err)
	}
	comments, err := repository.ListPracticeComments(context.Background(), commenter.ID, eventID, socialapp.CommentPageRequest{Snapshot: now.Add(4 * time.Second), Limit: 10})
	if err != nil || len(comments.Items) != 0 {
		t.Fatalf("comments=%+v err=%v", comments, err)
	}
	if _, err := repository.CreatePracticeComment(context.Background(), socialCommentTestCommand(commenter.ID, commentTarget, "hidden", "", 0, socialapp.CreatePracticeCommentOperation, "setting-comment-02", now.Add(4*time.Second))); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("create err=%v", err)
	}
	for table, where := range map[string]string{"social_practice_reaction_models": "social_feed_event_id = ?", "social_practice_comment_models": "social_feed_event_id = ?"} {
		var count int64
		if err := migrationStore.DB.Table(table).Where(where, eventID).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
	var hearts int64
	if err := migrationStore.DB.Table("social_practice_comment_heart_models").Where("comment_id = ?", comment.Comment.ID).Count(&hearts).Error; err != nil || hearts != 1 {
		t.Fatalf("hearts=%d err=%v", hearts, err)
	}
	resolver := pathstore.New(runtimeStore.DB)
	for _, notice := range notices {
		projection, err := resolver.GetNotification(context.Background(), notice.RecipientUserID, notice.ID)
		want := "comments"
		if notice.Kind == "practice_reaction" {
			want = "reactions"
		}
		if err != nil || string(projection.InteractionDisabled) != want || projection.SocialFeedEventID != eventID || projection.CommentID != "" || projection.Reaction != "" || projection.Actor.UserID != owner.ID {
			t.Fatalf("notice=%+v projection=%+v err=%v", notice, projection, err)
		}
	}
	var deliveryRows int64
	if err := migrationStore.DB.Table("notification_push_outbox_models").Joins("JOIN notification_models n ON n.id = notification_push_outbox_models.notification_id").Where("n.social_feed_event_id = ?", eventID).Count(&deliveryRows).Error; err != nil || deliveryRows != 0 {
		t.Fatalf("outbox=%d err=%v", deliveryRows, err)
	}
	enabled := socialapp.InteractionSettings{CommentsEnabled: true, ReactionsEnabled: true}
	if _, err := repository.UpdateInteractionSettings(context.Background(), interactionSettingsTestCommand(owner.ID, enabled, "setting-enable--01", now.Add(5*time.Second))); err != nil {
		t.Fatal(err)
	}
	page, err = repository.ListPracticeCandidates(context.Background(), commenter.ID, socialapp.FeedPageRequest{Snapshot: now.Add(6 * time.Second), Limit: 1})
	comments, commentsErr := repository.ListPracticeComments(context.Background(), commenter.ID, eventID, socialapp.CommentPageRequest{Snapshot: now.Add(6 * time.Second), Limit: 10})
	if err != nil || commentsErr != nil || len(page.Items) != 1 || page.Items[0].Reactions.Counts.Fire != 1 || len(comments.Items) != 1 || comments.Items[0].HeartCount != 1 {
		t.Fatalf("restored feed=%+v comments=%+v errs=%v/%v", page, comments, err, commentsErr)
	}
}

func TestPostgresInteractionDisableSerializesBeforeQueuedInteractionCreation(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
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
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner := socialRelationshipTestUser(t, migrationStore.DB, "raceowner", identity.ProfileVisibilityPublic, now)
	actor := socialRelationshipTestUser(t, migrationStore.DB, "raceactor", identity.ProfileVisibilityPublic, now)
	pathID, activityID := "race-path-"+newTestID(), "race-activity-"+newTestID()
	eventID := "practice:" + activityID
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{owner.ID, actor.ID}).Delete(&struct{ ID string }{})
	})
	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		CreatedAt, UpdatedAt              time.Time
	}
	type activityRow struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
	}
	if err := migrationStore.DB.Table("path_models").Create(&pathRow{ID: pathID, OwnerUserID: owner.ID, Name: "Piano", Visibility: "public", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("recorded_activity_models").Create(&activityRow{ID: activityID, PathID: pathID, ParticipantID: owner.ID, OccurrenceTimeZone: "Etc/UTC", StartedAt: now.Add(-time.Hour), EndedAt: now.Add(-time.Minute), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: actor.ID, FollowingUserID: owner.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	blocker := migrationStore.DB.Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	t.Cleanup(func() { _ = blocker.Rollback().Error })
	if err := lockSocialInteractionOwner(blocker, owner.ID); err != nil {
		_ = blocker.Rollback()
		t.Fatal(err)
	}
	repository := NewSocialFeedRepository(runtimeStore.DB)
	settingsResult := make(chan error, 1)
	go func() {
		_, err := repository.UpdateInteractionSettings(context.Background(), interactionSettingsTestCommand(owner.ID, socialapp.InteractionSettings{CommentsEnabled: false, ReactionsEnabled: false}, "race-disable---01", now.Add(time.Second)))
		settingsResult <- err
	}()
	waitForAdvisoryWaiters(t, migrationStore, owner.ID, 1)
	reactionResult := make(chan error, 1)
	go func() {
		_, err := repository.SetPracticeReaction(context.Background(), socialReactionTestCommand(actor.ID, socialapp.ReactionTarget{EventID: eventID, PathID: pathID, OwnerUserID: owner.ID}, socialdomain.ReactionFire, socialapp.SetPracticeReactionOperation, "race-reaction--01", now.Add(2*time.Second)))
		reactionResult <- err
	}()
	waitForAdvisoryWaiters(t, migrationStore, owner.ID, 2)
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	if err := <-settingsResult; err != nil {
		t.Fatalf("settings err=%v", err)
	}
	if err := <-reactionResult; !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("reaction err=%v", err)
	}
	var count int64
	if err := migrationStore.DB.Table("social_practice_reaction_models").Where("social_feed_event_id = ?", eventID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("reactions=%d err=%v", count, err)
	}
	if err := migrationStore.DB.Table("notification_models").Where("social_feed_event_id = ? AND deleted_at IS NULL", eventID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("visible notices=%d err=%v", count, err)
	}
}

func waitForAdvisoryWaiters(t *testing.T, store *Store, owner string, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var count int64
		key := socialLockKey("social-interaction-owner", owner)
		if err := store.DB.Table("pg_locks").Where(`locktype = 'advisory' AND NOT granted
AND classid = ((hashtextextended(?, 0) >> 32) & 4294967295)::oid
AND objid = (hashtextextended(?, 0) & 4294967295)::oid
AND objsubid = 1`, key, key).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d queued advisory locks", want)
}

func interactionSettingsTestCommand(actor string, settings socialapp.InteractionSettings, key string, at time.Time) socialapp.InteractionSettingsCommand {
	digest := sha256.Sum256([]byte{boolTestByte(settings.CommentsEnabled), boolTestByte(settings.ReactionsEnabled)})
	return socialapp.InteractionSettingsCommand{ActorUserID: actor, Settings: settings, OccurredAt: at,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: socialapp.UpdateInteractionSettingsOperation, Key: key, RequestHash: digest[:]},
		Audit:       audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: audit.ResourceUpdated, TargetType: "social_interaction_settings", TargetID: actor, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}}
}

func boolTestByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}
