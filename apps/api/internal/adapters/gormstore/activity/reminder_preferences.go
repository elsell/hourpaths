package activitystore

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

type goalReminderPreferenceModel struct {
	ParticipantID, PathID string
	Enabled               bool
	Revision              int64
	CreatedAt, UpdatedAt  time.Time
}

func (goalReminderPreferenceModel) TableName() string { return "goal_reminder_preference_models" }

type goalReminderPreferenceReplayModel struct {
	ParticipantID, IdempotencyKey, PathID string
	RequestHash                           []byte
	Enabled                               bool
	Revision                              int64
	CreatedAt                             time.Time
}

func (goalReminderPreferenceReplayModel) TableName() string {
	return "goal_reminder_preference_replay_models"
}

func (r *Repository) GetGoalReminderPreference(ctx context.Context, participantID, pathID string) (application.GoalReminderPreference, error) {
	if r == nil || r.DB == nil || !validReminderID(participantID) || !validReminderID(pathID) {
		return application.GoalReminderPreference{}, ports.ErrInvalidArgument
	}
	var result application.GoalReminderPreference
	err := r.DB.WithContext(ctx).Table("path_models path").
		Select("COALESCE(preference.enabled,true) AS enabled, COALESCE(preference.revision,0) AS revision").
		Joins("JOIN user_models participant ON participant.id = ? AND participant.status = 'active'", participantID).
		Joins("JOIN path_membership_models membership ON membership.path_id = path.id AND membership.user_id = participant.id AND membership.role IN ('administrator','participant')").
		Joins("LEFT JOIN goal_reminder_preference_models preference ON preference.path_id = path.id AND preference.participant_id = participant.id").
		Where("path.id = ? AND path.archived_at IS NULL", pathID).
		Where("NOT EXISTS (SELECT 1 FROM block_models b WHERE (b.blocker_user_id = participant.id AND b.blocked_user_id = path.owner_user_id) OR (b.blocked_user_id = participant.id AND b.blocker_user_id = path.owner_user_id))").
		Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, ports.ErrNotFound
	}
	return result, err
}

func (r *Repository) UpdateGoalReminderPreference(ctx context.Context, c application.GoalReminderPreferenceCommand) (application.GoalReminderPreferenceResult, error) {
	if r == nil || r.DB == nil || !validReminderID(c.ParticipantID) || !validReminderID(c.PathID) || c.ExpectedRevision < 0 || c.ExpectedRevision == math.MaxInt64 || c.At.IsZero() || c.At.Location() != time.UTC ||
		!validIdempotency(c.Idempotency, c.ParticipantID, application.UpdateGoalReminderPreferenceOperation) || len(c.Idempotency.Key) < 16 || len(c.Idempotency.Key) > 128 ||
		!c.Audit.Valid() || c.Audit.ActorUserID != c.ParticipantID || c.Audit.OwnerUserID != c.ParticipantID || c.Audit.TargetID != c.PathID || c.Audit.TargetType != "goal_reminder_preference" || c.Audit.Action != audit.ResourceUpdated || c.Audit.Outcome != audit.Succeeded || !c.Audit.OccurredAt.Equal(c.At) {
		return application.GoalReminderPreferenceResult{}, ports.ErrInvalidArgument
	}
	var result application.GoalReminderPreferenceResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActivePath(tx, c.PathID, c.ParticipantID); err != nil {
			return err
		}
		current, err := New(tx).GetGoalReminderPreference(ctx, c.ParticipantID, c.PathID)
		if err != nil {
			return err
		}
		var replay goalReminderPreferenceReplayModel
		err = tx.Where("participant_id = ? AND idempotency_key = ?", c.ParticipantID, c.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, c.Idempotency.RequestHash) || replay.PathID != c.PathID || replay.Enabled != c.Enabled || replay.Revision != c.ExpectedRevision+1 {
				return ports.ErrIdempotencyConflict
			}
			result = application.GoalReminderPreferenceResult{Preference: application.GoalReminderPreference{Enabled: replay.Enabled, Revision: replay.Revision}, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if current.Revision != c.ExpectedRevision {
			return ports.ErrConflict
		}
		next := application.GoalReminderPreference{Enabled: c.Enabled, Revision: current.Revision + 1}
		if current.Revision == 0 {
			if err := tx.Create(&goalReminderPreferenceModel{ParticipantID: c.ParticipantID, PathID: c.PathID, Enabled: next.Enabled, Revision: next.Revision, CreatedAt: c.At, UpdatedAt: c.At}).Error; err != nil {
				return err
			}
		} else {
			update := tx.Model(&goalReminderPreferenceModel{}).Where("participant_id = ? AND path_id = ? AND revision = ?", c.ParticipantID, c.PathID, current.Revision).Updates(map[string]any{"enabled": next.Enabled, "revision": next.Revision, "updated_at": c.At})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return ports.ErrConflict
			}
		}
		replay = goalReminderPreferenceReplayModel{ParticipantID: c.ParticipantID, PathID: c.PathID, IdempotencyKey: c.Idempotency.Key, RequestHash: c.Idempotency.RequestHash, Enabled: next.Enabled, Revision: next.Revision, CreatedAt: c.At}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		if err := tx.Create(fromAudit(c.Audit)).Error; err != nil {
			return err
		}
		result.Preference = next
		return nil
	})
	return result, err
}
func validReminderID(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && !strings.ContainsRune(value, '\x00')
}

var _ application.GoalReminderPreferenceRepository = (*Repository)(nil)
