package activitystore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	channelstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationchannel"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) ListLongTimerCandidates(ctx context.Context, at time.Time, after string, limit int) ([]application.LongTimerCandidate, error) {
	if r == nil || r.DB == nil || limit < 1 || limit > 100 || len(after) > 256 || at.IsZero() {
		return nil, ports.ErrInvalidArgument
	}
	var result []application.LongTimerCandidate
	err := r.DB.WithContext(ctx).Table("running_timer_models timer").
		Select("timer.id AS timer_id, timer.path_id, timer.participant_id").
		Joins("JOIN user_models owner ON owner.id = timer.participant_id AND owner.status = 'active'").
		Joins("JOIN path_models path ON path.id = timer.path_id AND path.archived_at IS NULL").
		Where("timer.id > ?", after).
		Where("NOT EXISTS (SELECT 1 FROM notification_channel_preference_models preference WHERE preference.user_id = timer.participant_id AND preference.channel = 'timer_health' AND NOT preference.enabled)").
		Where(`EXISTS (SELECT 1 FROM recorded_activity_models entry
   WHERE entry.participant_id = timer.participant_id AND entry.path_id = timer.path_id
    AND entry.ended_at >= entry.started_at + interval '1 second'
   HAVING count(*) >= 3 AND extract(epoch FROM (?::timestamptz - timer.started_at)) * 2 * count(*) >= sum(floor(extract(epoch FROM (entry.ended_at - entry.started_at)))) * 3)`, at).
		Where("NOT EXISTS (SELECT 1 FROM long_timer_notice_receipt_models receipt WHERE receipt.timer_id = timer.id)").
		Order("timer.id").Limit(limit).Find(&result).Error
	return result, err
}

func (r *Repository) PublishLongTimerNotice(ctx context.Context, c application.LongTimerNoticeCommand) (bool, error) {
	candidate := c.Candidate
	if r == nil || r.DB == nil || strings.TrimSpace(candidate.TimerID) == "" || strings.TrimSpace(candidate.PathID) == "" || strings.TrimSpace(candidate.ParticipantID) == "" || c.At.IsZero() || !c.Audit.Valid() || c.Audit.OwnerUserID != candidate.ParticipantID || c.Audit.ActorUserID != candidate.ParticipantID || c.Audit.TargetID != candidate.TimerID || c.Audit.TargetType != "long_timer_notice" || c.Audit.Action != audit.ResourceCreated || c.Audit.Outcome != audit.Succeeded || !c.Audit.OccurredAt.Equal(c.At) {
		return false, ports.ErrInvalidArgument
	}
	published := false
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// All progress writes take this same participant/Path lock before row locks.
		if err := lockActivePath(tx, candidate.PathID, candidate.ParticipantID); err != nil {
			if errors.Is(err, ports.ErrNotFound) || errors.Is(err, ports.ErrConflict) {
				return nil
			}
			return err
		}
		var timer timerModel
		err := tx.Where("id = ? AND participant_id = ? AND path_id = ?", candidate.TimerID, candidate.ParticipantID, candidate.PathID).Take(&timer).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var receipts int64
		if err := tx.Table("long_timer_notice_receipt_models").Where("timer_id = ? AND participant_id = ?", candidate.TimerID, candidate.ParticipantID).Count(&receipts).Error; err != nil {
			return err
		}
		if receipts != 0 {
			return nil
		}
		enabled, err := channelstore.Allowed(tx, candidate.ParticipantID, "timer_health")
		if err != nil || !enabled {
			return err
		}
		var active int64
		if err := tx.Table("user_models").Where("id = ? AND status = 'active'", candidate.ParticipantID).Count(&active).Error; err != nil {
			return err
		}
		if active != 1 {
			return nil
		}
		var summary struct{ Count, Total int64 }
		if err := tx.Model(&activityModel{}).Select("count(*) AS count, COALESCE(sum(floor(extract(epoch FROM (ended_at - started_at)))),0)::bigint AS total").
			Where("participant_id = ? AND path_id = ? AND ended_at >= started_at + interval '1 second'", candidate.ParticipantID, candidate.PathID).Scan(&summary).Error; err != nil {
			return err
		}
		due, err := toTimer(timer).LongTimerNoticeDue(summary.Count, summary.Total, c.At)
		if err != nil || !due {
			return err
		}
		created := tx.Table("long_timer_notice_receipt_models").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"timer_id": candidate.TimerID, "participant_id": candidate.ParticipantID, "created_at": c.At})
		if created.Error != nil || created.RowsAffected == 0 {
			return created.Error
		}
		id := longTimerNotificationID(candidate.TimerID)
		if err := tx.Table("notification_models").Create(map[string]any{"id": id, "recipient_user_id": candidate.ParticipantID, "actor_user_id": candidate.ParticipantID, "path_id": candidate.PathID, "timer_id": candidate.TimerID, "kind": "long_timer_running", "presentation_class": "informational", "channel": "timer_health", "created_at": c.At}).Error; err != nil {
			return err
		}
		if err := channelstore.QueuePush(tx, id, candidate.ParticipantID, c.At); err != nil {
			return err
		}
		if err := tx.Create(fromAudit(c.Audit)).Error; err != nil {
			return err
		}
		published = true
		return nil
	})
	return published && err == nil, err
}

func longTimerNotificationID(timerID string) string {
	digest := sha256.Sum256([]byte(timerID))
	return "long-timer:" + hex.EncodeToString(digest[:])
}
