package pathstore

import (
	"context"
	"errors"
	"strings"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func (r *Repository) GetNotification(
	ctx context.Context,
	recipientUserID, notificationID string,
) (application.InvitationNotificationProjection, error) {
	if r == nil || r.DB == nil ||
		strings.TrimSpace(recipientUserID) == "" ||
		strings.TrimSpace(notificationID) == "" {
		return application.InvitationNotificationProjection{}, ports.ErrInvalidArgument
	}
	var row invitationNotificationRow
	err := notificationProjectionQuery(r.DB.WithContext(ctx)).
		Where("notification_models.id = ? AND notification_models.recipient_user_id = ? AND notification_models.created_at <= CURRENT_TIMESTAMP", notificationID, recipientUserID).
		Where("("+visibleNotificationPredicate+") OR (notification_models.interaction_disabled_reason IS NOT NULL AND "+disabledNotificationEventAccessiblePredicate+")", recipientUserID, recipientUserID, recipientUserID, recipientUserID, recipientUserID, recipientUserID, recipientUserID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return application.InvitationNotificationProjection{}, ports.ErrNotFound
	}
	if err != nil {
		return application.InvitationNotificationProjection{}, err
	}
	item, err := invitationNotificationFromRow(row, recipientUserID)
	if err != nil {
		return application.InvitationNotificationProjection{}, errInvalidPersistedInvitation
	}
	return item, nil
}

const disabledNotificationEventAccessiblePredicate = `
notification_models.deleted_at IS NOT NULL
AND ` + notificationPairVisiblePredicate + `
AND notification_event.id IS NOT NULL
AND event_owner.id IS NOT NULL
AND EXISTS (SELECT 1 FROM user_models disabled_viewer WHERE disabled_viewer.id = ? AND disabled_viewer.status = 'active')
AND NOT EXISTS (SELECT 1 FROM block_models disabled_block
  WHERE (disabled_block.blocker_user_id = ? AND disabled_block.blocked_user_id = notification_event.participant_user_id)
     OR (disabled_block.blocker_user_id = notification_event.participant_user_id AND disabled_block.blocked_user_id = ?))
AND (
  notification_event.participant_user_id = ?
  OR EXISTS (SELECT 1 FROM follow_models disabled_follow
    WHERE disabled_follow.follower_user_id = ? AND disabled_follow.following_user_id = notification_event.participant_user_id)
  OR ((path_models.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models disabled_vm
      WHERE disabled_vm.path_id = path_models.id AND disabled_vm.user_id = ? AND disabled_vm.role IN ('administrator', 'participant')))
    AND (path_models.owner_user_id = notification_event.participant_user_id OR EXISTS (SELECT 1 FROM path_membership_models disabled_om
      WHERE disabled_om.path_id = path_models.id AND disabled_om.user_id = notification_event.participant_user_id AND disabled_om.role IN ('administrator', 'participant'))))
)`
