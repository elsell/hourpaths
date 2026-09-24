package activitystore

import (
	"context"
	"errors"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

type intervalGoalRow struct {
	TargetSeconds *int64  `gorm:"column:interval_goal_target_seconds"`
	Recurrence    *string `gorm:"column:interval_goal_recurrence"`
	StartMinute   *int16  `gorm:"column:interval_goal_start_minute"`
	StartHour     *int16  `gorm:"column:interval_goal_start_hour"`
	StartWeekday  *int16  `gorm:"column:interval_goal_start_weekday"`
	StartDay      *int16  `gorm:"column:interval_goal_start_day"`
	StartMonth    *int16  `gorm:"column:interval_goal_start_month"`
}

func (r *Repository) IntervalGoal(ctx context.Context, participantID, pathID string) (pathdomain.IntervalGoal, error) {
	if r == nil || r.DB == nil || participantID == "" || strings.TrimSpace(participantID) != participantID || pathID == "" || strings.TrimSpace(pathID) != pathID {
		return pathdomain.IntervalGoal{}, ports.ErrInvalidArgument
	}
	var row intervalGoalRow
	err := r.DB.WithContext(ctx).
		Table("path_models").
		Select(`path_models.interval_goal_target_seconds, path_models.interval_goal_recurrence,
			path_models.interval_goal_start_minute, path_models.interval_goal_start_hour,
			path_models.interval_goal_start_weekday, path_models.interval_goal_start_day,
			path_models.interval_goal_start_month`).
		Joins("LEFT JOIN path_membership_models ON path_membership_models.path_id = path_models.id AND path_membership_models.user_id = ?", participantID).
		Where("path_models.id = ? AND (path_models.owner_user_id = ? OR path_membership_models.user_id = ?)", pathID, participantID, participantID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pathdomain.IntervalGoal{}, ports.ErrNotFound
	}
	if err != nil {
		return pathdomain.IntervalGoal{}, err
	}
	return intervalGoalFromRow(row)
}

func intervalGoalFromRow(row intervalGoalRow) (pathdomain.IntervalGoal, error) {
	absent := row.TargetSeconds == nil && row.Recurrence == nil && row.StartMinute == nil &&
		row.StartHour == nil && row.StartWeekday == nil && row.StartDay == nil && row.StartMonth == nil
	if absent {
		return pathdomain.IntervalGoal{}, nil
	}
	if row.TargetSeconds == nil || row.Recurrence == nil {
		return pathdomain.IntervalGoal{}, errors.New("persisted interval goal is invalid")
	}
	goal := pathdomain.IntervalGoal{
		Present: true, TargetSeconds: *row.TargetSeconds, Recurrence: pathdomain.Recurrence(*row.Recurrence),
	}
	switch goal.Recurrence {
	case pathdomain.RecurrenceHourly:
		if row.StartMinute == nil || row.StartHour != nil || row.StartWeekday != nil || row.StartDay != nil || row.StartMonth != nil {
			return pathdomain.IntervalGoal{}, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.Minute = int(*row.StartMinute)
	case pathdomain.RecurrenceDaily:
		if row.StartMinute != nil || row.StartHour == nil || row.StartWeekday != nil || row.StartDay != nil || row.StartMonth != nil {
			return pathdomain.IntervalGoal{}, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.Hour = int(*row.StartHour)
	case pathdomain.RecurrenceWeekly:
		if row.StartMinute != nil || row.StartHour != nil || row.StartWeekday == nil || row.StartDay != nil || row.StartMonth != nil {
			return pathdomain.IntervalGoal{}, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.ISOWeekday = int(*row.StartWeekday)
	case pathdomain.RecurrenceMonthly:
		if row.StartMinute != nil || row.StartHour != nil || row.StartWeekday != nil || row.StartDay == nil || row.StartMonth != nil {
			return pathdomain.IntervalGoal{}, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.Day = int(*row.StartDay)
	case pathdomain.RecurrenceYearly:
		if row.StartMinute != nil || row.StartHour != nil || row.StartWeekday != nil || row.StartDay == nil || row.StartMonth == nil {
			return pathdomain.IntervalGoal{}, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.Month = int(*row.StartMonth)
		goal.Alignment.Day = int(*row.StartDay)
	default:
		return pathdomain.IntervalGoal{}, errors.New("persisted interval goal is invalid")
	}
	validated, err := pathdomain.New("interval-goal-validation", "interval-goal-validation", pathdomain.Attributes{
		Name: "Interval goal", Visibility: "private", IntervalGoal: goal,
	})
	if err != nil {
		return pathdomain.IntervalGoal{}, errors.New("persisted interval goal is invalid")
	}
	return validated.IntervalGoal, nil
}

func (r *Repository) CurrentProjection(ctx context.Context, participantID, pathID string, request *application.IntervalProgressRequest) (application.CurrentTimerResult, error) {
	if r == nil || r.DB == nil || participantID == "" || strings.TrimSpace(participantID) != participantID || pathID == "" || strings.TrimSpace(pathID) != pathID || !validIntervalProgressRequest(request) {
		return application.CurrentTimerResult{}, ports.ErrInvalidArgument
	}
	return currentProjection(r.DB.WithContext(ctx), participantID, pathID, request)
}

// IntervalSeconds projects canonical whole activity seconds into one half-open
// UTC interval. Cumulative floors from each activity's start preserve the
// canonical duration when a subsecond timer span crosses adjacent intervals.
func (r *Repository) IntervalSeconds(ctx context.Context, participantID, pathID string, window application.IntervalWindow) (int64, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(participantID) != participantID || participantID == "" ||
		strings.TrimSpace(pathID) != pathID || pathID == "" || window.StartedAt.IsZero() || window.EndedAt.IsZero() ||
		window.StartedAt.Location() != time.UTC || window.EndedAt.Location() != time.UTC || !window.EndedAt.After(window.StartedAt) {
		return 0, ports.ErrInvalidArgument
	}
	return intervalSeconds(r.DB.WithContext(ctx), participantID, pathID, window)
}

func intervalSeconds(db *gorm.DB, participantID, pathID string, window application.IntervalWindow) (int64, error) {
	var total int64
	err := db.Model(&activityModel{}).
		Select(`COALESCE(SUM(
			FLOOR(EXTRACT(EPOCH FROM (LEAST(ended_at, ?) - started_at))) -
			FLOOR(EXTRACT(EPOCH FROM (GREATEST(started_at, ?) - started_at)))
		), 0)::bigint`, window.EndedAt, window.StartedAt).
		Where("participant_id = ? AND path_id = ? AND started_at < ? AND ended_at > ?", participantID, pathID, window.EndedAt, window.StartedAt).
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if total < 0 {
		return 0, errors.New("persisted interval activity duration is invalid")
	}
	return total, nil
}

func accumulatedSeconds(db *gorm.DB, participantID, pathID string) (int64, error) {
	var total int64
	err := db.Model(&activityModel{}).
		Select("COALESCE(SUM(FLOOR(EXTRACT(EPOCH FROM (ended_at - started_at)))), 0)::bigint").
		Where("participant_id = ? AND path_id = ?", participantID, pathID).
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if total < 0 {
		return 0, errors.New("persisted activity duration is invalid")
	}
	return total, nil
}

func validIntervalProgressRequest(request *application.IntervalProgressRequest) bool {
	return request == nil || (request.TargetSeconds > 0 && !request.Window.StartedAt.IsZero() && !request.Window.EndedAt.IsZero() &&
		request.Window.StartedAt.Location() == time.UTC && request.Window.EndedAt.Location() == time.UTC && request.Window.EndedAt.After(request.Window.StartedAt))
}
