package activitystore

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) DeleteActivity(ctx context.Context, command application.DeleteActivityCommand) (application.DeleteActivityResult, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(command.ActivityID) == "" || strings.TrimSpace(command.PathID) == "" || strings.TrimSpace(command.ParticipantID) == "" || !validIdempotency(command.Idempotency, command.ParticipantID, application.DeleteActivityOperation) || !validActivityAudit(command.Audit, audit.ResourceDeleted, command.ParticipantID, command.ActivityID) || !validIntervalProgressRequest(command.IntervalProgress) {
		return application.DeleteActivityResult{}, ports.ErrInvalidArgument
	}
	var result application.DeleteActivityResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActivePath(tx, command.PathID, command.ParticipantID); err != nil {
			return err
		}
		if replay, found, err := replayedDelete(tx, command); err != nil || found {
			result = replay
			return err
		}

		var current activityModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND participant_id = ? AND path_id = ?", command.ActivityID, command.ParticipantID, command.PathID).
			First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if replay, found, replayErr := replayedDelete(tx, command); replayErr != nil || found {
				result = replay
				return replayErr
			}
			return ports.ErrNotFound
		}
		if err != nil {
			return err
		}
		if replay, found, err := replayedDelete(tx, command); err != nil || found {
			result = replay
			return err
		}
		var practiceEvent practiceFeedEventModel
		if err := tx.Where("source_activity_id = ? AND participant_user_id = ? AND path_id = ?", command.ActivityID, command.ParticipantID, command.PathID).Take(&practiceEvent).Error; err != nil {
			return err
		}
		if practiceEvent.ID != "practice:"+command.ActivityID {
			return errors.New("persisted practice feed event is invalid")
		}

		activityID := command.ActivityID
		initialAccumulatedSeconds := int64(0)
		initialSessionCount := int64(0)
		initialUnreadNotificationCount := int64(0)
		removedFeedEventIDs := []string{practiceEvent.ID}
		reservation := mutationModel{
			ParticipantID: command.ParticipantID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key,
			RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), PathID: command.PathID,
			ResultActivityID: &activityID, ResultActivityDeleted: true,
			ResultAccumulatedSeconds: &initialAccumulatedSeconds,
			ResultSessionCount:       &initialSessionCount, ResultUnreadNotificationCount: &initialUnreadNotificationCount,
			ResultRemovedFeedEventIDs: removedFeedEventIDs,
		}
		if err := tx.Create(&reservation).Error; err != nil {
			return err
		}

		scrubbed := tx.Model(&mutationModel{}).
			Where("participant_id = ? AND result_activity_id = ? AND NOT (operation = ? AND key = ?)", command.ParticipantID, command.ActivityID, command.Idempotency.Operation, command.Idempotency.Key).
			Updates(map[string]any{
				"result_activity_saved": false, "result_activity_deleted": true,
				"result_started_at": nil, "result_ended_at": nil, "result_time_zone": "",
				"result_created_at": nil, "result_updated_at": nil, "result_note": nil, "result_version": nil,
				"result_accumulated_seconds": nil,
				"result_session_count":       nil, "result_unread_notification_count": nil, "result_removed_feed_event_ids": nil,
			})
		if scrubbed.Error != nil {
			return scrubbed.Error
		}

		deleted := tx.Where("id = ? AND participant_id = ? AND path_id = ?", command.ActivityID, command.ParticipantID, command.PathID).Delete(&activityModel{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		removedAchievements, err := removeUnsupportedGoalAchievements(tx, command.ParticipantID, command.PathID)
		if err != nil {
			return err
		}
		removedFeedEventIDs = append(removedFeedEventIDs, removedAchievements...)
		sort.Strings(removedFeedEventIDs)
		projection, err := currentProjection(tx, command.ParticipantID, command.PathID, command.IntervalProgress)
		if err != nil {
			return err
		}
		var sessionCount int64
		if err := tx.Model(&activityModel{}).Where("participant_id = ? AND path_id = ?", command.ParticipantID, command.PathID).Count(&sessionCount).Error; err != nil {
			return err
		}
		unreadNotificationCount, err := activityDeletionVisibleUnreadNotificationCount(tx, command.ParticipantID, command.Audit.OccurredAt)
		if err != nil {
			return err
		}
		if err := tx.Model(&mutationModel{}).
			Where("participant_id = ? AND operation = ? AND key = ?", command.ParticipantID, command.Idempotency.Operation, command.Idempotency.Key).
			Updates(map[string]any{"result_accumulated_seconds": projection.AccumulatedSeconds, "result_session_count": sessionCount, "result_unread_notification_count": unreadNotificationCount, "result_removed_feed_event_ids": pq.StringArray(removedFeedEventIDs)}).Error; err != nil {
			return err
		}
		result = application.DeleteActivityResult{AccumulatedSeconds: projection.AccumulatedSeconds, SessionCount: sessionCount, UnreadNotificationCount: unreadNotificationCount, RemovedFeedEventIDs: append([]string(nil), removedFeedEventIDs...), IntervalProgress: projection.IntervalProgress}
		return nil
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		replay, found, replayErr := replayedDelete(r.DB.WithContext(ctx), command)
		if replayErr != nil {
			return application.DeleteActivityResult{}, replayErr
		}
		if found {
			return replay, nil
		}
	}
	return result, err
}

func replayedDelete(tx *gorm.DB, command application.DeleteActivityCommand) (application.DeleteActivityResult, bool, error) {
	existing, err := findMutation(tx, command.Idempotency)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return application.DeleteActivityResult{}, false, nil
	}
	if err != nil {
		return application.DeleteActivityResult{}, false, err
	}
	if !sameHash(existing.RequestHash, command.Idempotency.RequestHash) {
		return application.DeleteActivityResult{}, true, ports.ErrIdempotencyConflict
	}
	if existing.ParticipantID != command.ParticipantID || existing.PathID != command.PathID || existing.Operation != application.DeleteActivityOperation || existing.ResultActivityID == nil || *existing.ResultActivityID != command.ActivityID || !existing.ResultActivityDeleted || existing.ResultActivitySaved || existing.ResultStartedAt != nil || existing.ResultEndedAt != nil || existing.ResultTimeZone != "" || existing.ResultCreatedAt != nil || existing.ResultUpdatedAt != nil || existing.ResultNote != nil || existing.ResultVersion != nil || existing.ResultAccumulatedSeconds == nil || *existing.ResultAccumulatedSeconds < 0 || existing.ResultSessionCount == nil || *existing.ResultSessionCount < 0 || existing.ResultUnreadNotificationCount == nil || *existing.ResultUnreadNotificationCount < 0 || !validPersistedRemovedFeedEventIDs(existing.ResultRemovedFeedEventIDs, command.ActivityID) {
		return application.DeleteActivityResult{}, true, errors.New("persisted activity deletion result is invalid")
	}
	projection, err := currentProjection(tx, command.ParticipantID, command.PathID, command.IntervalProgress)
	if err != nil {
		return application.DeleteActivityResult{}, true, err
	}
	return application.DeleteActivityResult{AccumulatedSeconds: *existing.ResultAccumulatedSeconds, SessionCount: *existing.ResultSessionCount, UnreadNotificationCount: *existing.ResultUnreadNotificationCount, RemovedFeedEventIDs: append([]string(nil), existing.ResultRemovedFeedEventIDs...), IntervalProgress: projection.IntervalProgress, Replayed: true}, true, nil
}

const activityDeletionVisibleNotificationPredicate = `notification_models.deleted_at IS NULL AND
  NOT EXISTS (SELECT 1 FROM block_models notification_block
    WHERE (notification_block.blocker_user_id = notification_models.recipient_user_id AND notification_block.blocked_user_id = notification_models.actor_user_id)
       OR (notification_block.blocker_user_id = notification_models.actor_user_id AND notification_block.blocked_user_id = notification_models.recipient_user_id)) AND
  (notification_models.kind <> 'path_invitation_received' OR (path_invitation_models.accepted_at IS NULL AND path_invitation_models.rejected_at IS NULL AND path_invitation_models.canceled_at IS NULL)) AND
  (notification_models.kind <> 'path_ownership_transfer_received' OR (path_ownership_transfer_models.accepted_at IS NULL AND path_ownership_transfer_models.declined_at IS NULL AND path_ownership_transfer_models.canceled_at IS NULL AND path_ownership_transfer_models.expired_at IS NULL AND path_ownership_transfer_models.expires_at > CURRENT_TIMESTAMP)) AND
  (notification_models.kind <> 'follow_request_received' OR (follow_request_models.accepted_at IS NULL AND follow_request_models.rejected_at IS NULL AND follow_request_models.canceled_at IS NULL))`

func activityDeletionVisibleUnreadNotificationCount(tx *gorm.DB, recipientUserID string, snapshot time.Time) (int64, error) {
	var count int64
	err := tx.Table("notification_models").
		Joins("LEFT JOIN path_invitation_models ON path_invitation_models.id = notification_models.path_invitation_id").
		Joins("LEFT JOIN path_ownership_transfer_models ON path_ownership_transfer_models.id = notification_models.path_ownership_transfer_id").
		Joins("LEFT JOIN follow_request_models ON follow_request_models.id = notification_models.follow_request_id").
		Joins("JOIN user_models AS notification_actor ON notification_actor.id = notification_models.actor_user_id").
		Where("notification_models.recipient_user_id = ? AND notification_models.created_at <= ? AND notification_models.read_at IS NULL AND "+activityDeletionVisibleNotificationPredicate, recipientUserID, snapshot).
		Count(&count).Error
	return count, err
}

func validPersistedRemovedFeedEventIDs(ids []string, activityID string) bool {
	if len(ids) == 0 || !slices.IsSorted(ids) {
		return false
	}
	wantPracticeID := "practice:" + activityID
	practiceCount := 0
	for index, id := range ids {
		if strings.TrimSpace(id) != id || id == "" || (index > 0 && ids[index-1] == id) {
			return false
		}
		if id == wantPracticeID {
			practiceCount++
		} else if !strings.HasPrefix(id, "achievement:") || id == "achievement:" {
			return false
		}
	}
	return practiceCount == 1
}
