package activitystore

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresProtectedActivityReadsRejectStaleFollowerAuthorizationAfterBlock(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID, viewerID, pathID := "protected-owner-"+suffix, "protected-viewer-"+suffix, "protected-path-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedParticipantAndPath(t, migrationDB, ownerID, pathID, now)
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	if err := migrationDB.Table("user_models").Create(&userRow{ID: viewerID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_models").Where("id = ?", pathID).Update("visibility", "followers").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("follow_models").Create(map[string]any{"follower_user_id": viewerID, "following_user_id": ownerID, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	entry, err := domain.RecordManualActivity(domain.ManualActivity{ID: "protected-activity-" + suffix, PathID: pathID, ParticipantID: ownerID, StartedAt: now.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Create(fromActivity(entry)).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationDB.Table("user_models").Where("id IN ?", []string{ownerID, viewerID}).Delete(&userRow{})
	})

	repository := New(runtimeDB)
	if got, _, err := repository.GetActivity(context.Background(), viewerID, pathID, entry.ID); err != nil || got.ID != entry.ID {
		t.Fatalf("follower read before block=%+v err=%v", got, err)
	}
	if err := migrationDB.Table("block_models").Create(map[string]any{"blocker_user_id": ownerID, "blocked_user_id": viewerID, "created_at": now.Add(time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("follow_models").Where("follower_user_id = ? AND following_user_id = ?", viewerID, ownerID).Delete(&struct{}{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.GetActivity(context.Background(), viewerID, pathID, entry.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("stale protected activity detail read=%v, want not found", err)
	}
	page, err := repository.ListActivities(context.Background(), viewerID, pathID, application.ActivityPageRequest{Limit: 10, Snapshot: now.Add(2 * time.Second)})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("stale protected activity list=%+v err=%v", page, err)
	}
	if _, err := repository.ListActivityRevisions(context.Background(), viewerID, pathID, entry.ID, application.ActivityRevisionPageRequest{Limit: 10, Snapshot: now.Add(2 * time.Second)}); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("stale protected activity revisions=%v, want not found", err)
	}
}

func TestPostgresRetainedActivityReadsRequireCurrentSourceMembershipAndRestoreOnRejoin(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID, sourceID, followerID := "retained-owner-"+suffix, "retained-source-"+suffix, "retained-follower-"+suffix
	pathID, activityID := "retained-path-"+suffix, "retained-activity-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedParticipantAndPath(t, migrationDB, ownerID, pathID, now)
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	if err := migrationDB.Table("user_models").Create([]userRow{{ID: sourceID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}, {ID: followerID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_models").Where("id = ?", pathID).Update("visibility", "public").Error; err != nil {
		t.Fatal(err)
	}
	type membershipRow struct{ PathID, UserID, Role string }
	if err := migrationDB.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: sourceID, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("follow_models").Create(map[string]any{"follower_user_id": followerID, "following_user_id": sourceID, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	entry, err := domain.RecordManualActivity(domain.ManualActivity{ID: activityID, PathID: pathID, ParticipantID: sourceID, StartedAt: now.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Create(fromActivity(entry)).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Create(&activityRevisionModel{ActivityID: activityID, Version: 1, StartedAt: entry.StartedAt.Add(-time.Minute), EndedAt: entry.EndedAt, OccurrenceTimeZone: "Etc/UTC", PublicChanged: true, UpdatedAt: now, ReplacedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationDB.Table("user_models").Where("id IN ?", []string{ownerID, sourceID, followerID}).Delete(&userRow{})
	})

	repository := New(runtimeDB)
	assertVisible := func(viewer string) {
		t.Helper()
		if got, _, err := repository.GetActivity(context.Background(), viewer, pathID, activityID); err != nil || got.ID != activityID {
			t.Fatalf("viewer %s detail=%+v, %v", viewer, got, err)
		}
		page, err := repository.ListActivities(context.Background(), viewer, pathID, application.ActivityPageRequest{Limit: 10, Snapshot: now.Add(time.Second)})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("viewer %s list=%+v, %v", viewer, page, err)
		}
		revisions, err := repository.ListActivityRevisions(context.Background(), viewer, pathID, activityID, application.ActivityRevisionPageRequest{Limit: 10, Snapshot: now.Add(time.Second)})
		if err != nil || len(revisions.Items) != 1 {
			t.Fatalf("viewer %s revisions=%+v, %v", viewer, revisions, err)
		}
	}
	assertHidden := func(viewer string) {
		t.Helper()
		if _, _, err := repository.GetActivity(context.Background(), viewer, pathID, activityID); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("viewer %s detail error=%v, want opaque not found", viewer, err)
		}
		page, err := repository.ListActivities(context.Background(), viewer, pathID, application.ActivityPageRequest{Limit: 10, Snapshot: now.Add(time.Second)})
		if err != nil || len(page.Items) != 0 {
			t.Fatalf("viewer %s retained list=%+v, %v", viewer, page, err)
		}
		if _, err := repository.ListActivityRevisions(context.Background(), viewer, pathID, activityID, application.ActivityRevisionPageRequest{Limit: 10, Snapshot: now.Add(time.Second)}); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("viewer %s revisions error=%v, want opaque not found", viewer, err)
		}
	}
	for _, viewer := range []string{ownerID, followerID} {
		assertVisible(viewer)
	}
	if err := migrationDB.Table("block_models").Create(map[string]any{"blocker_user_id": followerID, "blocked_user_id": sourceID, "created_at": now.Add(time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	assertHidden(followerID)
	if err := migrationDB.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: followerID, Role: "supporter"}).Error; err != nil {
		t.Fatal(err)
	}
	assertVisible(followerID)
	if err := migrationDB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, followerID).Delete(&membershipRow{}).Error; err != nil {
		t.Fatal(err)
	}
	assertHidden(followerID)
	if err := migrationDB.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", followerID, sourceID).Delete(&struct{}{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("block_models").Create(map[string]any{"blocker_user_id": sourceID, "blocked_user_id": followerID, "created_at": now.Add(2 * time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	assertHidden(followerID)
	if err := migrationDB.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", sourceID, followerID).Delete(&struct{}{}).Error; err != nil {
		t.Fatal(err)
	}
	assertVisible(followerID)
	if err := migrationDB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, sourceID).Delete(&membershipRow{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []string{ownerID, followerID} {
		assertHidden(viewer)
	}
	if err := migrationDB.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: sourceID, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []string{ownerID, followerID} {
		assertVisible(viewer)
	}
}
