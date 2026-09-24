package pathstore

import (
	"time"

	"gorm.io/gorm"
)

func visibleUnreadNotificationCount(
	tx *gorm.DB,
	recipientUserID string,
	snapshot time.Time,
) (int64, error) {
	var count int64
	err := joinNotificationSubjects(tx.Table("notification_models")).
		Where("notification_models.recipient_user_id = ? AND notification_models.created_at <= ? AND notification_models.read_at IS NULL AND "+visibleNotificationPredicate,
			recipientUserID, snapshot).
		Count(&count).Error
	return count, err
}
