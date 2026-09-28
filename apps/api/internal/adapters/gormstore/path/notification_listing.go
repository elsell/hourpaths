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
		Where(notificationRepresentationPredicate(len(emojiReactions) > 0 && emojiReactions[0])).
		Where("notification_models.recipient_user_id = ? AND notification_models.created_at <= ? AND notification_models.read_at IS NULL AND "+visibleNotificationPredicate,
			recipientUserID, snapshot).
		Count(&count).Error
	return count, err
}

func notificationRepresentationPredicate(emojiReactions bool) string {
	if emojiReactions {
		return "TRUE"
	}
	return "(notification_models.kind <> 'practice_reaction' OR notification_models.reaction_type IN ('heart', 'applause', 'fire', 'strong', 'celebrate'))"
}
