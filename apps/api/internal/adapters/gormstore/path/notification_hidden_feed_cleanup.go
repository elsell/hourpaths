package pathstore

import (
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	sociallock "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
	"gorm.io/gorm"
)

var hiddenFeedTargetNotificationKinds = []string{"practice_reaction", "practice_comment", "comment_heart"}

func hiddenFeedTargetNotificationKind(kind string) bool {
	for _, allowed := range hiddenFeedTargetNotificationKinds {
		if kind == allowed {
			return true
		}
	}
	return false
}

func lockHiddenFeedEventOwner(tx *gorm.DB, owner string) error {
	return progresslock.LockKey(tx, sociallock.InteractionOwnerKey(owner))
}

func retireHiddenFeedTargetNotifications(tx *gorm.DB, pathID, participantUserID string, leftAt time.Time) error {
	var notificationIDs []string
	if err := tx.Table("notification_models AS notice").
		Joins("JOIN social_feed_event_models AS event ON event.id = notice.social_feed_event_id").
		Where("event.path_id = ? AND event.participant_user_id = ?", pathID, participantUserID).
		Where("notice.kind IN ?", hiddenFeedTargetNotificationKinds).
		Where("notice.deleted_at IS NULL OR notice.interaction_disabled_reason IS NOT NULL").
		Pluck("notice.id", &notificationIDs).Error; err != nil || len(notificationIDs) == 0 {
		return err
	}
	if err := tx.Table("notification_models").Where("id IN ?", notificationIDs).Updates(map[string]any{
		"deleted_at": gorm.Expr("GREATEST(created_at, ?)", leftAt), "interaction_disabled_reason": nil,
	}).Error; err != nil {
		return err
	}
	terminal := "delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL"
	if err := tx.Model(&invitationPushDeliveryModel{}).Where("notification_id IN ? AND "+terminal, notificationIDs).Updates(map[string]any{
		"suppressed_at": gorm.Expr("GREATEST(created_at, ?)", leftAt), "failure_code": pathAccessRevokedPushFailureCode,
		"token_ciphertext": nil, "token_nonce": nil, "token_hash": nil, "locked_by": nil, "locked_until": nil,
	}).Error; err != nil {
		return err
	}
	return tx.Table("notification_push_outbox_models").Where("notification_id IN ? AND "+terminal, notificationIDs).Updates(map[string]any{
		"suppressed_at": gorm.Expr("GREATEST(created_at, ?)", leftAt), "failure_code": pathAccessRevokedPushFailureCode,
		"locked_by": nil, "locked_until": nil,
	}).Error
}
