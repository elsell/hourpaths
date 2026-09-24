package gormstore

import (
	"context"
	"errors"
	"testing"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresSocialFeedCandidatesAreCurrentSafeChronologicalAndStable(t *testing.T) {
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

	now := time.Now().UTC().Truncate(time.Microsecond)
	visibility := identity.ProfileVisibilityPublic
	viewer := socialRelationshipTestUser(t, migrationStore.DB, "feedviewer", visibility, now)
	followed := socialRelationshipTestUser(t, migrationStore.DB, "feedfollowed", visibility, now)
	shared := socialRelationshipTestUser(t, migrationStore.DB, "feedshared", visibility, now)
	unrelated := socialRelationshipTestUser(t, migrationStore.DB, "feedunrelated", visibility, now)
	blocked := socialRelationshipTestUser(t, migrationStore.DB, "feedblocked", visibility, now)
	users := []userModel{viewer, followed, shared, unrelated, blocked}
	pathIDs := make([]string, 0, 4)
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id IN ?", pathIDs).Delete(&struct{ ID string }{})
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
	type membershipRow struct{ PathID, UserID, Role string }
	type activityRow struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
	}
	type revisionRow struct {
		ActivityID, OccurrenceTimeZone            string
		Version                                   int64
		StartedAt, EndedAt, UpdatedAt, ReplacedAt time.Time
		PublicChanged                             bool
	}

	seedEvent := func(prefix string, source userModel, published time.Time) (string, string) {
		t.Helper()
		pathID, activityID := prefix+"-path-"+newTestID(), prefix+"-activity-"+newTestID()
		pathIDs = append(pathIDs, pathID)
		if err := migrationStore.DB.Table("path_models").Create(&pathRow{ID: pathID, OwnerUserID: source.ID, Name: prefix + " Path", Visibility: "public", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationStore.DB.Table("recorded_activity_models").Create(&activityRow{
			ID: activityID, PathID: pathID, ParticipantID: source.ID,
			StartedAt: published.Add(-10 * time.Minute), EndedAt: published.Add(-5 * time.Minute),
			OccurrenceTimeZone: "Etc/UTC", CreatedAt: published, UpdatedAt: published,
		}).Error; err != nil {
			t.Fatal(err)
		}
		return pathID, activityID
	}

	unrelatedPath, _ := seedEvent("Unrelated", unrelated, now.Add(-time.Minute))
	blockedPath, _ := seedEvent("Blocked", blocked, now.Add(-2*time.Minute))
	followedPath, followedActivity := seedEvent("Followed", followed, now.Add(-3*time.Minute))
	sharedPath, sharedActivity := seedEvent("Shared", shared, now.Add(-4*time.Minute))
	_ = blockedPath
	if err := migrationStore.DB.Table("follow_models").Create([]socialFollowTestModel{
		{FollowerUserID: viewer.ID, FollowingUserID: followed.ID, CreatedAt: now},
		{FollowerUserID: viewer.ID, FollowingUserID: blocked.ID, CreatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("block_models").Create(&socialBlockTestModel{BlockerUserID: blocked.ID, BlockedUserID: viewer.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("path_membership_models").Create([]membershipRow{
		{PathID: sharedPath, UserID: viewer.ID, Role: "participant"},
		{PathID: unrelatedPath, UserID: viewer.ID, Role: "supporter"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("recorded_activity_revision_models").Create(&revisionRow{
		ActivityID: sharedActivity, Version: 1, StartedAt: now.Add(-20 * time.Minute), EndedAt: now.Add(-15 * time.Minute),
		OccurrenceTimeZone: "Etc/UTC", PublicChanged: true, UpdatedAt: now.Add(-4 * time.Minute), ReplacedAt: now.Add(-time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}
	achievementID := "feed-achievement-" + newTestID()
	achievementPublishedAt := now.Add(-150 * time.Second)
	if err := migrationStore.DB.Table("social_goal_achievement_models").Create(map[string]any{
		"id": achievementID, "participant_user_id": followed.ID, "path_id": followedPath,
		"kind": "overall", "target_seconds": int64(300), "published_at": achievementPublishedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("social_feed_event_models").Create(map[string]any{
		"id": "achievement:" + achievementID, "achievement_id": achievementID,
		"participant_user_id": followed.ID, "path_id": followedPath, "published_at": achievementPublishedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewSocialFeedRepository(runtimeStore.DB)
	first, err := repository.ListPracticeCandidates(context.Background(), viewer.ID, socialapp.FeedPageRequest{Snapshot: now, Limit: 2})
	if err != nil || len(first.Items) != 2 || !first.HasMore {
		t.Fatalf("first candidates = %+v, %v", first, err)
	}
	if item := first.Items[0]; item.ID != "achievement:"+achievementID || item.Type != socialapp.FeedEventGoalAchievement ||
		item.ParticipantID != followed.ID || item.PathID != followedPath || item.Achievement == nil ||
		item.Achievement.Kind != socialapp.AchievementOverall || item.Achievement.TargetSeconds != 300 || item.ActivityID != "" {
		t.Fatalf("achievement candidate = %+v", item)
	}
	if item := first.Items[1]; item.ID != "practice:"+followedActivity || item.Type != socialapp.FeedEventPracticeSession || item.ParticipantID != followed.ID || item.PathID != followedPath || item.DurationSeconds != 300 || item.Edited {
		t.Fatalf("followed candidate = %+v", item)
	}
	second, err := repository.ListPracticeCandidates(context.Background(), viewer.ID, socialapp.FeedPageRequest{
		AfterID: first.Items[1].ID, AfterPublished: first.Items[1].PublishedAt, Snapshot: now, Limit: 1,
	})
	if err != nil || len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second candidates = %+v, %v", second, err)
	}
	if item := second.Items[0]; item.ID != "practice:"+sharedActivity || item.ParticipantID != shared.ID || item.PathID != sharedPath || !item.Edited {
		t.Fatalf("shared edited candidate = %+v", item)
	}
	if err := migrationStore.DB.Table("path_models").Where("id = ?", followedPath).Update("visibility", "private").Error; err != nil {
		t.Fatal(err)
	}
	narrowed, err := repository.ListPracticeCandidates(context.Background(), viewer.ID, socialapp.FeedPageRequest{Snapshot: now, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range narrowed.Items {
		if item.PathID == followedPath {
			t.Fatalf("narrowed Path remained in feed candidate page: %+v", item)
		}
	}
}

func TestPostgresRetainedFeedAndAchievementRequireCurrentSourceMembershipAndRestoreOnRejoin(t *testing.T) {
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
	now := time.Now().UTC().Truncate(time.Microsecond)
	visibility := identity.ProfileVisibilityPublic
	owner := socialRelationshipTestUser(t, migrationStore.DB, "retainedfeedowner", visibility, now)
	source := socialRelationshipTestUser(t, migrationStore.DB, "retainedfeedsource", visibility, now)
	viewer := socialRelationshipTestUser(t, migrationStore.DB, "retainedfeedviewer", visibility, now)
	pathID, activityID := "retained-feed-path-"+newTestID(), "retained-feed-activity-"+newTestID()
	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		CreatedAt, UpdatedAt              time.Time
	}
	type membershipRow struct{ PathID, UserID, Role string }
	if err := migrationStore.DB.Table("path_models").Create(&pathRow{ID: pathID, OwnerUserID: owner.ID, Name: "Retained Feed", Visibility: "public", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: source.ID, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: viewer.ID, FollowingUserID: source.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	activity := map[string]any{"id": activityID, "path_id": pathID, "participant_id": source.ID, "started_at": now.Add(-time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now}
	if err := migrationStore.DB.Table("recorded_activity_models").Create(activity).Error; err != nil {
		t.Fatal(err)
	}
	achievementID := "retained-achievement-" + newTestID()
	if err := migrationStore.DB.Table("social_goal_achievement_models").Create(map[string]any{"id": achievementID, "participant_user_id": source.ID, "path_id": pathID, "kind": "overall", "target_seconds": int64(60), "published_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	// The recorded-activity insert publishes its practice event through the
	// database trigger. Seed only the achievement event explicitly.
	if err := migrationStore.DB.Table("social_feed_event_models").Create(map[string]any{"id": "achievement:" + achievementID, "achievement_id": achievementID, "participant_user_id": source.ID, "path_id": pathID, "published_at": now.Add(time.Microsecond)}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{owner.ID, source.ID, viewer.ID}).Delete(&struct{ ID string }{})
	})
	repository := NewSocialFeedRepository(runtimeStore.DB)
	visible, err := repository.ListPracticeCandidates(context.Background(), viewer.ID, socialapp.FeedPageRequest{Snapshot: now.Add(time.Second), Limit: 10})
	if err != nil || len(visible.Items) != 2 {
		t.Fatalf("visible feed=%+v, %v", visible, err)
	}
	if err := migrationStore.DB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, source.ID).Delete(&membershipRow{}).Error; err != nil {
		t.Fatal(err)
	}
	hidden, err := repository.ListPracticeCandidates(context.Background(), viewer.ID, socialapp.FeedPageRequest{Snapshot: now.Add(time.Second), Limit: 10})
	if err != nil || len(hidden.Items) != 0 {
		t.Fatalf("retained feed=%+v, %v", hidden, err)
	}
	for _, id := range []string{"practice:" + activityID, "achievement:" + achievementID} {
		if _, err := repository.GetPracticeCandidate(context.Background(), viewer.ID, id, now.Add(time.Second)); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("retained event %s error=%v, want opaque not found", id, err)
		}
	}
	if err := migrationStore.DB.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: source.ID, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	restored, err := repository.ListPracticeCandidates(context.Background(), viewer.ID, socialapp.FeedPageRequest{Snapshot: now.Add(time.Second), Limit: 10})
	if err != nil || len(restored.Items) != 2 {
		t.Fatalf("restored feed=%+v, %v", restored, err)
	}
}
