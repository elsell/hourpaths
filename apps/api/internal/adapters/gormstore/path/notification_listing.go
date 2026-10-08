package pathstore

import (
	"time"

	"gorm.io/gorm"
)

func visibleUnreadNotificationCount(
	tx *gorm.DB,
	recipientUserID string,
	snapshot time.Time,
	emojiReactions ...bool,
) (int64, error) {
	var count int64
	err := joinNotificationSubjects(tx.Table("notification_models")).
		Where(notificationRepresentationPredicate(len(emojiReactions) > 0 && emojiReactions[0], len(emojiReactions) > 1 && emojiReactions[1])).
		Where("notification_models.recipient_user_id = ? AND notification_models.created_at <= ? AND notification_models.read_at IS NULL AND "+visibleNotificationPredicate,
			recipientUserID, snapshot).
		Count(&count).Error
	return count, err
}

func notificationRepresentationPredicate(emojiReactions bool, timerStarts ...bool) string {
	reaction := "TRUE"
	if !emojiReactions {
		reaction = "(notification_models.kind <> 'practice_reaction' OR notification_models.reaction_type IN ('heart', 'applause', 'fire', 'strong', 'celebrate'))"
	}
	if len(timerStarts) == 0 || !timerStarts[0] {
		return "(" + reaction + ") AND notification_models.kind <> 'timer_started'"
	}
	return reaction
}
