package pathstore

import (
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationreminder"
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func loadGoalReminderPaths(tx *gorm.DB, recipient string, item *application.InvitationNotificationProjection) error {
	if item.Kind != application.NotificationGoalPracticeReminder {
		return nil
	}
	var paths []application.GoalReminderPath
	err := tx.Table("goal_reminder_receipt_models reminder_receipt").Select("reminder_path.id, reminder_path.name").Joins("JOIN path_models reminder_path ON reminder_path.id = reminder_receipt.path_id").Where("reminder_receipt.notification_id = ? AND reminder_receipt.participant_id = ?", item.ID, recipient).Where(notificationreminder.PathAccessPredicate).Order("reminder_receipt.scheduled_at, reminder_path.id").Find(&paths).Error
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return ports.ErrNotFound
	}
	item.Reminder = &application.GoalReminderBundle{Paths: paths}
	return nil
}
