package activitystore

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type offlineActivityReplay struct {
	ParticipantID string `gorm:"primaryKey"`
	Key           string `gorm:"primaryKey"`
	PathID        string
	ActivityID    string
	RequestHash   []byte
	Outcome       string
}

func (offlineActivityReplay) TableName() string { return "offline_activity_replay_models" }

func (r *Repository) SynchronizeActivity(ctx context.Context, c application.OfflineActivityCommand) (application.OfflineActivityResult, error) {
	action := audit.ResourceCreated
	if c.Kind == "edit" {
		action = audit.ResourceUpdated
	}
	if r == nil || r.DB == nil || (c.Kind != "create" && c.Kind != "edit") || c.Activity.ValidateAt(c.Activity.UpdatedAt) != nil || !validIdempotency(c.Idempotency, c.Activity.ParticipantID, application.OfflineActivityOperation) || !validActivityAudit(c.Audit, action, c.Activity.ParticipantID, c.Activity.ID) || c.Order.OperationID != c.Idempotency.Key {
		return application.OfflineActivityResult{}, ports.ErrInvalidArgument
	}
	if _, err := domain.NewActivityEditOrder(c.Order.AuthoredAt, c.Order.Counter, c.Order.OperationID); err != nil {
		return application.OfflineActivityResult{}, ports.ErrInvalidArgument
	}
	var result application.OfflineActivityResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := progresslock.LockKey(tx, "offline-activities:"+c.Activity.ParticipantID); err != nil {
			return err
		}
		joined, archived, err := lockPathMembership(tx, c.Activity.PathID, c.Activity.ParticipantID)
		if err != nil {
			return err
		}
		if c.Activity.StartedAt.Before(joined) {
			return ports.ErrNotFound
		}
		var replay offlineActivityReplay
		found := tx.Where("participant_id = ? AND key = ?", c.Activity.ParticipantID, c.Idempotency.Key).Take(&replay).Error
		if found != nil && !errors.Is(found, gorm.ErrRecordNotFound) {
			return found
		}
		if found == nil && (replay.PathID != c.Activity.PathID || replay.ActivityID != c.Activity.ID || !sameHash(replay.RequestHash, c.Idempotency.RequestHash)) {
			return ports.ErrIdempotencyConflict
		}
		var row activityModel
		read := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND participant_id = ? AND path_id = ?", c.Activity.ID, c.Activity.ParticipantID, c.Activity.PathID).Take(&row).Error
		if read != nil && !errors.Is(read, gorm.ErrRecordNotFound) {
			return read
		}
		if found == nil {
			result.Outcome = replay.Outcome
			result.Replayed = true
			if replay.Outcome == "accepted" {
				if errors.Is(read, gorm.ErrRecordNotFound) {
					result.Outcome = "deleted"
					return nil
				}
				entry := toActivity(row)
				order, err := loadEditOrder(tx, entry)
				if err != nil {
					return err
				}
				result.Activity = &entry
				result.Order = &order
			}
			return nil
		}
		result.Outcome = "accepted"
		if archived != nil {
			result.Outcome = "archived"
		} else if c.Kind == "edit" && errors.Is(read, gorm.ErrRecordNotFound) {
			result.Outcome = "deleted"
		} else if c.Kind == "create" {
			var deleted int64
			if err := tx.Model(&mutationModel{}).Where("participant_id = ? AND path_id = ? AND result_activity_id = ? AND result_activity_deleted = TRUE", c.Activity.ParticipantID, c.Activity.PathID, c.Activity.ID).Count(&deleted).Error; err != nil {
				return err
			}
			if deleted > 0 {
				result.Outcome = "deleted"
			} else {
				if read == nil {
					return ports.ErrIdempotencyConflict
				}
				if err := persistCompletedActivity(tx, c.Activity); err != nil {
					return err
				}
				if err := saveEditOrder(tx, c.Activity.ID, c.Order); err != nil {
					return err
				}
				result.Activity = &c.Activity
				result.Order = &c.Order
			}
		} else {
			prior := toActivity(row)
			order, err := loadEditOrder(tx, prior)
			if err != nil {
				return err
			}
			merged, err := prior.MergeEditByOwner(c.Activity.ParticipantID, domain.ActivityEdit{StartedAt: c.Activity.StartedAt, DurationSeconds: c.Activity.DurationSeconds(), OccurrenceTimeZone: c.Activity.OccurrenceTimeZone, Note: c.Activity.Note}, order, c.Order, c.Activity.UpdatedAt)
			if err != nil {
				return ports.ErrInvalidArgument
			}
			var count int64
			if err := tx.Model(&activityRevisionModel{}).Where("activity_id = ?", prior.ID).Count(&count).Error; err != nil {
				return err
			}
			publicChanged := prior.StartedAt != c.Activity.StartedAt || prior.EndedAt != c.Activity.EndedAt || prior.OccurrenceTimeZone != c.Activity.OccurrenceTimeZone
			revision := fromRevision(merged.Revision, count+1, publicChanged)
			if err := tx.Create(&revision).Error; err != nil {
				return err
			}
			if merged.Applied {
				next := merged.Activity
				if err := tx.Model(&activityModel{}).Where("id = ? AND participant_id = ? AND path_id = ?", next.ID, next.ParticipantID, next.PathID).Updates(map[string]any{"started_at": next.StartedAt, "ended_at": next.EndedAt, "occurrence_time_zone": next.OccurrenceTimeZone, "note": optionalNote(next.Note), "updated_at": next.UpdatedAt}).Error; err != nil {
					return err
				}
				if err := saveEditOrder(tx, next.ID, merged.Order); err != nil {
					return err
				}
				if _, err := removeUnsupportedGoalAchievements(tx, next.ParticipantID, next.PathID); err != nil {
					return err
				}
			}
			result.Activity = &merged.Activity
			result.Order = &merged.Order
		}
		replay = offlineActivityReplay{ParticipantID: c.Activity.ParticipantID, Key: c.Idempotency.Key, PathID: c.Activity.PathID, ActivityID: c.Activity.ID, RequestHash: c.Idempotency.RequestHash, Outcome: result.Outcome}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		return tx.Create(fromAudit(c.Audit)).Error
	})
	return result, err
}
