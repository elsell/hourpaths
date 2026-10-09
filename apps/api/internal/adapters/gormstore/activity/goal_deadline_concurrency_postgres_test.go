package activitystore

import (
	"context"
	"fmt"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestPostgresGoalDeadlineConcurrentWorkersCommitOneNotice(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("PostgreSQL runtime and migration DSNs required")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	closePostgresFixture(t, runtimeDB)
	ownerDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	closePostgresFixture(t, ownerDB)
	suffix := fmt.Sprint(time.Now().UnixNano())
	owner, path := "deadline-owner-"+suffix, "deadline-path-"+suffix
	at := time.Date(2026, 10, 10, 23, 59, 0, 0, time.UTC)
	seedParticipantAndPath(t, ownerDB, owner, path, at.Add(-time.Hour))
	t.Cleanup(func() {
		ownerDB.Table("path_models").Where("id = ?", path).Delete(&struct{ ID string }{})
		ownerDB.Table("user_models").Where("id = ?", owner).Delete(&struct{ ID string }{})
	})
	if err := ownerDB.Table("user_preference_models").Create(map[string]any{"user_id": owner, "first_day_of_week": 1, "current_time_zone": "UTC", "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := ownerDB.Table("path_models").Where("id = ?", path).Updates(map[string]any{"interval_goal_target_seconds": 3600, "interval_goal_recurrence": "daily", "interval_goal_start_hour": 0}).Error; err != nil {
		t.Fatal(err)
	}
	candidate := application.GoalDeadlineCandidate{PathID: path, ParticipantID: owner}
	window, due, err := goalDeadlineDue(ownerDB, candidate, at)
	if err != nil || !due {
		t.Fatalf("not due: %v %v", due, err)
	}
	type result struct {
		sent bool
		err  error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func(index int) {
			<-start
			event := timerAudit(fmt.Sprintf("long-audit-%s-%d", suffix, index), owner, path, audit.ResourceCreated, at)
			event.TargetType = "goal_deadline_notice"
			sent, err := New(runtimeDB).PublishGoalDeadlineNotice(context.Background(), application.GoalDeadlineNoticeCommand{Candidate: candidate, At: at, Audit: event})
			results <- result{sent, err}
		}(i)
	}
	close(start)
	sent := 0
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			if r.err != nil {
				t.Fatal(r.err)
			}
			if r.sent {
				sent++
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent notice workers did not finish")
		}
	}
	if sent != 1 {
		t.Fatalf("committed deliveries=%d", sent)
	}
	for _, target := range []struct{ table, column, value string }{{"notification_models", "path_id", path}, {"goal_deadline_notice_receipt_models", "path_id", path}, {"notification_push_outbox_models", "notification_id", goalDeadlineNoticeID(candidate, window)}, {"audit_event_models", "target_id", path}} {
		var count int64
		if err := runtimeDB.Table(target.table).Where(target.column+" = ?", target.value).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s rows=%d err=%v", target.table, count, err)
		}
	}
}
