package pathstore

import (
	"time"

	"gorm.io/gorm"
)

const (
	pathAccessRevokedPushFailureCode = "path_access_revoked"
	nudgeReceivedNotificationKind    = "nudge_received"
)

func targetOpeningPathNotificationKind(kind string) bool {
	return kind == nudgeReceivedNotificationKind
}

func retainsPathAccessAfterMemberRemoval(visibility string, followsOwner, blockedByOwner bool) bool {
	return !blockedByOwner && (visibility == "public" || visibility == "followers" && followsOwner)
}

func retireInaccessiblePathTargetNotifications(tx *gorm.DB, pathID, recipientUserID string, removedAt time.Time) error {
	var path struct{ Visibility, OwnerUserID string }
	if err := tx.Table("path_models").Select("visibility, owner_user_id").Where("id = ?", pathID).Take(&path).Error; err != nil {
		return err
	}
	var followsOwner int64
	if path.Visibility == "followers" {
		if err := tx.Table("follow_models").
			Where("follower_user_id = ? AND following_user_id = ?", recipientUserID, path.OwnerUserID).
			Count(&followsOwner).Error; err != nil {
			return err
		}
	}
	var blockedByOwner int64
	if path.Visibility != "private" {
		if err := tx.Table("block_models").Where(
			"(blocker_user_id = ? AND blocked_user_id = ?) OR (blocker_user_id = ? AND blocked_user_id = ?)",
			recipientUserID, path.OwnerUserID, path.OwnerUserID, recipientUserID,
		).Count(&blockedByOwner).Error; err != nil {
			return err
		}
	}
	if retainsPathAccessAfterMemberRemoval(path.Visibility, followsOwner > 0, blockedByOwner > 0) {
		return nil
	}

	var notificationIDs []string
	if err := tx.Model(&notificationModel{}).
		Where("path_id = ? AND recipient_user_id = ? AND kind = ? AND deleted_at IS NULL AND created_at <= ?", pathID, recipientUserID, nudgeReceivedNotificationKind, removedAt).
		Pluck("id", &notificationIDs).Error; err != nil || len(notificationIDs) == 0 {
		return err
	}
	if err := tx.Model(&notificationModel{}).
		Where("id IN ? AND deleted_at IS NULL", notificationIDs).
		Update("deleted_at", removedAt).Error; err != nil {
		return err
	}

	terminal := "delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL"
	if err := tx.Model(&invitationPushDeliveryModel{}).
		Where("notification_id IN ? AND "+terminal, notificationIDs).
		Updates(map[string]any{
			"suppressed_at": removedAt, "failure_code": pathAccessRevokedPushFailureCode,
			"token_ciphertext": nil, "token_nonce": nil, "token_hash": nil,
			"locked_by": nil, "locked_until": nil,
		}).Error; err != nil {
		return err
	}
	return tx.Table("notification_push_outbox_models").
		Where("notification_id IN ? AND "+terminal, notificationIDs).
		Updates(map[string]any{
			"suppressed_at": removedAt, "failure_code": pathAccessRevokedPushFailureCode,
			"locked_by": nil, "locked_until": nil,
		}).Error
}
