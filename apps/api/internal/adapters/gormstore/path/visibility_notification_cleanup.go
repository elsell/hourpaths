package pathstore

import (
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	sociallock "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
	"gorm.io/gorm"
)

var visibilityTargetOpeningNotificationKinds = []string{
	"nudge_received", "practice_reaction", "practice_comment", "comment_heart",
}

func visibilityTargetOpeningNotificationKind(kind string) bool {
	for _, allowed := range visibilityTargetOpeningNotificationKinds {
		if kind == allowed {
			return true
		}
	}
	return false
}

func retainsPathAccessAfterVisibility(visibility string, member, followsOwner, blockedByOwner bool) bool {
	return member || !blockedByOwner && (visibility == "public" || visibility == "followers" && followsOwner)
}

func lockVisibilityNotificationWriters(tx *gorm.DB, pathID string) error {
	return progresslock.LockKey(tx, sociallock.PathAudienceKey(pathID))
}

func retireNotificationsOutsideVisibility(tx *gorm.DB, pathID, ownerUserID, visibility string, changedAt time.Time) error {
	var notificationIDs []string
	if err := tx.Table("notification_models AS notice").
		Where("notice.path_id = ? AND notice.kind IN ?", pathID, visibilityTargetOpeningNotificationKinds).
		Where("notice.recipient_user_id <> ?", ownerUserID).
		Where("notice.deleted_at IS NULL OR notice.interaction_disabled_reason IS NOT NULL").
		Where(`NOT EXISTS (SELECT 1 FROM path_membership_models membership
  WHERE membership.path_id = notice.path_id AND membership.user_id = notice.recipient_user_id)`).
		Where(`EXISTS (SELECT 1 FROM block_models access_block
  WHERE (access_block.blocker_user_id = notice.recipient_user_id AND access_block.blocked_user_id = ?)
     OR (access_block.blocker_user_id = ? AND access_block.blocked_user_id = notice.recipient_user_id))
  OR ? = 'private'
  OR (? = 'followers' AND NOT EXISTS (SELECT 1 FROM follow_models access_follow
    WHERE access_follow.follower_user_id = notice.recipient_user_id AND access_follow.following_user_id = ?))`, ownerUserID, ownerUserID, visibility, visibility, ownerUserID).
		Pluck("notice.id", &notificationIDs).Error; err != nil || len(notificationIDs) == 0 {
		return err
	}
	if err := tx.Table("notification_models").Where("id IN ?", notificationIDs).Updates(map[string]any{
		"deleted_at": gorm.Expr("GREATEST(created_at, ?)", changedAt), "interaction_disabled_reason": nil,
	}).Error; err != nil {
		return err
	}
	terminal := "delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL"
	if err := tx.Model(&invitationPushDeliveryModel{}).Where("notification_id IN ? AND "+terminal, notificationIDs).Updates(map[string]any{
		"suppressed_at": gorm.Expr("GREATEST(created_at, ?)", changedAt), "failure_code": pathAccessRevokedPushFailureCode,
		"token_ciphertext": nil, "token_nonce": nil, "token_hash": nil, "locked_by": nil, "locked_until": nil,
	}).Error; err != nil {
		return err
	}
	return tx.Table("notification_push_outbox_models").Where("notification_id IN ? AND "+terminal, notificationIDs).Updates(map[string]any{
		"suppressed_at": gorm.Expr("GREATEST(created_at, ?)", changedAt), "failure_code": pathAccessRevokedPushFailureCode,
		"locked_by": nil, "locked_until": nil,
	}).Error
}
