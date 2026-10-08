package activitystore

import (
	channelstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationchannel"
	"github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// The achievement, feed event, notification, push work and activity audit all
// share the caller's transaction. Its participant is the only recipient.
func createAchievementNotification(tx *gorm.DB, achievement social.GoalAchievement) error {
	if tx == nil || !achievement.Valid() {
		return ports.ErrInvalidArgument
	}
	enabled, err := channelstore.Allowed(tx, achievement.ParticipantID, "achievements")
	if err != nil || !enabled {
		return err
	}
	kind := "interval_goal_achieved"
	if achievement.Kind == social.AchievementOverall {
		kind = "overall_target_achieved"
	}
	id := "achievement-notice:" + achievement.ID
	created := tx.Table("notification_models").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{
		"id": id, "recipient_user_id": achievement.ParticipantID, "actor_user_id": achievement.ParticipantID,
		"path_id": achievement.PathID, "social_feed_event_id": "achievement:" + achievement.ID,
		"kind": kind, "presentation_class": "informational", "channel": "achievements", "created_at": achievement.PublishedAt,
	})
	if created.Error != nil || created.RowsAffected == 0 {
		return created.Error
	}
	return channelstore.QueuePush(tx, id, achievement.ParticipantID, achievement.PublishedAt)
}
