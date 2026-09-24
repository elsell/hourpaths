package gormstore

import (
	"context"
	"errors"
	"testing"
	"time"

	sociallock "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresCommentHeartQueuedBehindVisibilityContractionRevalidatesAndWritesNothing(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtimeStore, migrationStore, fixture := seedSocialInteractionRaceFixture(t)
	blocker := migrationStore.DB.Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	t.Cleanup(func() { _ = blocker.Rollback().Error })
	if err := lockSocialPathAudience(blocker, fixture.pathID); err != nil {
		t.Fatal(err)
	}
	command := socialCommentHeartTestCommand(fixture.actor.ID, fixture.commentTarget(), fixture.ownerComment, true, "visibility-heart-race", fixture.now.Add(time.Second))
	completed := make(chan error, 1)
	go func() {
		_, err := NewSocialFeedRepository(runtimeStore.DB).SetPracticeCommentHeart(context.Background(), command)
		completed <- err
	}()
	waitForPathAudienceWaiter(t, migrationStore, fixture.pathID)
	if err := blocker.Table("path_models").Where("id = ? AND visibility = 'followers'", fixture.pathID).Update("visibility", "private").Error; err != nil {
		t.Fatal(err)
	}
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-completed:
		if !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("comment heart after contraction error=%v, want opaque not found", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("comment heart remained blocked after visibility contraction committed")
	}
	var hearts, notifications, audits int64
	if err := migrationStore.DB.Table("social_practice_comment_heart_models").Where("comment_id = ? AND actor_user_id = ?", fixture.ownerComment.ID, fixture.actor.ID).Count(&hearts).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("notification_models").Where("kind = 'comment_heart' AND comment_id = ? AND actor_user_id = ?", fixture.ownerComment.ID, fixture.actor.ID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("audit_event_models").Where("id = ?", command.Audit.ID).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if hearts != 0 || notifications != 0 || audits != 0 {
		t.Fatalf("late mutation committed hearts=%d notifications=%d audits=%d", hearts, notifications, audits)
	}
}

func waitForPathAudienceWaiter(t *testing.T, store *Store, pathID string) {
	t.Helper()
	key := sociallock.PathAudienceKey(pathID)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var count int64
		if err := store.DB.Table("pg_locks").Where(`locktype = 'advisory' AND NOT granted
AND classid = ((hashtextextended(?, 0) >> 32) & 4294967295)::oid
AND objid = (hashtextextended(?, 0) & 4294967295)::oid
AND objsubid = 1`, key, key).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("comment-heart writer did not wait for the Path audience lock")
}
