package gormstore

import (
	"bytes"
	"context"
	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresTimerAdmissionNotifiesOnlyNewLiveStarts(t *testing.T) {
	for _, mode := range []string{"online", "offline-running", "offline-completed", "offline-old-client"} {
		t.Run(mode, func(t *testing.T) {
			f := newNudgeFixture(t, "admission", false)
			if err := f.migration.DB.Table("path_membership_models").Create(map[string]any{"path_id": f.pathID, "user_id": f.sender.ID, "role": "participant", "joined_at": f.now.Add(-time.Hour)}).Error; err != nil {
				t.Fatal(err)
			}
			repo := activitystore.New(f.runtime.DB)
			timer, err := domain.StartTimer("timer-"+newTestID(), f.pathID, f.sender.ID, f.now, "Etc/UTC", f.now)
			if err != nil {
				t.Fatal(err)
			}
			event := audit.Event{ID: newTestID(), OwnerUserID: f.sender.ID, ActorUserID: f.sender.ID, Action: audit.ActivityTimerStarted, TargetType: "timer", TargetID: timer.ID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: f.now}
			replay := ports.Idempotency{PrincipalID: f.sender.ID, Operation: activityapp.StartTimerOperation, Key: newTestID(), RequestHash: bytes.Repeat([]byte{1}, 32)}
			recipients := []string{f.recipient.ID}
			if mode == "online" {
				command := activityapp.StartTimerCommand{Timer: timer, NotificationRecipients: recipients, Idempotency: replay, Audit: event}
				for i := 0; i < 2; i++ {
					if _, err := repo.StartTimer(context.Background(), command); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				replay.Operation = activityapp.OfflineTimerOperation
				command := activityapp.OfflineTimerCommand{Timer: timer, Kind: "start", NotifyStart: mode == "offline-running", NotificationRecipients: recipients, IdentityHash: activityapp.OfflineTimerIdentityHash(timer), RecordedAt: f.now, Idempotency: replay, Audit: event}
				if _, err := repo.SynchronizeTimer(context.Background(), command); err != nil {
					t.Fatal(err)
				}
				// Transport context is not mutation identity and cannot recreate delivery.
				command.NotifyStart = !command.NotifyStart
				if _, err := repo.SynchronizeTimer(context.Background(), command); err != nil {
					t.Fatal(err)
				}
				if mode == "offline-completed" {
					ended := f.now.Add(time.Minute)
					command.Kind = "stop"
					command.EndedAt = &ended
					command.RecordedAt = ended
					command.ActivityID = newTestID()
					command.NotifyStart = false
					command.Idempotency.Key = newTestID()
					command.Idempotency.RequestHash = bytes.Repeat([]byte{2}, 32)
					command.Audit.ID = newTestID()
					command.Audit.Action = audit.ActivityTimerStopped
					command.Audit.OccurredAt = ended
					if _, err := repo.SynchronizeTimer(context.Background(), command); err != nil {
						t.Fatal(err)
					}
				}
			}
			var count int64
			if err := f.migration.DB.Table("notification_models").Where("path_id = ? AND recipient_user_id = ? AND kind = ?", f.pathID, f.recipient.ID, "timer_started").Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			expected := int64(0)
			if mode == "online" || mode == "offline-running" {
				expected = 1
			}
			if count != expected {
				t.Fatalf("notifications=%d wanted=%d", count, expected)
			}
		})
	}
}
