package activitystore

import (
	"context"
	"fmt"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestPostgresLongTimerConcurrentWorkersCommitOneNotice(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("PostgreSQL runtime and migration DSNs required")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	ownerDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	owner, path, timerID := "long-owner-"+suffix, "long-path-"+suffix, "long-timer-"+suffix
	at := time.Now().UTC().Truncate(time.Microsecond)
	seedParticipantAndPath(t, ownerDB, owner, path, at.Add(-time.Hour))
	t.Cleanup(func() {
		ownerDB.Table("path_models").Where("id = ?", path).Delete(&struct{ ID string }{})
		ownerDB.Table("user_models").Where("id = ?", owner).Delete(&struct{ ID string }{})
	})
	for i := 0; i < 3; i++ {
		entry := domain.RecordedActivity{ID: fmt.Sprintf("long-history-%s-%d", suffix, i), PathID: path, ParticipantID: owner, StartedAt: at.Add(-time.Hour), EndedAt: at.Add(-time.Hour + time.Minute), OccurrenceTimeZone: "UTC", CreatedAt: at, UpdatedAt: at}
		if err := ownerDB.Create(fromActivity(entry)).Error; err != nil {
			t.Fatal(err)
		}
	}
	timer, _ := domain.StartTimer(timerID, path, owner, at.Add(-90*time.Second), "UTC", at)
	if err := ownerDB.Create(fromTimer(timer)).Error; err != nil {
		t.Fatal(err)
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
			event := timerAudit(fmt.Sprintf("long-audit-%s-%d", suffix, index), owner, timerID, audit.ResourceCreated, at)
			event.TargetType = "long_timer_notice"
			sent, err := New(runtimeDB).PublishLongTimerNotice(context.Background(), application.LongTimerNoticeCommand{Candidate: application.LongTimerCandidate{TimerID: timerID, PathID: path, ParticipantID: owner}, At: at, Audit: event})
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
	for _, target := range []struct{ table, column, value string }{{"notification_models", "timer_id", timerID}, {"long_timer_notice_receipt_models", "timer_id", timerID}, {"notification_push_outbox_models", "notification_id", longTimerNotificationID(timerID)}, {"audit_event_models", "target_id", timerID}} {
		var count int64
		if err := runtimeDB.Table(target.table).Where(target.column+" = ?", target.value).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s rows=%d err=%v", target.table, count, err)
		}
	}
}
