package activitystore

import (
	"context"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

// StopPathTimersForArchive stops every server-known timer for one Path inside
// the caller's transaction. The shared instant makes the lifecycle boundary
// exact across participants; canonical subsecond timers stop without creating
// invalid zero-duration activity.
func (r *Repository) StopPathTimersForArchive(ctx context.Context, pathID string, stoppedAt time.Time, newActivityID func() string) error {
	if r == nil || r.DB == nil || strings.TrimSpace(pathID) != pathID || pathID == "" || stoppedAt.IsZero() || newActivityID == nil {
		return ports.ErrInvalidArgument
	}
	var rows []timerModel
	if err := r.DB.WithContext(ctx).
		Where("path_id = ?", pathID).
		// The caller owns the exclusive parent Path row. Every timer mutation
		// takes a parent share lock first, making that least-privilege lock the
		// serialization boundary without requiring timer-row UPDATE privilege.
		Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		entry, saved, err := toTimer(row).Stop(newActivityID(), stoppedAt, stoppedAt)
		if err != nil {
			return ports.ErrInvalidArgument
		}
		if saved {
			if err := r.DB.WithContext(ctx).Create(fromActivity(entry)).Error; err != nil {
				return err
			}
		}
		deleted := r.DB.WithContext(ctx).
			Where("id = ? AND path_id = ? AND participant_id = ?", row.ID, row.PathID, row.ParticipantID).
			Delete(&timerModel{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return ports.ErrConflict
		}
	}
	return nil
}
