package activitystore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	activitydomain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"gorm.io/gorm"
)

type goalAchievementModel struct {
	ID                string `gorm:"primaryKey"`
	ParticipantUserID string
	PathID            string
	Kind              string
	TargetSeconds     int64
	IntervalStartedAt *time.Time
	IntervalEndedAt   *time.Time
	PublishedAt       time.Time
}

func (goalAchievementModel) TableName() string { return "social_goal_achievement_models" }

type achievementGoalRow struct {
	IntervalGoal         intervalGoalRow `gorm:"embedded"`
	OverallTargetSeconds *int64          `gorm:"column:overall_target_seconds"`
}

func publishGoalAchievements(tx *gorm.DB, activity activitydomain.RecordedActivity) error {
	if err := lockGoalAchievementAggregate(tx, activity.ParticipantID, activity.PathID); err != nil {
		return err
	}
	goals, err := currentAchievementGoals(tx, activity.PathID)
	if err != nil {
		return err
	}
	overallAfter, err := accumulatedSeconds(tx, activity.ParticipantID, activity.PathID)
	if err != nil {
		return err
	}
	overallBefore := overallAfter - activity.DurationSeconds()
	if goals.OverallTargetSeconds != nil && overallBefore < *goals.OverallTargetSeconds && overallAfter >= *goals.OverallTargetSeconds {
		if err := createGoalAchievement(tx, activity, socialdomain.AchievementOverall, *goals.OverallTargetSeconds, application.IntervalWindow{}); err != nil {
			return err
		}
	}

	intervalGoal, err := intervalGoalFromRow(goals.IntervalGoal)
	if err != nil {
		return err
	}
	if !intervalGoal.Present {
		return nil
	}
	timeZone, err := currentParticipantTimeZone(tx, activity.ParticipantID)
	if err != nil {
		return err
	}
	windows, err := overlappingIntervalWindows(intervalGoal, timeZone, activity.StartedAt, activity.EndedAt)
	if err != nil {
		return err
	}
	for _, window := range windows {
		intervalAfter, err := intervalSeconds(tx, activity.ParticipantID, activity.PathID, window)
		if err != nil {
			return err
		}
		intervalBefore, err := intervalSecondsWithoutActivity(tx, activity.ParticipantID, activity.PathID, activity.ID, window)
		if err != nil {
			return err
		}
		if intervalBefore < intervalGoal.TargetSeconds && intervalAfter >= intervalGoal.TargetSeconds {
			if err := createGoalAchievement(tx, activity, socialdomain.AchievementInterval, intervalGoal.TargetSeconds, window); err != nil {
				return err
			}
		}
	}
	return nil
}

func currentParticipantTimeZone(tx *gorm.DB, participantID string) (string, error) {
	var timeZone string
	if err := tx.Table("user_preference_models AS preference").Select("preference.current_time_zone").
		Joins("JOIN user_models AS participant ON participant.id = preference.user_id AND participant.status = ?", identity.StatusActive).
		Where("preference.user_id = ?", participantID).Take(&timeZone).Error; err != nil {
		return "", err
	}
	if identity.ValidateIANATimeZone(identity.IANATimeZone(timeZone)) != nil {
		return "", errors.New("persisted participant time zone is invalid")
	}
	return timeZone, nil
}

func overlappingIntervalWindows(goal pathdomain.IntervalGoal, timeZone string, startedAt, endedAt time.Time) ([]application.IntervalWindow, error) {
	windows := make([]application.IntervalWindow, 0, 2)
	cursor := startedAt
	for cursor.Before(endedAt) {
		window, err := application.CurrentIntervalWindow(goal, timeZone, cursor)
		if err != nil {
			return nil, err
		}
		if !window.EndedAt.After(cursor) || (len(windows) > 0 && !window.StartedAt.Equal(windows[len(windows)-1].EndedAt)) {
			return nil, errors.New("interval occurrence sequence is invalid")
		}
		windows = append(windows, window)
		cursor = window.EndedAt
	}
	return windows, nil
}

func currentAchievementGoals(tx *gorm.DB, pathID string) (achievementGoalRow, error) {
	var row achievementGoalRow
	err := tx.Table("path_models").Select(`interval_goal_target_seconds, interval_goal_recurrence,
interval_goal_start_minute, interval_goal_start_hour, interval_goal_start_weekday,
interval_goal_start_day, interval_goal_start_month, overall_target_seconds`).Where("id = ?", pathID).Take(&row).Error
	return row, err
}

func intervalSecondsWithoutActivity(tx *gorm.DB, participantID, pathID, activityID string, window application.IntervalWindow) (int64, error) {
	var total int64
	err := tx.Model(&activityModel{}).
		Select(`COALESCE(SUM(
FLOOR(EXTRACT(EPOCH FROM (LEAST(ended_at, ?) - started_at))) -
FLOOR(EXTRACT(EPOCH FROM (GREATEST(started_at, ?) - started_at)))
), 0)::bigint`, window.EndedAt, window.StartedAt).
		Where("participant_id = ? AND path_id = ? AND id <> ? AND started_at < ? AND ended_at > ?", participantID, pathID, activityID, window.EndedAt, window.StartedAt).
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if total < 0 {
		return 0, errors.New("persisted interval activity duration is invalid")
	}
	return total, nil
}

func createGoalAchievement(tx *gorm.DB, activity activitydomain.RecordedActivity, kind socialdomain.AchievementKind, target int64, window application.IntervalWindow) error {
	id, err := newGoalAchievementID()
	if err != nil {
		return err
	}
	value, err := socialdomain.NewGoalAchievement(id, activity.ParticipantID, activity.PathID, kind, target, window.StartedAt, window.EndedAt, activity.CreatedAt)
	if err != nil {
		return err
	}
	row := goalAchievementModel{ID: value.ID, ParticipantUserID: value.ParticipantID, PathID: value.PathID, Kind: string(value.Kind), TargetSeconds: value.TargetSeconds, PublishedAt: postgresInstant(value.PublishedAt)}
	if kind == socialdomain.AchievementInterval {
		started, ended := postgresInstant(value.IntervalStartedAt), postgresInstant(value.IntervalEndedAt)
		row.IntervalStartedAt, row.IntervalEndedAt = &started, &ended
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	return tx.Table("social_feed_event_models").Create(map[string]any{
		"id": "achievement:" + row.ID, "achievement_id": row.ID,
		"participant_user_id": row.ParticipantUserID, "path_id": row.PathID,
		"published_at": row.PublishedAt,
	}).Error
}

func removeUnsupportedGoalAchievements(tx *gorm.DB, participantID, pathID string) ([]string, error) {
	if err := lockGoalAchievementAggregate(tx, participantID, pathID); err != nil {
		return nil, err
	}
	var rows []goalAchievementModel
	if err := tx.Where("participant_user_id = ? AND path_id = ?", participantID, pathID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	overall, err := accumulatedSeconds(tx, participantID, pathID)
	if err != nil {
		return nil, err
	}
	removed := make([]string, 0)
	for _, row := range rows {
		var supported bool
		switch socialdomain.AchievementKind(row.Kind) {
		case socialdomain.AchievementOverall:
			supported = overall >= row.TargetSeconds
		case socialdomain.AchievementInterval:
			if row.IntervalStartedAt == nil || row.IntervalEndedAt == nil {
				return nil, errors.New("persisted goal achievement is invalid")
			}
			seconds, err := intervalSeconds(tx, participantID, pathID, application.IntervalWindow{StartedAt: row.IntervalStartedAt.UTC(), EndedAt: row.IntervalEndedAt.UTC()})
			if err != nil {
				return nil, err
			}
			supported = seconds >= row.TargetSeconds
		default:
			return nil, errors.New("persisted goal achievement is invalid")
		}
		if !supported {
			if err := tx.Delete(&goalAchievementModel{}, "id = ? AND participant_user_id = ? AND path_id = ?", row.ID, participantID, pathID).Error; err != nil {
				return nil, err
			}
			removed = append(removed, "achievement:"+row.ID)
		}
	}
	return removed, nil
}

func lockGoalAchievementAggregate(tx *gorm.DB, participantID, pathID string) error {
	return progresslock.Lock(tx, participantID, pathID)
}

func newGoalAchievementID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
