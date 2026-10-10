package activitystore

import (
	"context"
	"fmt"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestPostgresOwnHistoryRetentionScopeAndSnapshot(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("PostgreSQL integration DSNs required")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	admin, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer closePostgresFixture(t, runtimeDB)
	defer closePostgresFixture(t, admin)
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", now.UnixNano())
	owner, peer := "history-owner-"+suffix, "history-peer-"+suffix
	active, archived, foreign := "history-active-"+suffix, "history-archived-"+suffix, "history-foreign-"+suffix
	seedParticipantAndPath(t, admin, owner, active, now)
	seedParticipantAndPath(t, admin, peer, foreign, now)
	seedPath(t, admin, owner, archived, now)
	paths := []string{active, archived, foreign}
	t.Cleanup(func() {
		admin.Table("path_models").Where("id IN ?", paths).Delete(&struct{}{})
		admin.Table("user_models").Where("id IN ?", []string{owner, peer}).Delete(&struct{}{})
	})
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("history-empty-%d-%s", i, suffix)
		seedPath(t, admin, owner, id, now)
		paths = append(paths, id)
	}
	if err := admin.Table("path_models").Where("id = ?", archived).Update("archived_at", now).Error; err != nil {
		t.Fatal(err)
	}
	note := "private history"
	cutoff := now.Add(-90 * 24 * time.Hour)
	row := func(id, path, participant string, end time.Time) activityModel {
		return activityModel{ID: id + suffix, PathID: path, ParticipantID: participant, StartedAt: end.Add(-time.Minute), EndedAt: end, OccurrenceTimeZone: "Etc/UTC", Note: &note, CreatedAt: now.Add(-time.Second), UpdatedAt: now.Add(-time.Second)}
	}
	newest := row("newest-", active, owner, now.Add(-time.Hour))
	boundary := row("boundary-", archived, owner, cutoff)
	old := row("old-", active, owner, cutoff.Add(-time.Microsecond))
	other := row("foreign-", foreign, peer, now.Add(-time.Minute))
	if err := admin.Create(&[]activityModel{newest, boundary, old, other}).Error; err != nil {
		t.Fatal(err)
	}
	repo := New(runtimeDB)
	page := application.OwnHistoryPageRequest{ActivityPageRequest: application.ActivityPageRequest{ParticipantID: owner, Snapshot: now, Limit: 1}, EndedSince: cutoff}
	first, err := repo.ListOwnActivityCandidates(context.Background(), owner, page)
	if err != nil || len(first.Items) != 1 || first.Items[0].Activity.ID != newest.ID || !first.HasMore {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	// Move the unseen boundary record outside retention after taking the snapshot.
	revision := activityRevisionModel{ActivityID: boundary.ID, Version: 1, StartedAt: boundary.StartedAt, EndedAt: boundary.EndedAt, OccurrenceTimeZone: boundary.OccurrenceTimeZone, Note: boundary.Note, PublicChanged: true, UpdatedAt: boundary.UpdatedAt, ReplacedAt: now.Add(time.Second)}
	if err := admin.Create(&revision).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Model(&activityModel{}).Where("id = ?", boundary.ID).Updates(map[string]any{"started_at": cutoff.Add(-2 * time.Hour), "ended_at": cutoff.Add(-time.Hour), "updated_at": now.Add(time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	page.AfterID, page.AfterStartedAt = newest.ID, newest.StartedAt
	second, err := repo.ListOwnActivityCandidates(context.Background(), owner, page)
	if err != nil || len(second.Items) != 1 || second.Items[0].Activity.ID != boundary.ID || !second.Items[0].Activity.EndedAt.Equal(cutoff) || second.Items[0].Activity.Note != note || second.Items[0].PathName == "" || second.HasMore {
		t.Fatalf("snapshot page=%+v err=%v", second, err)
	}
	page.AfterID = ""
	page.AfterStartedAt = time.Time{}
	page.EndedSince = time.Time{}
	page.Limit = 100
	all, err := repo.ListOwnActivityCandidates(context.Background(), owner, page)
	if err != nil || len(all.Items) != 3 || all.HasMore {
		t.Fatalf("all=%+v err=%v", all, err)
	}
	page.ParticipantID = peer
	if _, err := repo.ListOwnActivityCandidates(context.Background(), owner, page); err == nil {
		t.Fatal("accepted another participant")
	}
}
