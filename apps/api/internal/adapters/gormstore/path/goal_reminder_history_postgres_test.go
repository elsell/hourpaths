package pathstore

import (
	"context"
	"errors"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestPostgresGoalReminderHistoryCapabilityOwnershipAndMembership(t *testing.T) {
	runtime, db := goalUpdateDatabases(t)
	ctx := context.Background()
	suffix := uuid.NewString()
	owner, pathID, id := "reminder-owner-"+suffix, "reminder-path-"+suffix, "reminder-notice-"+suffix
	at := time.Now().UTC().Truncate(time.Microsecond)
	seedInvitationUser(t, db, owner, "reminder."+suffix[:8], identity.ProfileVisibilityPrivate, at)
	seedGoalUpdatePath(t, db, domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Practice", Visibility: "private"}, CreatedAt: at.Add(-time.Hour), UpdatedAt: at}, owner)
	t.Cleanup(func() { cleanupInvitationFixture(t, db, []string{owner}, []string{pathID}) })
	if err := db.Table("notification_models").Create(map[string]any{"id": id, "recipient_user_id": owner, "actor_user_id": owner, "kind": "goal_practice_reminder", "presentation_class": "informational", "channel": "goal_reminders", "created_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("goal_reminder_receipt_models").Create(map[string]any{"participant_id": owner, "path_id": pathID, "notification_id": id, "interval_started_at": at.Add(-time.Hour), "interval_ended_at": at.Add(time.Hour), "scheduled_at": at, "created_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	history := New(runtime)
	for _, enabled := range []bool{false, true} {
		page, err := history.ListNotifications(ctx, owner, pathapp.NotificationPageRequest{Limit: 10, Snapshot: at, GoalReminders: enabled})
		want := 0
		if enabled {
			want = 1
		}
		if err != nil || len(page.Items) != want || page.UnreadCount != int64(want) {
			t.Fatalf("capability=%v: %+v %v", enabled, page, err)
		}
	}
	page, err := history.ListNotifications(ctx, "foreign", pathapp.NotificationPageRequest{Limit: 10, Snapshot: at, GoalReminders: true})
	if err != nil || len(page.Items) != 0 || page.UnreadCount != 0 {
		t.Fatalf("foreign: %+v %v", page, err)
	}
	command := notificationMutationCommand(owner, id, at.Add(time.Second), "reminder-read-"+suffix, audit.ResourceUpdated)
	if _, err := history.MarkNotificationRead(ctx, command); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("old client mutated notice: %v", err)
	}
	if err := db.Table("notification_channel_preference_models").Create(map[string]any{"user_id": owner, "channel": "goal_reminders", "enabled": false, "revision": 1, "created_at": at, "updated_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	page, err = history.ListNotifications(ctx, owner, pathapp.NotificationPageRequest{Limit: 10, Snapshot: at, GoalReminders: true})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("disabled channel erased history: %+v %v", page, err)
	}
	if len(page.Items) != 1 || page.Items[0].Reminder == nil || len(page.Items[0].Reminder.Paths) != 1 || string(page.Items[0].Reminder.Paths[0].ID) != pathID {
		t.Fatalf("missing bundle paths %+v", page)
	}
	command.GoalReminders = true
	if result, err := history.MarkNotificationRead(ctx, command); err != nil || result.UnreadCount != 0 {
		t.Fatalf("read: %+v %v", result, err)
	}

	secondPath := "second-reminder-path-" + suffix
	seedGoalUpdatePath(t, db, domain.Entity{ID: domain.ID(secondPath), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Still accessible", Visibility: "private"}, CreatedAt: at.Add(-time.Hour), UpdatedAt: at}, owner)
	t.Cleanup(func() { cleanupInvitationFixture(t, db, nil, []string{secondPath}) })
	if err := db.Table("goal_reminder_receipt_models").Create(map[string]any{"participant_id": owner, "path_id": secondPath, "notification_id": id, "interval_started_at": at.Add(-time.Hour), "interval_ended_at": at.Add(time.Hour), "scheduled_at": at, "created_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, owner).UpdateColumn("role", "supporter").Error; err != nil {
		t.Fatal(err)
	}
	page, err = history.ListNotifications(ctx, owner, pathapp.NotificationPageRequest{Limit: 10, Snapshot: at.Add(time.Second), GoalReminders: true})
	if err != nil || len(page.Items) != 1 || page.Items[0].Reminder == nil || len(page.Items[0].Reminder.Paths) != 1 || string(page.Items[0].Reminder.Paths[0].ID) != secondPath {
		t.Fatalf("partially revoked bundle leaked or disappeared: %+v %v", page, err)
	}
	if err := db.Table("path_membership_models").Where("path_id = ? AND user_id = ?", secondPath, owner).UpdateColumn("role", "supporter").Error; err != nil {
		t.Fatal(err)
	}
	page, err = history.ListNotifications(ctx, owner, pathapp.NotificationPageRequest{Limit: 10, Snapshot: at.Add(time.Second), GoalReminders: true})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("former participant history exposed: %+v %v", page, err)
	}
}
