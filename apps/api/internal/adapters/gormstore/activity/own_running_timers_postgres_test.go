package activitystore

import (
	"context"
	"fmt"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresOwnRunningTimersScopeAndCursor(t *testing.T) {
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
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", now.UnixNano())
	owner, peer := "timer-owner-"+suffix, "timer-peer-"+suffix
	first, second, foreign := "timer-first-"+suffix, "timer-second-"+suffix, "timer-foreign-"+suffix
	seedParticipantAndPath(t, admin, owner, first, now)
	seedParticipantAndPath(t, admin, peer, foreign, now)
	seedPath(t, admin, owner, second, now)
	timers := []timerModel{
		{ID: "a-" + suffix, PathID: first, ParticipantID: owner, StartedAt: now.Add(-time.Hour), OccurrenceTimeZone: "Etc/UTC"},
		{ID: "b-" + suffix, PathID: second, ParticipantID: owner, StartedAt: now.Add(-time.Hour), OccurrenceTimeZone: "Etc/UTC"},
		{ID: "c-" + suffix, PathID: foreign, ParticipantID: peer, StartedAt: now.Add(-2 * time.Hour), OccurrenceTimeZone: "Etc/UTC"},
	}
	if err := admin.Create(&timers).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Table("path_models").Where("id IN ?", []string{first, second, foreign}).Delete(&struct{}{})
		admin.Table("user_models").Where("id IN ?", []string{owner, peer}).Delete(&struct{}{})
	})
	repo := New(runtimeDB)
	page := application.RunningTimerPageRequest{Snapshot: now, Limit: 1}
	rows, err := repo.ListRunningTimerCandidates(context.Background(), owner, page)
	if err != nil || len(rows) != 1 || rows[0].Timer.ID != timers[0].ID || rows[0].PathName == "" {
		t.Fatalf("first page=%+v err=%v", rows, err)
	}
	page.AfterID = rows[0].Timer.ID
	page.AfterStartedAt = rows[0].Timer.StartedAt
	rows, err = repo.ListRunningTimerCandidates(context.Background(), owner, page)
	if err != nil || len(rows) != 1 || rows[0].Timer.ID != timers[1].ID {
		t.Fatalf("second page=%+v err=%v", rows, err)
	}
	rows, err = repo.ListRunningTimerCandidates(context.Background(), peer, application.RunningTimerPageRequest{Snapshot: now, Limit: 10})
	if err != nil || len(rows) != 1 || rows[0].Timer.ID != timers[2].ID {
		t.Fatalf("peer page=%+v err=%v", rows, err)
	}
	if err := admin.Table("user_models").Where("id = ?", owner).Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	rows, err = repo.ListRunningTimerCandidates(context.Background(), owner, application.RunningTimerPageRequest{Snapshot: now, Limit: 10})
	if err != nil || len(rows) != 0 {
		t.Fatalf("inactive account=%+v err=%v", rows, err)
	}
}
