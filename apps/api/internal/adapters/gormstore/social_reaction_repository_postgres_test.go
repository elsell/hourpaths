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
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPracticeReactionForeignKeyRaceIsOpaque(t *testing.T) {
	if err := classifySocialReactionError(gorm.ErrForeignKeyViolated); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("foreign-key race error=%v, want opaque not found", err)
	}
}

func TestPostgresAchievementEventSupportsReactionsCommentsHeartsGraceAndOpaqueAuthorization(t *testing.T) {
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
	actor := socialRelationshipTestUser(t, migrationStore.DB, "achievementreactor", identity.ProfileVisibilityPublic, now)
	owner := socialRelationshipTestUser(t, migrationStore.DB, "achievementowner", identity.ProfileVisibilityPublic, now)
	outsider := socialRelationshipTestUser(t, migrationStore.DB, "achievementoutsider", identity.ProfileVisibilityPublic, now)
	pathID := "achievement-path-" + newTestID()
	achievementID := "achievement-" + newTestID()
	eventID := "achievement:" + achievementID
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{actor.ID, owner.ID, outsider.ID}).Delete(&struct{ ID string }{})
	})
	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		CreatedAt, UpdatedAt              time.Time
	}
	if err := migrationStore.DB.Table("path_models").Create(&pathRow{ID: pathID, OwnerUserID: owner.ID, Name: "Piano", Visibility: "public", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: actor.ID, FollowingUserID: owner.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("social_goal_achievement_models").Create(map[string]any{
		"id": achievementID, "participant_user_id": owner.ID, "path_id": pathID,
		"kind": "overall", "target_seconds": int64(300), "published_at": now.Add(-time.Second),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("social_feed_event_models").Create(map[string]any{
		"id": eventID, "achievement_id": achievementID, "participant_user_id": owner.ID,
		"path_id": pathID, "published_at": now.Add(-time.Second),
	}).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewSocialFeedRepository(runtimeStore.DB)
	reactionTarget := socialapp.ReactionTarget{EventID: eventID, PathID: pathID, OwnerUserID: owner.ID}
	resolvedReaction, err := repository.ResolvePracticeReactionTarget(context.Background(), actor.ID, eventID)
	if err != nil || resolvedReaction != reactionTarget {
		t.Fatalf("achievement reaction target=%+v err=%v", resolvedReaction, err)
	}
	if _, err := repository.ResolvePracticeReactionTarget(context.Background(), outsider.ID, eventID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("inaccessible achievement reaction error=%v, want opaque not found", err)
	}
	reactionCommand := socialReactionTestCommand(actor.ID, reactionTarget, socialdomain.ReactionCelebrate, socialapp.SetPracticeReactionOperation, "achievement-reaction", now)
	reacted, err := repository.SetPracticeReaction(context.Background(), reactionCommand)
	if err != nil || reacted.Summary.Counts.Celebrate != 1 || reacted.Summary.ViewerReaction != socialdomain.ReactionCelebrate {
		t.Fatalf("achievement reaction=%+v err=%v", reacted, err)
	}
	assertReactionPersistence(t, migrationStore, eventID, actor.ID, owner.ID, "celebrate", 1, now.Add(5*time.Second))
	commentTarget := socialapp.PracticeCommentTarget{EventID: eventID, PathID: pathID, OwnerUserID: owner.ID}
	resolvedComment, err := repository.ResolvePracticeCommentTarget(context.Background(), actor.ID, eventID)
	if err != nil || resolvedComment != commentTarget {
		t.Fatalf("achievement comment target=%+v err=%v", resolvedComment, err)
	}
	if _, err := repository.ResolvePracticeCommentTarget(context.Background(), outsider.ID, eventID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("inaccessible achievement comment error=%v, want opaque not found", err)
	}
	commentCommand := socialCommentTestCommand(actor.ID, commentTarget, "Nice work", "", 0, socialapp.CreatePracticeCommentOperation, "achievement-comment", now)
	created, err := repository.CreatePracticeComment(context.Background(), commentCommand)
	if err != nil || created.Comment.EventID != eventID || created.Comment.AuthorID != actor.ID {
		t.Fatalf("achievement comment=%+v err=%v", created, err)
	}
	assertCommentNotification(t, migrationStore.DB, created.Comment.ID, eventID, actor.ID, owner.ID, 1, now.Add(5*time.Second))
	comments, err := repository.ListPracticeComments(context.Background(), actor.ID, eventID, socialapp.CommentPageRequest{Snapshot: now.Add(time.Second), Limit: 10})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].Comment.ID != created.Comment.ID {
		t.Fatalf("achievement comments=%+v err=%v", comments, err)
	}
	editCommand := socialCommentTestCommand(actor.ID, commentTarget, "Really nice work", created.Comment.ID, 1, socialapp.EditPracticeCommentOperation, "achievement-edit-comment", now.Add(time.Second))
	edited, err := repository.EditPracticeComment(context.Background(), editCommand)
	if err != nil || edited.Comment.Version != 2 || edited.Comment.Text != "Really nice work" {
		t.Fatalf("edited achievement comment=%+v err=%v", edited, err)
	}
	history, err := repository.ListCommentHistory(context.Background(), actor.ID, eventID, created.Comment.ID, socialapp.CommentHistoryPageRequest{Snapshot: now.Add(2 * time.Second), Limit: 10})
	if err != nil || len(history.Versions) != 2 || history.Versions[0].Version != 1 || history.Versions[1].Version != 2 {
		t.Fatalf("achievement comment history=%+v err=%v", history, err)
	}
	assertCommentNotification(t, migrationStore.DB, created.Comment.ID, eventID, actor.ID, owner.ID, 1, now.Add(5*time.Second))

	heartCommand := socialCommentHeartTestCommand(owner.ID, commentTarget, edited.Comment, true, "achievement-heart", now.Add(2*time.Second))
	hearted, err := repository.SetPracticeCommentHeart(context.Background(), heartCommand)
	if err != nil || hearted.HeartCount != 1 || !hearted.HeartedByViewer {
		t.Fatalf("achievement comment heart=%+v err=%v", hearted, err)
	}
	assertCommentHeartNotification(t, migrationStore.DB, created.Comment.ID, owner.ID, actor.ID, 1, now.Add(7*time.Second))
	roster, err := repository.ListPracticeCommentHearts(context.Background(), actor.ID, eventID, created.Comment.ID, socialapp.CommentHeartRosterPageRequest{Snapshot: now.Add(3 * time.Second), Limit: 10})
	if err != nil || len(roster.Items) != 1 || roster.Items[0].Profile.ID != owner.ID {
		t.Fatalf("achievement heart roster=%+v err=%v", roster, err)
	}
	feed, err := repository.ListPracticeCandidates(context.Background(), actor.ID, socialapp.FeedPageRequest{Snapshot: now, Limit: 1})
	if err != nil || len(feed.Items) != 1 || feed.Items[0].ID != eventID || feed.Items[0].Type != socialapp.FeedEventGoalAchievement ||
		feed.Items[0].Reactions != reacted.Summary || !feed.Items[0].PublishedAt.Equal(now.Add(-time.Second)) {
		t.Fatalf("achievement feed engagement=%+v err=%v", feed, err)
	}
	detail, err := repository.GetPracticeCandidate(context.Background(), actor.ID, eventID, now)
	if err != nil || detail.ID != eventID || !detail.CommentsEnabled || !detail.ReactionsEnabled || detail.Reactions != reacted.Summary {
		t.Fatalf("achievement detail=%+v err=%v", detail, err)
	}

	removeHeart := socialCommentHeartTestCommand(owner.ID, commentTarget, edited.Comment, false, "achievement-unheart", now.Add(3*time.Second))
	if _, err := repository.RemovePracticeCommentHeart(context.Background(), removeHeart); err != nil {
		t.Fatal(err)
	}
	assertCommentHeartNotification(t, migrationStore.DB, created.Comment.ID, owner.ID, actor.ID, 0, time.Time{})
	removeReaction := socialReactionTestCommand(actor.ID, reactionTarget, "", socialapp.RemovePracticeReactionOperation, "achievement-unreact", now.Add(6*time.Second))
	if _, err := repository.RemovePracticeReaction(context.Background(), removeReaction); err != nil {
		t.Fatal(err)
	}
	assertReactionPersistence(t, migrationStore, eventID, actor.ID, owner.ID, "", 0, time.Time{})
	deleteComment := socialCommentTestCommand(actor.ID, commentTarget, "", created.Comment.ID, 0, socialapp.DeletePracticeCommentOperation, "achievement-delete-comment", now.Add(6*time.Second))
	if _, err := repository.DeletePracticeComment(context.Background(), deleteComment); err != nil {
		t.Fatal(err)
	}
	assertCommentRemoved(t, migrationStore.DB, created.Comment.ID)
}

func TestPostgresRetainedActivityInteractionsRequireCurrentSourceMembershipAndRestoreOnRejoin(t *testing.T) {
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
	owner := socialRelationshipTestUser(t, migrationStore.DB, "retainedinteractionowner", identity.ProfileVisibilityPublic, now)
	source := socialRelationshipTestUser(t, migrationStore.DB, "retainedinteractionsource", identity.ProfileVisibilityPublic, now)
	viewer := socialRelationshipTestUser(t, migrationStore.DB, "retainedinteractionviewer", identity.ProfileVisibilityPublic, now)
	pathID, activityID := "retained-interaction-path-"+newTestID(), "retained-interaction-activity-"+newTestID()
	eventID := "practice:" + activityID
	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		CreatedAt, UpdatedAt              time.Time
	}
	type membershipRow struct{ PathID, UserID, Role string }
	type activityRow struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
	}
	if err := migrationStore.DB.Table("path_models").Create(&pathRow{ID: pathID, OwnerUserID: owner.ID, Name: "Retained interactions", Visibility: "public", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: source.ID, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: viewer.ID, FollowingUserID: source.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("recorded_activity_models").Create(&activityRow{ID: activityID, PathID: pathID, ParticipantID: source.ID, OccurrenceTimeZone: "Etc/UTC", StartedAt: now.Add(-time.Minute), EndedAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{owner.ID, source.ID, viewer.ID}).Delete(&struct{ ID string }{})
	})

	repository := NewSocialFeedRepository(runtimeStore.DB)
	reactionTarget := socialapp.ReactionTarget{EventID: eventID, PathID: pathID, OwnerUserID: source.ID}
	commentTarget := socialapp.PracticeCommentTarget(reactionTarget)
	if resolved, err := repository.ResolvePracticeReactionTarget(context.Background(), viewer.ID, eventID); err != nil || resolved != reactionTarget {
		t.Fatalf("initial reaction target=%+v err=%v", resolved, err)
	}
	created, err := repository.CreatePracticeComment(context.Background(), socialCommentTestCommand(viewer.ID, commentTarget, "Before leaving", "", 0, socialapp.CreatePracticeCommentOperation, "retained-comment-before-leave", now.Add(time.Second)))
	if err != nil {
		t.Fatal(err)
	}

	deleteMembership := func() {
		t.Helper()
		if err := migrationStore.DB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, source.ID).Delete(&membershipRow{}).Error; err != nil {
			t.Fatal(err)
		}
	}
	addMembership := func(role string) {
		t.Helper()
		if err := migrationStore.DB.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: source.ID, Role: role}).Error; err != nil {
			t.Fatal(err)
		}
	}
	deleteMembership()

	if _, err := repository.ResolvePracticeReactionTarget(context.Background(), viewer.ID, eventID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("retained reaction target error=%v, want opaque not found", err)
	}
	if _, err := repository.ResolvePracticeCommentTarget(context.Background(), viewer.ID, eventID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("retained comment target error=%v, want opaque not found", err)
	}
	if _, err := repository.SetPracticeReaction(context.Background(), socialReactionTestCommand(viewer.ID, reactionTarget, socialdomain.ReactionHeart, socialapp.SetPracticeReactionOperation, "retained-reaction-after-leave", now.Add(2*time.Second))); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("retained reaction mutation error=%v, want opaque not found", err)
	}
	if _, err := repository.CreatePracticeComment(context.Background(), socialCommentTestCommand(viewer.ID, commentTarget, "After leaving", "", 0, socialapp.CreatePracticeCommentOperation, "retained-comment-after-leave", now.Add(2*time.Second))); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("retained comment mutation error=%v, want opaque not found", err)
	}
	if _, err := repository.SetPracticeCommentHeart(context.Background(), socialCommentHeartTestCommand(owner.ID, commentTarget, created.Comment, true, "retained-heart-after-leave", now.Add(2*time.Second))); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("retained comment heart mutation error=%v, want opaque not found", err)
	}
	if _, err := repository.ListPracticeCommentHearts(context.Background(), viewer.ID, eventID, created.Comment.ID, socialapp.CommentHeartRosterPageRequest{Snapshot: now.Add(3 * time.Second), Limit: 10}); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("retained comment heart roster error=%v, want opaque not found", err)
	}

	for _, role := range []string{"administrator", "participant"} {
		addMembership(role)
		if resolved, err := repository.ResolvePracticeReactionTarget(context.Background(), viewer.ID, eventID); err != nil || resolved != reactionTarget {
			t.Fatalf("%s rejoin reaction target=%+v err=%v", role, resolved, err)
		}
		if resolved, err := repository.ResolvePracticeCommentTarget(context.Background(), viewer.ID, eventID); err != nil || resolved != commentTarget {
			t.Fatalf("%s rejoin comment target=%+v err=%v", role, resolved, err)
		}
		deleteMembership()
	}
}

func TestPostgresPracticeReactionSetReplaceReplayAndRemoveOwnOneDelayedNotification(t *testing.T) {
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
	assertRuntimeSocialFeedEventPrivileges(t, runtimeStore)

	now := time.Now().UTC().Truncate(time.Microsecond)
	actor := socialRelationshipTestUser(t, migrationStore.DB, "reactor", identity.ProfileVisibilityPublic, now)
	owner := socialRelationshipTestUser(t, migrationStore.DB, "practiceowner", identity.ProfileVisibilityPublic, now)
	pathID, activityID := "reaction-path-"+newTestID(), "reaction-activity-"+newTestID()
	eventID := "practice:" + activityID
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{actor.ID, owner.ID}).Delete(&struct{ ID string }{})
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
	if err := migrationStore.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: actor.ID, FollowingUserID: owner.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewSocialFeedRepository(runtimeStore.DB)
	target, err := repository.ResolvePracticeReactionTarget(context.Background(), actor.ID, eventID)
	if err != nil || target != (socialapp.ReactionTarget{EventID: eventID, PathID: pathID, OwnerUserID: owner.ID}) {
		t.Fatalf("target=%+v err=%v", target, err)
	}
	heart := socialReactionTestCommand(actor.ID, target, socialdomain.ReactionHeart, socialapp.SetPracticeReactionOperation, "reaction-heart-01", now)
	created, err := repository.SetPracticeReaction(context.Background(), heart)
	if err != nil || created.Replayed || created.Summary.ViewerReaction != socialdomain.ReactionHeart || created.Summary.Counts.Heart != 1 {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	assertReactionPersistence(t, migrationStore, eventID, actor.ID, owner.ID, "heart", 1, heart.NotificationEligibleAt)

	replayed, err := repository.SetPracticeReaction(context.Background(), heart)
	if err != nil || !replayed.Replayed || replayed.Summary.ViewerReaction != socialdomain.ReactionHeart {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	fire := socialReactionTestCommand(actor.ID, target, socialdomain.ReactionFire, socialapp.SetPracticeReactionOperation, "reaction-fire--01", now.Add(time.Second))
	replaced, err := repository.SetPracticeReaction(context.Background(), fire)
	if err != nil || replaced.Summary.Counts.Heart != 0 || replaced.Summary.Counts.Fire != 1 {
		t.Fatalf("replaced=%+v err=%v", replaced, err)
	}
	assertReactionPersistence(t, migrationStore, eventID, actor.ID, owner.ID, "fire", 1, heart.NotificationEligibleAt)
	feed, err := repository.ListPracticeCandidates(context.Background(), actor.ID, socialapp.FeedPageRequest{Snapshot: now.Add(3 * time.Second), Limit: 1})
	if err != nil || len(feed.Items) != 1 || feed.Items[0].ID != eventID || feed.Items[0].Reactions.ViewerReaction != socialdomain.ReactionFire || feed.Items[0].Reactions.Counts.Fire != 1 {
		t.Fatalf("feed reaction projection=%+v err=%v", feed, err)
	}

	remove := socialReactionTestCommand(actor.ID, target, "", socialapp.RemovePracticeReactionOperation, "reaction-remove-1", now.Add(2*time.Second))
	removed, err := repository.RemovePracticeReaction(context.Background(), remove)
	if err != nil || removed.Summary.ViewerReaction != "" || removed.Summary.Counts != (socialdomain.ReactionCounts{}) {
		t.Fatalf("removed=%+v err=%v", removed, err)
	}
	assertReactionPersistence(t, migrationStore, eventID, actor.ID, owner.ID, "", 0, time.Time{})
}

func assertRuntimeSocialFeedEventPrivileges(t *testing.T, store *Store) {
	t.Helper()
	var privileges struct {
		CanSelect   bool
		CanInsert   bool
		CanUpdate   bool
		CanDelete   bool
		CanTruncate bool
	}
	if err := store.DB.Raw(`SELECT
has_table_privilege(current_user, 'public.social_feed_event_models', 'SELECT') AS can_select,
has_table_privilege(current_user, 'public.social_feed_event_models', 'INSERT') AS can_insert,
has_table_privilege(current_user, 'public.social_feed_event_models', 'UPDATE') AS can_update,
has_table_privilege(current_user, 'public.social_feed_event_models', 'DELETE') AS can_delete,
has_table_privilege(current_user, 'public.social_feed_event_models', 'TRUNCATE') AS can_truncate`).Scan(&privileges).Error; err != nil {
		t.Fatal(err)
	}
	if !privileges.CanSelect || !privileges.CanInsert || privileges.CanUpdate || privileges.CanDelete || privileges.CanTruncate {
		t.Fatalf("runtime social-feed privileges=%+v, want SELECT/INSERT only", privileges)
	}
}

func socialReactionTestCommand(actor string, target socialapp.ReactionTarget, reaction socialdomain.Reaction, operation, key string, at time.Time) socialapp.ReactionCommand {
	digest := sha256.Sum256([]byte(operation + "\x00" + target.EventID + "\x00" + string(reaction)))
	action := audit.ResourceUpdated
	if operation == socialapp.RemovePracticeReactionOperation {
		action = audit.ResourceDeleted
	}
	return socialapp.ReactionCommand{ActorUserID: actor, Target: target, Reaction: reaction, OccurredAt: at, NotificationEligibleAt: at.Add(5 * time.Second),
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: digest[:]},
		Audit:       audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: action, TargetType: "practice_reaction", TargetID: target.EventID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}}
}

func assertReactionPersistence(t *testing.T, store *Store, eventID, actor, owner, reaction string, notificationCount int64, notificationAt time.Time) {
	t.Helper()
	var rows int64
	if err := store.DB.Table("social_practice_reaction_models").Where("social_feed_event_id = ? AND actor_user_id = ? AND reaction_type = ?", eventID, actor, reaction).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if reaction == "" {
		rows = 0
		if err := store.DB.Table("social_practice_reaction_models").Where("social_feed_event_id = ? AND actor_user_id = ?", eventID, actor).Count(&rows).Error; err != nil {
			t.Fatal(err)
		}
	}
	if (reaction == "" && rows != 0) || (reaction != "" && rows != 1) {
		t.Fatalf("reaction rows=%d", rows)
	}
	var notification struct {
		ReactionType string
		CreatedAt    time.Time
	}
	query := store.DB.Table("notification_models").Where("kind = 'practice_reaction' AND social_feed_event_id = ? AND actor_user_id = ? AND recipient_user_id = ?", eventID, actor, owner)
	if err := query.Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != notificationCount {
		t.Fatalf("notification rows=%d want=%d", rows, notificationCount)
	}
	if notificationCount == 1 {
		if err := query.Take(&notification).Error; err != nil {
			t.Fatal(err)
		}
		if notification.ReactionType != reaction || !notification.CreatedAt.Equal(notificationAt) {
			t.Fatalf("notification=%+v", notification)
		}
	}
}
