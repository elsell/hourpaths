package activitystore

import (
	"context"
	"fmt"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"testing"
	"time"
)

func TestPostgresLongTimerNoticeIsAtomicScopedAndDurablyOnce(t *testing.T) {
	db := postgresDB(t, false)
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	owner, path := "long-timer-owner", "long-timer-path"
	seedParticipantAndPath(t, db, owner, path, at.Add(-time.Hour))
	for i, seconds := range []int{60, 120, 180} {
		entry := domain.RecordedActivity{ID: fmt.Sprintf("long-history-%d", i), PathID: path, ParticipantID: owner, StartedAt: at.Add(-time.Hour), EndedAt: at.Add(-time.Hour).Add(time.Duration(seconds) * time.Second), OccurrenceTimeZone: "UTC", CreatedAt: at, UpdatedAt: at}
		if err := db.Create(fromActivity(entry)).Error; err != nil {
			t.Fatal(err)
		}
	}
	timer, _ := domain.StartTimer("long-timer", path, owner, at, "UTC", at)
	if err := db.Create(fromTimer(timer)).Error; err != nil {
		t.Fatal(err)
	}
	r := New(db)
	command := application.LongTimerNoticeCommand{Candidate: application.LongTimerCandidate{TimerID: timer.ID, PathID: path, ParticipantID: owner}, At: at.Add(180 * time.Second)}
	command.Audit = timerAudit("long-notice-audit", owner, timer.ID, audit.ResourceCreated, command.At)
	command.Audit.TargetType = "long_timer_notice"
	candidates, err := r.ListLongTimerCandidates(context.Background(), command.At, "", 10)
	if err != nil || len(candidates) != 1 || candidates[0] != command.Candidate {
		t.Fatalf("due candidates=%+v err=%v", candidates, err)
	}
	earlyCandidates, err := r.ListLongTimerCandidates(context.Background(), command.At.Add(-time.Microsecond), "", 10)
	if err != nil || len(earlyCandidates) != 0 {
		t.Fatalf("premature candidates=%+v err=%v", earlyCandidates, err)
	}

	early := command
	early.At = command.At.Add(-time.Microsecond)
	early.Audit.OccurredAt = early.At
	if sent, err := r.PublishLongTimerNotice(context.Background(), early); err != nil || sent {
		t.Fatalf("early: %v %v", sent, err)
	}
	foreign := command
	foreign.Candidate.ParticipantID = "other-account"
	if sent, err := r.PublishLongTimerNotice(context.Background(), foreign); err == nil || sent {
		t.Fatalf("foreign command accepted: %v %v", sent, err)
	}

	// A disabled channel does not consume the timer's once-only receipt.
	preference := map[string]any{"user_id": owner, "channel": "timer_health", "enabled": false, "revision": 1, "created_at": at, "updated_at": at}
	if err := db.Table("notification_channel_preference_models").Create(preference).Error; err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishLongTimerNotice(context.Background(), command); err != nil || sent {
		t.Fatalf("disabled channel: %v %v", sent, err)
	}
	if err := db.Table("notification_channel_preference_models").Where("user_id = ? AND channel = 'timer_health'", owner).Update("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	// The threshold must use the edited current durations rather than a snapshot.
	if err := db.Model(&activityModel{}).Where("id = ?", "long-history-2").UpdateColumn("ended_at", at.Add(-time.Hour+360*time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishLongTimerNotice(context.Background(), command); err != nil || sent {
		t.Fatalf("stale average used: %v %v", sent, err)
	}
	if err := db.Model(&activityModel{}).Where("id = ?", "long-history-2").UpdateColumn("ended_at", at.Add(-time.Hour+180*time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	// Audit failure rolls back the receipt, notification and push outbox together.
	if err := db.Create(fromAudit(command.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishLongTimerNotice(context.Background(), command); err == nil || sent {
		t.Fatalf("unaudited notice accepted: %v %v", sent, err)
	}
	for _, table := range []string{"long_timer_notice_receipt_models", "notification_models", "notification_push_outbox_models"} {
		var count int64
		query := db.Table(table)
		if table == "notification_push_outbox_models" {
			query = query.Where("notification_id = ?", longTimerNotificationID(timer.ID))
		} else {
			query = query.Where("timer_id = ?", timer.ID)
		}
		if err := query.Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("rollback left %s rows=%d err=%v", table, count, err)
		}
	}
	command.Audit.ID = "long-notice-success-audit"
	for i := 0; i < 2; i++ {
		sent, err := r.PublishLongTimerNotice(context.Background(), command)
		if err != nil || sent != (i == 0) {
			t.Fatalf("pass %d: %v %v", i, sent, err)
		}
	}
	var notices int64
	if err := db.Table("notification_models").Where("timer_id = ? AND kind = 'long_timer_running' AND recipient_user_id = ?", timer.ID, owner).Count(&notices).Error; err != nil || notices != 1 {
		t.Fatalf("notice count %d: %v", notices, err)
	}
	// Even physical removal of the notice must not re-notify a still-running timer.
	if err := db.Table("notification_models").Where("timer_id = ? AND kind = 'long_timer_running'", timer.ID).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	if sent, err := r.PublishLongTimerNotice(context.Background(), command); err != nil || sent {
		t.Fatalf("deleted notice recreated: %v %v", sent, err)
	}
}
