package gormstore

import (
	"context"
	"fmt"
	"time"

	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (repository *SocialFeedRepository) activePathProgress(ctx context.Context, participant, pathID string, at time.Time) (*socialapp.ActivePathProgress, error) {
	type ProgressGoalFields nudgeStateRow
	var row struct {
		ProgressGoalFields   `gorm:"embedded"`
		OverallTargetSeconds *int64
	}
	db := repository.db.WithContext(ctx)
	err := db.Table("path_models AS path").Select(`path.overall_target_seconds AS overall_target_seconds,
 preferences.current_time_zone,
 path.interval_goal_target_seconds AS target_seconds, path.interval_goal_recurrence AS recurrence,
 path.interval_goal_start_minute AS start_minute, path.interval_goal_start_hour AS start_hour,
 path.interval_goal_start_weekday AS start_weekday, path.interval_goal_start_day AS start_day,
 path.interval_goal_start_month AS start_month`).Joins("LEFT JOIN user_preference_models preferences ON preferences.user_id = ?", participant).Where("path.id = ?", pathID).Take(&row).Error
	if err != nil {
		return nil, fmt.Errorf("active progress: %w", ports.ErrUnavailable)
	}
	progress := &socialapp.ActivePathProgress{AsOf: at, OverallTargetSeconds: row.OverallTargetSeconds}
	err = db.Table("recorded_activity_models").Select("COALESCE(SUM(FLOOR(EXTRACT(EPOCH FROM (ended_at - started_at)))),0)::bigint").Where("participant_id = ? AND path_id = ?", participant, pathID).Scan(&progress.AccumulatedSeconds).Error
	if err != nil {
		return nil, fmt.Errorf("active progress: %w", ports.ErrUnavailable)
	}
	goal, err := nudgeIntervalGoal(nudgeStateRow(row.ProgressGoalFields))
	if err != nil {
		return nil, fmt.Errorf("active progress: %w", ports.ErrUnavailable)
	}
	if goal == nil {
		return progress, nil
	}
	if row.CurrentTimeZone == nil {
		return nil, fmt.Errorf("active progress: %w", ports.ErrUnavailable)
	}
	window, err := activityapp.CurrentIntervalWindow(*goal, *row.CurrentTimeZone, at)
	if err != nil {
		return nil, fmt.Errorf("active progress: %w", ports.ErrUnavailable)
	}
	interval := &socialapp.ActivePathIntervalProgress{TargetSeconds: goal.TargetSeconds, Recurrence: string(goal.Recurrence), StartedAt: window.StartedAt, EndedAt: window.EndedAt}
	err = db.Table("recorded_activity_models").Select(`COALESCE(SUM(
 FLOOR(EXTRACT(EPOCH FROM (LEAST(ended_at, ?) - started_at))) -
 FLOOR(EXTRACT(EPOCH FROM (GREATEST(started_at, ?) - started_at)))
 ),0)::bigint`, window.EndedAt, window.StartedAt).Where("participant_id = ? AND path_id = ? AND started_at < ? AND ended_at > ?", participant, pathID, window.EndedAt, window.StartedAt).Scan(&interval.RecordedSeconds).Error
	if err != nil {
		return nil, fmt.Errorf("active progress: %w", ports.ErrUnavailable)
	}
	progress.Interval = interval
	return progress, nil
}
