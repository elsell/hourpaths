package gormstore

import (
	timerstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationtimer"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func lockGoalReminderPaths(tx *gorm.DB, notificationID, recipient string) error {

	var paths []struct{ PathID, OwnerUserID string }
	if err := tx.Table("goal_reminder_receipt_models receipt").Select("receipt.path_id, path.owner_user_id").
		Joins("JOIN path_models path ON path.id = receipt.path_id").
		Where("receipt.notification_id = ? AND receipt.participant_id = ?", notificationID, recipient).
		Order("receipt.path_id").Find(&paths).Error; err != nil {
		return err
	}
	for _, path := range paths {
		if err := timerstore.Lock(tx, path.PathID); err != nil {
			return err
		}
		if path.OwnerUserID != recipient {
			if err := lockSocialPair(tx, path.OwnerUserID, recipient); err != nil {
				return err
			}
		}
		if err := progresslock.Lock(tx, recipient, path.PathID); err != nil {
			return err
		}
	}
	return nil
}

func goalReminderPushEligible(tx *gorm.DB, notificationID string) (bool, error) {
	var rows []struct {
		StartedAt, EndedAt, Now           time.Time
		TargetSeconds                     int64
		Recurrence, TimeZone              string
		Minute, Hour, Weekday, Day, Month int
	}
	err := tx.Table("goal_reminder_receipt_models receipt").
		Joins("JOIN notification_models notice ON notice.id = receipt.notification_id AND notice.recipient_user_id = receipt.participant_id").
		Joins("JOIN user_models recipient ON recipient.id = receipt.participant_id AND recipient.status = 'active'").
		Joins("JOIN user_preference_models preference ON preference.user_id = recipient.id").
		Joins("JOIN path_models path ON path.id = receipt.path_id AND path.archived_at IS NULL").
		Where("notice.id = ? AND notice.deleted_at IS NULL AND notice.recipient_user_id = notice.actor_user_id", notificationID).
		Where("receipt.interval_ended_at > CURRENT_TIMESTAMP").
		Joins(`JOIN LATERAL (
          SELECT COALESCE(SUM(
            FLOOR(EXTRACT(EPOCH FROM (LEAST(activity.ended_at, receipt.interval_ended_at) - activity.started_at))) -
            FLOOR(EXTRACT(EPOCH FROM (GREATEST(activity.started_at, receipt.interval_started_at) - activity.started_at)))
          ), 0) AS seconds FROM recorded_activity_models activity
          WHERE activity.participant_id = receipt.participant_id AND activity.path_id = receipt.path_id
            AND activity.started_at < receipt.interval_ended_at AND activity.ended_at > receipt.interval_started_at
        ) progress ON true`).
		Where("path.interval_goal_target_seconds IS NOT NULL AND path.interval_goal_target_seconds > progress.seconds").
		Where("path.interval_goal_target_seconds - progress.seconds <= EXTRACT(EPOCH FROM (receipt.interval_ended_at - CURRENT_TIMESTAMP))").
		Where("EXISTS (SELECT 1 FROM path_membership_models member WHERE member.path_id = path.id AND member.user_id = recipient.id AND member.role IN ('participant','administrator'))").
		Where("NOT EXISTS (SELECT 1 FROM goal_reminder_preference_models preference WHERE preference.path_id = path.id AND preference.participant_id = recipient.id AND NOT preference.enabled)").
		Where("NOT EXISTS (SELECT 1 FROM running_timer_models timer WHERE timer.path_id = path.id AND timer.participant_id = recipient.id)").
		Where("NOT EXISTS (SELECT 1 FROM block_models b WHERE (b.blocker_user_id = recipient.id AND b.blocked_user_id = path.owner_user_id) OR (b.blocked_user_id = recipient.id AND b.blocker_user_id = path.owner_user_id))").Select(`receipt.interval_started_at AS started_at, receipt.interval_ended_at AS ended_at, CURRENT_TIMESTAMP AS now,
 path.interval_goal_target_seconds AS target_seconds, path.interval_goal_recurrence AS recurrence, preference.current_time_zone AS time_zone,
 COALESCE(path.interval_goal_start_minute,0) AS minute, COALESCE(path.interval_goal_start_hour,0) AS hour,
 COALESCE(path.interval_goal_start_weekday,0) AS weekday, COALESCE(path.interval_goal_start_day,0) AS day,
 COALESCE(path.interval_goal_start_month,0) AS month`).Scan(&rows).Error
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		goal := pathdomain.IntervalGoal{Present: true, TargetSeconds: row.TargetSeconds, Recurrence: pathdomain.Recurrence(row.Recurrence), Alignment: pathdomain.GoalAlignment{Minute: row.Minute, Hour: row.Hour, ISOWeekday: row.Weekday, Day: row.Day, Month: row.Month}}
		window, err := application.CurrentIntervalWindow(goal, row.TimeZone, row.Now)
		if err != nil {
			return false, err
		}
		if window.StartedAt.Equal(row.StartedAt) && window.EndedAt.Equal(row.EndedAt) {
			return true, nil
		}
	}
	return false, nil
}

func lockGoalReminderTimeZone(tx *gorm.DB, recipient string) error {
	// Acquire this row lock after timer, progress and channel locks. Taking it
	// first can invert the settings writer order. Hold it through provider handoff.
	var preference struct{ UserID string }
	if err := tx.Table("user_preference_models").Clauses(clause.Locking{Strength: "SHARE"}).Select("user_id").Where("user_id = ?", recipient).Take(&preference).Error; err != nil {
		return err
	}

	return nil
}
