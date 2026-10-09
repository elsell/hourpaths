package gormstore

import (
	"context"
	"errors"
	"fmt"
	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresLongTimerHistoryHonorsCapabilityOwnerAndMembership(t *testing.T) {
	f := newNudgeFixture(t, "longhistory", false)
	at := f.now.Add(2 * time.Minute)
	timerID := "long-history-timer-" + newTestID()
	for i := 0; i < 3; i++ {
		if err := f.migration.DB.Table("recorded_activity_models").Create(map[string]any{"id": fmt.Sprintf("long-history-%s-%d", timerID, i), "path_id": f.pathID, "participant_id": f.recipient.ID, "started_at": f.now.Add(-time.Minute), "ended_at": f.now, "occurrence_time_zone": "UTC", "created_at": f.now, "updated_at": f.now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := f.migration.DB.Table("running_timer_models").Create(map[string]any{"id": timerID, "path_id": f.pathID, "participant_id": f.recipient.ID, "started_at": f.now, "occurrence_time_zone": "UTC"}).Error; err != nil {
		t.Fatal(err)
	}
	command := activityapp.LongTimerNoticeCommand{Candidate: activityapp.LongTimerCandidate{TimerID: timerID, PathID: f.pathID, ParticipantID: f.recipient.ID}, At: at, Audit: audit.Event{ID: newTestID(), OwnerUserID: f.recipient.ID, ActorUserID: f.recipient.ID, Action: audit.ResourceCreated, TargetType: "long_timer_notice", TargetID: timerID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}}
	if sent, err := activitystore.New(f.runtime.DB).PublishLongTimerNotice(context.Background(), command); err != nil || !sent {
		t.Fatalf("publish=%v err=%v", sent, err)
	}
	r := pathstore.New(f.runtime.DB)
	page := pathapp.NotificationPageRequest{Limit: 25, Snapshot: at, EmojiReactions: true, TimerStarts: true, Achievements: true}
	legacy, err := r.ListNotifications(context.Background(), f.recipient.ID, page)
	if err != nil || len(legacy.Items) != 0 || legacy.UnreadCount != 0 {
		t.Fatalf("unsupported vocabulary leaked: %+v %v", legacy, err)
	}
	page.LongTimers = true
	current, err := r.ListNotifications(context.Background(), f.recipient.ID, page)
	if err != nil || len(current.Items) != 1 || current.UnreadCount != 1 {
		t.Fatalf("current history: %+v %v", current, err)
	}
	notice := current.Items[0]
	if notice.Kind != pathapp.NotificationLongTimerRunning || notice.Actor.UserID != f.recipient.ID || string(notice.PathID) != f.pathID {
		t.Fatalf("unsafe notice: %+v", notice)
	}
	if _, err := r.GetNotification(context.Background(), f.sender.ID, notice.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("another account read notice: %v", err)
	}
	if err := f.migration.DB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", f.pathID, f.recipient.ID).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	revoked, err := r.ListNotifications(context.Background(), f.recipient.ID, page)
	if err != nil || len(revoked.Items) != 0 || revoked.UnreadCount != 0 {
		t.Fatalf("revoked history: %+v %v", revoked, err)
	}
	if _, err := r.GetNotification(context.Background(), f.recipient.ID, notice.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("revoked notice visible: %v", err)
	}
}
