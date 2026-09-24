package pathstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
)

func TestPostgresHiddenPairNotificationsCannotBeMutatedAndRestoreUnchanged(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 29, 13, 0, 0, 0, time.UTC)
	recipient, actor := "blocked-notification-recipient-"+uuid.NewString(), "blocked-notification-actor-"+uuid.NewString()
	cleanupInvitationFixture(t, migrationDB, []string{recipient, actor}, nil)
	t.Cleanup(func() { cleanupInvitationFixture(t, migrationDB, []string{recipient, actor}, nil) })
	seedInvitationUser(t, migrationDB, recipient, "Blocked.Notice.Recipient", identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, migrationDB, actor, "Blocked.Notice.Actor", identity.ProfileVisibilityPublic, now)
	notificationID := "blocked-notification-" + uuid.NewString()
	if err := migrationDB.Table("notification_models").Create(map[string]any{
		"id": notificationID, "recipient_user_id": recipient, "actor_user_id": actor,
		"follow_subject_user_id": actor, "kind": "new_follower", "presentation_class": "informational",
		"channel": "following", "created_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("block_models").Create(map[string]any{"blocker_user_id": recipient, "blocked_user_id": actor, "created_at": now.Add(time.Second)}).Error; err != nil {
		t.Fatal(err)
	}

	repository := New(runtimeDB)
	if _, err := repository.MarkNotificationRead(ctx, notificationMutationCommand(recipient, notificationID, now.Add(2*time.Second), "hidden-read", audit.ResourceUpdated)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("hidden read err=%v", err)
	}
	if _, err := repository.DeleteNotification(ctx, notificationMutationCommand(recipient, notificationID, now.Add(3*time.Second), "hidden-delete", audit.ResourceDeleted)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("hidden delete err=%v", err)
	}
	if _, err := repository.MarkAllNotificationsRead(ctx, notificationMutationCommand(recipient, "history", now.Add(4*time.Second), "hidden-read-all", audit.ResourceUpdated)); err != nil {
		t.Fatalf("hidden mark-all err=%v", err)
	}
	var retained notificationModel
	if err := migrationDB.Where("id = ?", notificationID).Take(&retained).Error; err != nil {
		t.Fatal(err)
	}
	if retained.ReadAt != nil || retained.DeletedAt != nil {
		t.Fatalf("hidden notification mutated: %+v", retained)
	}

	if err := migrationDB.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", recipient, actor).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.MarkNotificationRead(ctx, notificationMutationCommand(recipient, notificationID, now.Add(5*time.Second), "restored-read", audit.ResourceUpdated)); err != nil {
		t.Fatalf("restored read err=%v", err)
	}
	if err := migrationDB.Where("id = ?", notificationID).Take(&retained).Error; err != nil {
		t.Fatal(err)
	}
	if retained.ReadAt == nil || !retained.ReadAt.Equal(now.Add(5*time.Second)) || retained.DeletedAt != nil {
		t.Fatalf("restored notification=%+v", retained)
	}
}

func TestPostgresMarkAllNotificationsReadLeavesDelayedReactionUnreadUntilEligible(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 28, 12, 0, 0, 123_456_000, time.UTC)
	eligibleAt := now.Add(5 * time.Second)
	owner, actor := "reaction-read-owner", "reaction-read-actor"
	pathID, activityID := "reaction-read-path", "reaction-read-activity"
	eventID := "practice:" + activityID
	cleanupInvitationFixture(t, migrationDB, []string{owner, actor}, []string{pathID})
	t.Cleanup(func() {
		cleanupInvitationFixture(t, migrationDB, []string{owner, actor}, []string{pathID})
	})
	seedInvitationUser(t, migrationDB, owner, "Reaction.Read.Owner", identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, migrationDB, actor, "Reaction.Read.Actor", identity.ProfileVisibilityPublic, now)
	seedGoalUpdatePath(t, migrationDB, domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{Name: "Piano", Visibility: "public"},
		CreatedAt:  now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}, owner)
	type activityRow struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
	}
	if err := migrationDB.Table("recorded_activity_models").Create(&activityRow{
		ID: activityID, PathID: pathID, ParticipantID: owner, OccurrenceTimeZone: "Etc/UTC",
		StartedAt: now.Add(-time.Hour), EndedAt: now.Add(-30 * time.Minute), CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	const notificationID = "reaction-read-notification"
	if err := migrationDB.Table("notification_models").Create(map[string]any{
		"id": notificationID, "recipient_user_id": owner, "actor_user_id": actor, "path_id": pathID,
		"social_feed_event_id": eventID, "reaction_type": "heart", "kind": "practice_reaction",
		"presentation_class": "informational", "channel": "reactions", "created_at": eligibleAt,
	}).Error; err != nil {
		t.Fatal(err)
	}

	repository := New(runtimeDB)
	beforeEligibility, err := repository.MarkAllNotificationsRead(ctx, notificationMutationCommand(
		owner, "history", now, "reaction-read-before-eligibility", audit.ResourceUpdated,
	))
	if err != nil || beforeEligibility.UnreadCount != 0 {
		t.Fatalf("MarkAllNotificationsRead before eligibility = %+v, %v", beforeEligibility, err)
	}
	var persisted notificationModel
	if err := migrationDB.Where("id = ?", notificationID).Take(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.ReadAt != nil {
		t.Fatalf("future reaction notification read_at = %v before eligibility, want nil", persisted.ReadAt)
	}

	atEligibility, err := repository.MarkAllNotificationsRead(ctx, notificationMutationCommand(
		owner, "history", eligibleAt, "reaction-read-at-eligibility", audit.ResourceUpdated,
	))
	if err != nil || atEligibility.UnreadCount != 0 {
		t.Fatalf("MarkAllNotificationsRead at eligibility = %+v, %v", atEligibility, err)
	}
	if err := migrationDB.Where("id = ?", notificationID).Take(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.ReadAt == nil || !persisted.ReadAt.Equal(eligibleAt) {
		t.Fatalf("eligible reaction notification read_at = %v, want %v", persisted.ReadAt, eligibleAt)
	}

	var auditCount int64
	if err := migrationDB.Model(&auditModel{}).Where(
		"owner_user_id = ? AND target_type = ? AND target_id = ? AND action = ?",
		owner, "notification", "history", audit.ResourceUpdated,
	).Count(&auditCount).Error; err != nil || auditCount != 2 {
		t.Fatalf("mark-all audit count = %d, %v; want 2", auditCount, err)
	}
}
