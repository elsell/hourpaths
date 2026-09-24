package gormstore

import (
	"context"
	"strings"
	"testing"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
)

type socialFollowTestModel struct {
	FollowerUserID, FollowingUserID string
	CreatedAt                       time.Time
}

func (socialFollowTestModel) TableName() string { return "follow_models" }

type socialBlockTestModel struct {
	BlockerUserID, BlockedUserID string
	CreatedAt                    time.Time
}

func (socialBlockTestModel) TableName() string { return "block_models" }

func TestSocialProfilesRankActiveMatchesPageAndExcludeBlocksInEitherDirection(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	suffix := strings.ReplaceAll(newTestID(), "-", "")[:10]
	query := "ali" + suffix
	viewerID := "social-viewer-" + newTestID()
	exactID := "social-exact-" + newTestID()
	prefixID := "social-prefix-" + newTestID()
	displayPrefixID := "social-display-prefix-" + newTestID()
	substringID := "social-substring-" + newTestID()
	unicodeID := "social-unicode-" + newTestID()
	blockedByViewerID := "social-blocked-by-viewer-" + newTestID()
	blockedViewerID := "social-blocked-viewer-" + newTestID()
	disabledID := "social-disabled-" + newTestID()
	exactUsername := query
	prefixUsername := query + "ce"
	displayPrefixUsername := "betty" + suffix
	substringUsername := "carol" + suffix
	unicodeUsername := "unicode" + suffix
	users := []userModel{
		{ID: viewerID, Username: stringPointer("viewer" + suffix), DisplayName: "Viewer", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: exactID, Username: &exactUsername, DisplayName: "Zed", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: prefixID, Username: &prefixUsername, DisplayName: "Zed", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: displayPrefixID, Username: &displayPrefixUsername, DisplayName: query + " runner", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: substringID, Username: &substringUsername, DisplayName: "P" + query, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: unicodeID, Username: &unicodeUsername, DisplayName: "A\u0301L " + suffix, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: blockedByViewerID, Username: stringPointer(query + "x"), DisplayName: "Blocked", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: blockedViewerID, Username: stringPointer(query + "y"), DisplayName: "Blocked", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: disabledID, Username: stringPointer(query + "z"), DisplayName: "Disabled", Status: identity.StatusDisabled, CreatedAt: now, UpdatedAt: now},
	}
	blocks := []socialBlockTestModel{
		{BlockerUserID: viewerID, BlockedUserID: blockedByViewerID, CreatedAt: now},
		{BlockerUserID: blockedViewerID, BlockedUserID: viewerID, CreatedAt: now},
	}
	repository := seedSocialProfilePostgresFixtures(t, ctx, users, nil, blocks)

	first, err := repository.Search(ctx, viewerID, "ál", -1, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Profiles) != 1 || first.Profiles[0].Username != unicodeUsername || first.HasMore {
		t.Fatalf("Unicode page=%+v", first)
	}
	page, err := repository.Search(ctx, viewerID, query, -1, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Profiles) != 2 || page.Profiles[0].Username != exactUsername || page.Profiles[1].Username != prefixUsername || page.LastRank != 1 || !page.HasMore {
		t.Fatalf("first page=%+v", page)
	}
	next, err := repository.Search(ctx, viewerID, query, page.LastRank, prefixUsername, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Profiles) != 2 || next.Profiles[0].Username != displayPrefixUsername || next.Profiles[1].Username != substringUsername || next.HasMore {
		t.Fatalf("next page=%+v", next)
	}
	_ = socialapp.ProfilePage{} // compile-time ownership of the application port
}

func TestSocialProfileDetailCountsOnlyActiveUnblockedRelationshipsAndNeverReturnsBlockedTargets(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	suffix := strings.ReplaceAll(newTestID(), "-", "")[:10]
	viewerID := "social-detail-viewer-" + newTestID()
	targetID := "social-detail-target-" + newTestID()
	visibleFollowerID := "social-detail-visible-" + newTestID()
	blockedFollowerID := "social-detail-blocked-" + newTestID()
	disabledFollowingID := "social-detail-disabled-" + newTestID()
	targetUsername := "target" + suffix
	users := []userModel{
		{ID: viewerID, Username: stringPointer("viewer" + suffix), DisplayName: "Viewer", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: targetID, Username: &targetUsername, DisplayName: "Target", Description: stringPointer("Safe description"), Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: visibleFollowerID, Username: stringPointer("visible" + suffix), DisplayName: "Visible", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: blockedFollowerID, Username: stringPointer("blocked" + suffix), DisplayName: "Blocked", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: disabledFollowingID, Username: stringPointer("disabled" + suffix), DisplayName: "Disabled", Status: identity.StatusDisabled, CreatedAt: now, UpdatedAt: now},
	}
	follows := []socialFollowTestModel{
		{FollowerUserID: viewerID, FollowingUserID: targetID, CreatedAt: now},
		{FollowerUserID: visibleFollowerID, FollowingUserID: targetID, CreatedAt: now},
		{FollowerUserID: blockedFollowerID, FollowingUserID: targetID, CreatedAt: now},
		{FollowerUserID: targetID, FollowingUserID: disabledFollowingID, CreatedAt: now},
	}
	blocks := []socialBlockTestModel{{BlockerUserID: targetID, BlockedUserID: blockedFollowerID, CreatedAt: now}}
	repository := seedSocialProfilePostgresFixtures(t, ctx, users, follows, blocks)

	profile, err := repository.GetByUsername(ctx, viewerID, targetUsername)
	if err != nil || profile.FollowerCount != 2 || profile.FollowingCount != 0 || profile.Description != "Safe description" || profile.Relationship != domain.RelationshipFollowing {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, migrationStore)
	if err := migrationStore.DB.WithContext(ctx).Create(&socialBlockTestModel{BlockerUserID: targetID, BlockedUserID: viewerID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetByUsername(ctx, viewerID, targetUsername); err == nil {
		t.Fatal("blocked target was returned")
	}
}

func seedSocialProfilePostgresFixtures(t *testing.T, ctx context.Context, users []userModel, follows []socialFollowTestModel, blocks []socialBlockTestModel) *SocialProfileRepository {
	t.Helper()
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeStore, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, runtimeStore)
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, migrationStore)

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	t.Cleanup(func() {
		db := migrationStore.DB.WithContext(context.Background())
		if err := db.Where("blocker_user_id IN ? OR blocked_user_id IN ?", userIDs, userIDs).Delete(&socialBlockTestModel{}).Error; err != nil {
			t.Errorf("clean social block fixtures: %v", err)
		}
		if err := db.Where("follower_user_id IN ? OR following_user_id IN ?", userIDs, userIDs).Delete(&socialFollowTestModel{}).Error; err != nil {
			t.Errorf("clean social follow fixtures: %v", err)
		}
		if err := db.Where("id IN ?", userIDs).Delete(&userModel{}).Error; err != nil {
			t.Errorf("clean social user fixtures: %v", err)
		}
	})
	if err := migrationStore.DB.WithContext(ctx).Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	if len(follows) > 0 {
		if err := migrationStore.DB.WithContext(ctx).Create(&follows).Error; err != nil {
			t.Fatal(err)
		}
	}
	if len(blocks) > 0 {
		if err := migrationStore.DB.WithContext(ctx).Create(&blocks).Error; err != nil {
			t.Fatal(err)
		}
	}
	return NewSocialProfileRepository(runtimeStore.DB)
}

func closeSocialProfileTestStore(t *testing.T, store *Store) {
	t.Helper()
	sqlDB, err := store.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}

func stringPointer(value string) *string { return &value }
