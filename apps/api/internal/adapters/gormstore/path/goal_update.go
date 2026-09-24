package pathstore

import (
	"bytes"
	"context"
	"errors"
	"strings"

	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UpdateGoals atomically replaces both optional goals, records the successful
// mutation, and projects the actor's unchanged canonical activity under the
// replacement interval goal.
func (r *Repository) UpdateGoals(ctx context.Context, command application.UpdateGoalsCommand) (application.UpdateGoalsResult, error) {
	if !validGoalUpdateCommand(r, command) {
		return application.UpdateGoalsResult{}, ports.ErrInvalidArgument
	}

	var result application.UpdateGoalsResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{
			PrincipalID: command.Idempotency.PrincipalID,
			Operation:   command.Idempotency.Operation,
			Key:         command.Idempotency.Key,
			RequestHash: append([]byte(nil), command.Idempotency.RequestHash...),
			ResourceID:  string(command.Path.ID),
			CreatedAt:   command.ProjectedAt,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}

		replayed := created.RowsAffected == 0
		if replayed {
			var existingReservation idempotencyModel
			if err := tx.Where(
				"principal_id = ? AND operation = ? AND key = ?",
				command.Idempotency.PrincipalID,
				command.Idempotency.Operation,
				command.Idempotency.Key,
			).First(&existingReservation).Error; err != nil {
				return err
			}
			if existingReservation.ResourceID != string(command.Path.ID) ||
				!bytes.Equal(existingReservation.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
		}

		row, err := goalUpdatePathForActor(tx, command.ActorUserID, command.Path.ID)
		if err != nil {
			return err
		}
		current, err := toEntity(row)
		if err != nil {
			return err
		}
		if current.Archived() {
			return ports.ErrConflict
		}
		if replayed && (current.IntervalGoal != command.Path.IntervalGoal || current.OverallTarget != command.Path.OverallTarget) {
			return ports.ErrIdempotencyConflict
		}

		if !replayed {
			if current.GoalConfiguration() != command.ExpectedGoals {
				return ports.ErrConflict
			}
			if !sameGoalUpdateIdentity(current, command.Path) {
				return ports.ErrInvalidArgument
			}
			replacement := fromEntity(command.Path)
			updated := tx.Model(&model{}).
				Where("id = ? AND owner_user_id = ?", command.Path.ID, command.Path.OwnerUserID).
				Updates(map[string]any{
					"interval_goal_target_seconds": replacement.IntervalGoalTargetSeconds,
					"interval_goal_recurrence":     replacement.IntervalGoalRecurrence,
					"interval_goal_start_minute":   replacement.IntervalGoalStartMinute,
					"interval_goal_start_hour":     replacement.IntervalGoalStartHour,
					"interval_goal_start_weekday":  replacement.IntervalGoalStartWeekday,
					"interval_goal_start_day":      replacement.IntervalGoalStartDay,
					"interval_goal_start_month":    replacement.IntervalGoalStartMonth,
					"overall_target_seconds":       replacement.OverallTargetSeconds,
					"updated_at":                   command.Path.UpdatedAt,
				})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ports.ErrNotFound
			}
			if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
				return err
			}
			current = command.Path
		}

		projection, err := goalUpdateProjection(ctx, tx, command, current)
		if err != nil {
			return err
		}
		result = projection
		result.Replayed = replayed
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

func validGoalUpdateCommand(r *Repository, command application.UpdateGoalsCommand) bool {
	if r == nil || r.DB == nil || strings.TrimSpace(command.ActorUserID) != command.ActorUserID ||
		command.ActorUserID == "" ||
		strings.TrimSpace(string(command.Path.ID)) != string(command.Path.ID) ||
		command.Path.ID == "" ||
		strings.TrimSpace(command.Path.OwnerUserID) != command.Path.OwnerUserID ||
		command.Path.OwnerUserID == "" ||
		!command.ExpectedGoals.Valid() ||
		(command.ParticipantTimeZone != "" && (strings.TrimSpace(command.ParticipantTimeZone) != command.ParticipantTimeZone || command.ParticipantTimeZone == "Local")) ||
		(command.Path.IntervalGoal.Present && command.ParticipantTimeZone == "") ||
		command.ProjectedAt.IsZero() || command.ProjectedAt.Before(command.Path.CreatedAt) ||
		!command.ProjectedAt.Equal(command.Path.UpdatedAt) ||
		!validEntityForPersistence(command.Path) || command.Path.CreatedAt.IsZero() ||
		command.Idempotency.PrincipalID != command.ActorUserID ||
		command.Idempotency.Operation != application.UpdateGoalsOperation ||
		strings.TrimSpace(command.Idempotency.Key) == "" || len(command.Idempotency.RequestHash) != 32 ||
		!validEvent(command.Audit, audit.ResourceUpdated, command.Path) ||
		command.Audit.ActorUserID != command.ActorUserID ||
		!command.Audit.OccurredAt.Equal(command.ProjectedAt) {
		return false
	}
	return true
}

func goalUpdatePathForActor(tx *gorm.DB, actor string, id domain.ID) (model, error) {
	var row model
	err := tx.
		Table("path_models").
		Select("path_models.*").
		Joins("LEFT JOIN path_membership_models ON path_membership_models.path_id = path_models.id AND path_membership_models.user_id = ?", actor).
		Where("path_models.id = ? AND (path_models.owner_user_id = ? OR path_membership_models.user_id = ?)", id, actor, actor).
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "path_models"}}).
		First(&row).Error
	return row, err
}

func sameGoalUpdateIdentity(current, replacement domain.Entity) bool {
	return current.ID == replacement.ID &&
		current.OwnerUserID == replacement.OwnerUserID &&
		current.Name == replacement.Name &&
		current.Visibility == replacement.Visibility &&
		current.CreatedAt.Equal(replacement.CreatedAt)
}

func goalUpdateProjection(
	ctx context.Context,
	tx *gorm.DB,
	command application.UpdateGoalsCommand,
	path domain.Entity,
) (application.UpdateGoalsResult, error) {
	var request *activityapp.IntervalProgressRequest
	if path.IntervalGoal.Present {
		window, err := activityapp.CurrentIntervalWindow(path.IntervalGoal, command.ParticipantTimeZone, command.ProjectedAt)
		if err != nil {
			return application.UpdateGoalsResult{}, ports.ErrInvalidArgument
		}
		request = &activityapp.IntervalProgressRequest{
			TargetSeconds: path.IntervalGoal.TargetSeconds,
			Window:        window,
		}
	}
	projection, err := activitystore.New(tx).CurrentProjection(
		ctx,
		command.ActorUserID,
		string(path.ID),
		request,
	)
	if err != nil {
		return application.UpdateGoalsResult{}, err
	}
	result := application.UpdateGoalsResult{
		Path:               path,
		AccumulatedSeconds: projection.AccumulatedSeconds,
	}
	if projection.IntervalProgress != nil {
		result.IntervalProgress = &application.GoalIntervalProgress{
			TargetSeconds:      projection.IntervalProgress.TargetSeconds,
			AccumulatedSeconds: projection.IntervalProgress.AccumulatedSeconds,
			StartedAt:          projection.IntervalProgress.Window.StartedAt,
			EndedAt:            projection.IntervalProgress.Window.EndedAt,
		}
	}
	return result, nil
}
