package pathstore

import (
	"context"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"gorm.io/gorm"
)

func assertHiddenInvitationExcludedFromMutationCount(
	t *testing.T,
	ctx context.Context,
	repository *Repository,
	db *gorm.DB,
	recipientID string,
	actorID string,
	pathID string,
	invitationID string,
	now time.Time,
) {
	t.Helper()
	visible := map[string]any{
		"id": "path-invite-notification-recipient-visible", "recipient_user_id": recipientID,
		"actor_user_id": actorID, "path_id": pathID, "path_invitation_id": invitationID,
		"path_ownership_transfer_id": nil, "kind": "path_invitation_accepted",
		"presentation_class": "informational", "channel": invitationNotificationChannel,
		"offered_role": string(pathdomain.RoleParticipant), "created_at": now.Add(time.Minute),
	}
	if err := db.Table("notification_models").Create(visible).Error; err != nil {
		t.Fatal(err)
	}
	result, err := repository.MarkNotificationRead(ctx, notificationMutationCommand(
		recipientID, "path-invite-notification-recipient-visible", now.Add(2*time.Minute),
		"path-invite-hidden-count-read-audit", audit.ResourceUpdated,
	))
	if err != nil || result.UnreadCount != 0 {
		t.Fatalf("visible mutation count with hidden invitation = %+v, %v; want 0", result, err)
	}
}
