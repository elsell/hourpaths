package gormstore

import (
	"context"
	"testing"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
)

func TestPostgresBlockHidesRetainedSocialEngagementOnlyFromTheBlockedPair(t *testing.T) {
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
	owner := socialRelationshipTestUser(t, migrationStore.DB, "blockengagementowner", identity.ProfileVisibilityPublic, now)
	hidden := socialRelationshipTestUser(t, migrationStore.DB, "blockengagementhidden", identity.ProfileVisibilityPublic, now)
	unrelated := socialRelationshipTestUser(t, migrationStore.DB, "blockengagementunrelated", identity.ProfileVisibilityPublic, now)
	users := []userModel{owner, hidden, unrelated}
	pathID, activityID := "block-engagement-path-"+newTestID(), "block-engagement-activity-"+newTestID()
	eventID := "practice:" + activityID
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		ids := make([]string, 0, len(users))
		for _, user := range users {
			ids = append(ids, user.ID)
		}
		migrationStore.DB.Table("user_models").Where("id IN ?", ids).Delete(&struct{ ID string }{})
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
	if err := migrationStore.DB.Table("follow_models").Create([]socialFollowTestModel{
		{FollowerUserID: hidden.ID, FollowingUserID: owner.ID, CreatedAt: now},
		{FollowerUserID: unrelated.ID, FollowingUserID: owner.ID, CreatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	hiddenComment := socialCommentModel{ID: "block-hidden-comment-" + newTestID(), SocialFeedEventID: eventID, AuthorUserID: hidden.ID, Body: "Hidden encouragement", Version: 1, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)}
	unrelatedComment := socialCommentModel{ID: "block-unrelated-comment-" + newTestID(), SocialFeedEventID: eventID, AuthorUserID: unrelated.ID, Body: "Visible encouragement", Version: 1, CreatedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second)}
	if err := migrationStore.DB.Create([]socialCommentModel{hiddenComment, unrelatedComment}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Create(&socialReactionModel{SocialFeedEventID: eventID, ActorUserID: hidden.ID, ReactionType: string(socialdomain.ReactionFire), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Create(&socialCommentHeartModel{CommentID: unrelatedComment.ID, ActorUserID: hidden.ID, CreatedAt: now.Add(3 * time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	block := socialBlockTestModel{BlockerUserID: owner.ID, BlockedUserID: hidden.ID, CreatedAt: now.Add(4 * time.Second)}
	if err := migrationStore.DB.Table("block_models").Create(&block).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewSocialFeedRepository(runtimeStore.DB)
	snapshot := now.Add(time.Minute)
	ownerFeed, err := repository.GetPracticeCandidate(context.Background(), owner.ID, eventID, snapshot)
	if err != nil || ownerFeed.Reactions.Counts != (socialdomain.ReactionCounts{}) || ownerFeed.Reactions.ViewerReaction != "" {
		t.Fatalf("blocked-pair owner feed=%+v err=%v", ownerFeed, err)
	}
	unrelatedFeed, err := repository.GetPracticeCandidate(context.Background(), unrelated.ID, eventID, snapshot)
	if err != nil || unrelatedFeed.Reactions.Counts.Fire != 1 {
		t.Fatalf("unrelated viewer feed=%+v err=%v", unrelatedFeed, err)
	}

	ownerComments, err := repository.ListPracticeComments(context.Background(), owner.ID, eventID, socialapp.CommentPageRequest{Snapshot: snapshot, Limit: 10})
	if err != nil || len(ownerComments.Items) != 1 || ownerComments.Items[0].Comment.ID != unrelatedComment.ID || ownerComments.Items[0].HeartCount != 0 {
		t.Fatalf("blocked-pair owner comments=%+v err=%v", ownerComments, err)
	}
	ownerRoster, err := repository.ListPracticeCommentHearts(context.Background(), owner.ID, eventID, unrelatedComment.ID, socialapp.CommentHeartRosterPageRequest{Snapshot: snapshot, Limit: 10})
	if err != nil || len(ownerRoster.Items) != 0 {
		t.Fatalf("blocked-pair owner roster=%+v err=%v", ownerRoster, err)
	}
	unrelatedComments, err := repository.ListPracticeComments(context.Background(), unrelated.ID, eventID, socialapp.CommentPageRequest{Snapshot: snapshot, Limit: 10})
	if err != nil || len(unrelatedComments.Items) != 2 || unrelatedComments.Items[1].Comment.ID != unrelatedComment.ID || unrelatedComments.Items[1].HeartCount != 1 {
		t.Fatalf("unrelated viewer comments=%+v err=%v", unrelatedComments, err)
	}

	if err := migrationStore.DB.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", owner.ID, hidden.ID).Delete(&socialBlockTestModel{}).Error; err != nil {
		t.Fatal(err)
	}
	restoredFeed, err := repository.GetPracticeCandidate(context.Background(), owner.ID, eventID, snapshot)
	if err != nil || restoredFeed.Reactions.Counts.Fire != 1 {
		t.Fatalf("restored owner feed=%+v err=%v", restoredFeed, err)
	}
	restoredComments, err := repository.ListPracticeComments(context.Background(), owner.ID, eventID, socialapp.CommentPageRequest{Snapshot: snapshot, Limit: 10})
	if err != nil || len(restoredComments.Items) != 2 || restoredComments.Items[1].HeartCount != 1 {
		t.Fatalf("restored owner comments=%+v err=%v", restoredComments, err)
	}
}
