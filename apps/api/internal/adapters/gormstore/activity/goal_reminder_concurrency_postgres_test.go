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

func TestPostgresGoalReminderConcurrentWorkersCommitOneBundle(t *testing.T) {
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
	owner, path := "reminder-owner-"+suffix, "reminder-path-"+suffix
	at := time.Date(2026, 10, 10, 22, 30, 0, 0, time.UTC)
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
	type result struct {
		sent int
		err  error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func(index int) {
			<-start
			event := timerAudit(fmt.Sprintf("long-audit-%s-%d", suffix, index), owner, owner, audit.ResourceCreated, at)
			event.TargetType = "goal_reminder_delivery"
			sent, err := New(runtimeDB).PublishGoalReminders(context.Background(), application.GoalReminderDeliveryCommand{ParticipantID: owner, AuthorizedPathIDs: []string{path}, At: at, Audit: event})
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
			sent += r.sent
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent notice workers did not finish")
		}
	}
	if sent != 1 {
		t.Fatalf("committed deliveries=%d", sent)
	}
	for _, target := range []struct {
		table, column string
		want          int64
	}{
		{"notification_models", "recipient_user_id", 1},
		{"goal_reminder_receipt_models", "participant_id", 1},
		{"audit_event_models", "owner_user_id", 1},
	} {
		var count int64
		if err := runtimeDB.Table(target.table).Where(target.column+" = ?", owner).Count(&count).Error; err != nil || count != target.want {
			t.Fatalf("%s count=%d err=%v", target.table, count, err)
		}
	}
}
