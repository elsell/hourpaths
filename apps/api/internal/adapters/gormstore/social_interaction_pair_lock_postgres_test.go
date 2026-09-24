package gormstore

import (
	"context"
	"errors"
	"testing"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresDirectSocialMutationsSerializeBehindBlocking(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	for _, test := range []struct {
		name   string
		mutate func(*SocialFeedRepository, socialInteractionRaceFixture) error
	}{
		{name: "reaction", mutate: func(repository *SocialFeedRepository, fixture socialInteractionRaceFixture) error {
			_, err := repository.SetPracticeReaction(context.Background(), socialReactionTestCommand(fixture.actor.ID, fixture.reactionTarget(), socialdomain.ReactionHeart, socialapp.SetPracticeReactionOperation, "blocked-race-reaction", fixture.now))
			return err
		}},
		{name: "comment create", mutate: func(repository *SocialFeedRepository, fixture socialInteractionRaceFixture) error {
			_, err := repository.CreatePracticeComment(context.Background(), socialCommentTestCommand(fixture.actor.ID, fixture.commentTarget(), "Race-safe comment", "", 0, socialapp.CreatePracticeCommentOperation, "blocked-race-create", fixture.now))
			return err
		}},
		{name: "comment edit", mutate: func(repository *SocialFeedRepository, fixture socialInteractionRaceFixture) error {
			_, err := repository.EditPracticeComment(context.Background(), socialCommentTestCommand(fixture.actor.ID, fixture.commentTarget(), "Race-safe edit", fixture.actorComment.ID, 1, socialapp.EditPracticeCommentOperation, "blocked-race-edit", fixture.now))
			return err
		}},
		{name: "comment delete", mutate: func(repository *SocialFeedRepository, fixture socialInteractionRaceFixture) error {
			_, err := repository.DeletePracticeComment(context.Background(), socialCommentTestCommand(fixture.actor.ID, fixture.commentTarget(), "", fixture.actorComment.ID, 0, socialapp.DeletePracticeCommentOperation, "blocked-race-delete", fixture.now))
			return err
		}},
		{name: "comment heart", mutate: func(repository *SocialFeedRepository, fixture socialInteractionRaceFixture) error {
			_, err := repository.SetPracticeCommentHeart(context.Background(), socialCommentHeartTestCommand(fixture.actor.ID, fixture.commentTarget(), fixture.ownerComment, true, "blocked-race-heart", fixture.now))
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtimeStore, migrationStore, fixture := seedSocialInteractionRaceFixture(t)
			runBlockedSocialMutationRace(t, runtimeStore, migrationStore, fixture, test.mutate)
		})
	}
}

type socialInteractionRaceFixture struct {
	actor, owner                userModel
	pathID, activityID, eventID string
	actorComment, ownerComment  socialdomain.Comment
	now                         time.Time
}

func (fixture socialInteractionRaceFixture) reactionTarget() socialapp.ReactionTarget {
	return socialapp.ReactionTarget{EventID: fixture.eventID, PathID: fixture.pathID, OwnerUserID: fixture.owner.ID}
}

func (fixture socialInteractionRaceFixture) commentTarget() socialapp.PracticeCommentTarget {
	return socialapp.PracticeCommentTarget{EventID: fixture.eventID, PathID: fixture.pathID, OwnerUserID: fixture.owner.ID}
}

func seedSocialInteractionRaceFixture(t *testing.T) (*Store, *Store, socialInteractionRaceFixture) {
	t.Helper()
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
	actor := socialRelationshipTestUser(t, migrationStore.DB, "raceactor", identity.ProfileVisibilityPublic, now)
	owner := socialRelationshipTestUser(t, migrationStore.DB, "raceowner", identity.ProfileVisibilityPublic, now)
	fixture := socialInteractionRaceFixture{actor: actor, owner: owner, pathID: "race-path-" + newTestID(), activityID: "race-activity-" + newTestID(), now: now}
	fixture.eventID = "practice:" + fixture.activityID
	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		CreatedAt, UpdatedAt              time.Time
	}
	type activityRow struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
	}
	if err := migrationStore.DB.Table("path_models").Create(&pathRow{ID: fixture.pathID, OwnerUserID: owner.ID, Name: "Race", Visibility: "followers", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("recorded_activity_models").Create(&activityRow{ID: fixture.activityID, PathID: fixture.pathID, ParticipantID: owner.ID, OccurrenceTimeZone: "Etc/UTC", StartedAt: now.Add(-time.Hour), EndedAt: now.Add(-time.Minute), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: actor.ID, FollowingUserID: owner.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	fixture.actorComment = socialdomain.Comment{ID: "race-actor-comment-" + newTestID(), EventID: fixture.eventID, AuthorID: actor.ID, Text: "Actor", Version: 1, CreatedAt: now, UpdatedAt: now}
	fixture.ownerComment = socialdomain.Comment{ID: "race-owner-comment-" + newTestID(), EventID: fixture.eventID, AuthorID: owner.ID, Text: "Owner", Version: 1, CreatedAt: now, UpdatedAt: now}
	for _, comment := range []socialdomain.Comment{fixture.actorComment, fixture.ownerComment} {
		if err := migrationStore.DB.Create(&socialCommentModel{ID: comment.ID, SocialFeedEventID: comment.EventID, AuthorUserID: comment.AuthorID, Body: comment.Text, Version: comment.Version, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", fixture.pathID).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{actor.ID, owner.ID}).Delete(&struct{ ID string }{})
	})
	return runtimeStore, migrationStore, fixture
}

func runBlockedSocialMutationRace(t *testing.T, runtimeStore, migrationStore *Store, fixture socialInteractionRaceFixture, mutate func(*SocialFeedRepository, socialInteractionRaceFixture) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	sqlDB, err := runtimeStore.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	mutationConnection, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mutationConnection.Close() })
	observerConnection, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = observerConnection.Close() })
	var mutationBackendPID int
	if err := mutationConnection.QueryRowContext(ctx, "SELECT pg_catalog.pg_backend_pid()").Scan(&mutationBackendPID); err != nil {
		t.Fatal(err)
	}
	mutationDB, err := gorm.Open(postgres.New(postgres.Config{Conn: mutationConnection}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	blocking := runtimeStore.DB.WithContext(ctx).Begin()
	if blocking.Error != nil {
		t.Fatal(blocking.Error)
	}
	t.Cleanup(func() { _ = blocking.Rollback().Error })
	if err := lockSocialPair(blocking, fixture.actor.ID, fixture.owner.ID); err != nil {
		t.Fatal(err)
	}
	completed := make(chan error, 1)
	go func() { completed <- mutate(NewSocialFeedRepository(mutationDB), fixture) }()
	waitForPostgresLock(t, ctx, observerConnection, mutationBackendPID, completed)
	if err := blocking.Create(&socialBlockModel{BlockerUserID: fixture.owner.ID, BlockedUserID: fixture.actor.ID, CreatedAt: fixture.now.Add(time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := blocking.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-completed:
		if !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("mutation after concurrent block error=%v, want opaque not found", err)
		}
	case <-ctx.Done():
		t.Fatalf("mutation remained blocked: %v", context.Cause(ctx))
	}
	var interactionAudits int64
	if err := migrationStore.DB.Table("audit_event_models").Where("actor_user_id = ? AND occurred_at = ?", fixture.actor.ID, fixture.now).Count(&interactionAudits).Error; err != nil {
		t.Fatal(err)
	}
	if interactionAudits != 0 {
		t.Fatalf("blocked concurrent mutation wrote %d success audits", interactionAudits)
	}
}
