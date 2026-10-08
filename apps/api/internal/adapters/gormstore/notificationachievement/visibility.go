package notificationachievement

import "gorm.io/gorm"

const VisiblePredicate = `(notification_models.kind NOT IN ('interval_goal_achieved','overall_target_achieved') OR (
  notification_models.recipient_user_id = notification_models.actor_user_id
  AND notification_actor.status = 'active'
  AND notification_event.participant_user_id = notification_models.recipient_user_id
  AND notification_event.path_id = notification_models.path_id
  AND EXISTS (SELECT 1 FROM social_goal_achievement_models achievement
    WHERE achievement.id = notification_event.achievement_id
      AND achievement.participant_user_id = notification_models.recipient_user_id
      AND achievement.path_id = notification_models.path_id
      AND ((achievement.kind = 'interval' AND notification_models.kind = 'interval_goal_achieved')
        OR (achievement.kind = 'overall' AND notification_models.kind = 'overall_target_achieved')))
  AND (path_models.owner_user_id = notification_models.recipient_user_id OR EXISTS (
    SELECT 1 FROM path_membership_models participant WHERE participant.path_id = path_models.id
      AND participant.user_id = notification_models.recipient_user_id AND participant.role IN ('participant','administrator')))
  AND NOT EXISTS (SELECT 1 FROM block_models owner_block
    WHERE (owner_block.blocker_user_id = notification_models.recipient_user_id AND owner_block.blocked_user_id = path_models.owner_user_id)
      OR (owner_block.blocker_user_id = path_models.owner_user_id AND owner_block.blocked_user_id = notification_models.recipient_user_id))
))`

// Eligible rechecks the existing achievement under the final handoff locks.
func Eligible(tx *gorm.DB, notificationID, recipient string) (bool, error) {
	var count int64
	err := tx.Table("notification_models").
		Joins("JOIN path_models ON path_models.id = notification_models.path_id").
		Joins("JOIN user_models notification_actor ON notification_actor.id = notification_models.actor_user_id").
		Joins("JOIN social_feed_event_models notification_event ON notification_event.id = notification_models.social_feed_event_id").
		Where("notification_models.id = ? AND notification_models.recipient_user_id = ? AND notification_models.deleted_at IS NULL AND notification_models.kind IN ('interval_goal_achieved','overall_target_achieved')", notificationID, recipient).
		Where(VisiblePredicate).Count(&count).Error
	return count == 1, err
}
