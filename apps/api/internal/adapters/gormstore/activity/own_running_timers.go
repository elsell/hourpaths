package activitystore

import (
	"context"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
)

func (r *Repository) ListRunningTimerCandidates(ctx context.Context, owner string, page application.RunningTimerPageRequest) ([]application.RunningTimerCandidate, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(owner) == "" || page.Snapshot.IsZero() || page.Limit < 1 || page.Limit > 101 || (page.AfterID == "") != page.AfterStartedAt.IsZero() {
		return nil, ports.ErrInvalidArgument
	}
	var rows []struct {
		Timer    timerModel `gorm:"embedded"`
		PathName string
	}
	query := r.DB.WithContext(ctx).Table("running_timer_models AS timers").Select("timers.*, paths.name AS path_name").Joins("JOIN path_models AS paths ON paths.id = timers.path_id").Joins("JOIN user_models AS participant ON participant.id = timers.participant_id AND participant.status = ?", "active").Where("timers.participant_id = ? AND timers.started_at <= ?", owner, page.Snapshot)
	if page.AfterID != "" {
		query = query.Where("(timers.started_at, timers.id) > (?, ?)", page.AfterStartedAt, page.AfterID)
	}
	if err := query.Order("timers.started_at ASC, timers.id ASC").Limit(page.Limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]application.RunningTimerCandidate, 0, len(rows))
	for _, row := range rows {
		result = append(result, application.RunningTimerCandidate{Timer: toTimer(row.Timer), PathName: row.PathName})
	}
	return result, nil
}
