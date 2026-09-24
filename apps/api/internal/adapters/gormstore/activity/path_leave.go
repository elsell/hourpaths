package activitystore

import (
	"context"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm/clause"
)

// StopParticipantTimerForLeave completes a server-backed timer while the
// participant still has membership, allowing ordinary activity derivatives to
// be created before the caller atomically ends membership and hides them.
func (r *Repository) StopParticipantTimerForLeave(ctx context.Context, pathID, participantID string, stoppedAt time.Time, activityID string, event audit.Event) (bool, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(pathID) != pathID || pathID == "" || strings.TrimSpace(participantID) != participantID || participantID == "" || stoppedAt.IsZero() || activityID == "" || !event.Valid() || event.Action != audit.ActivityTimerStopped || event.ActorUserID != participantID || event.OwnerUserID != participantID || event.Outcome != audit.Succeeded {
		return false, ports.ErrInvalidArgument
	}
	if err := progresslock.Lock(r.DB.WithContext(ctx), participantID, pathID); err != nil {
		return false, err
	}
	var timer timerModel
	deleted := r.DB.WithContext(ctx).Clauses(clause.Returning{}).Where("path_id = ? AND participant_id = ?", pathID, participantID).Delete(&timer)
	if deleted.Error != nil {
		return false, deleted.Error
	}
	if deleted.RowsAffected == 0 {
		return false, nil
	}
	event.TargetID = timer.ID
	if deleted.RowsAffected != 1 {
		return false, ports.ErrNotFound
	}
	entry, saved, err := toTimer(timer).Stop(activityID, postgresInstant(stoppedAt), postgresInstant(stoppedAt))
	if err != nil {
		return false, ports.ErrInvalidArgument
	}
	if saved {
		if err := persistCompletedActivity(r.DB.WithContext(ctx), entry); err != nil {
			return false, err
		}
	}
	if err := r.DB.WithContext(ctx).Create(fromAudit(event)).Error; err != nil {
		return false, err
	}
	return saved, nil
}
