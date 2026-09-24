package pathstore

import (
	"bytes"
	"context"
	"errors"
	"strings"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) Rename(ctx context.Context, command application.RenameCommand) (application.RenameResult, error) {
	if !validRenameCommand(r, command) {
		return application.RenameResult{}, ports.ErrInvalidArgument
	}
	var result application.RenameResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{
			PrincipalID: command.Idempotency.PrincipalID,
			Operation:   command.Idempotency.Operation,
			Key:         command.Idempotency.Key,
			RequestHash: append([]byte(nil), command.Idempotency.RequestHash...),
			ResourceID:  string(command.Path.ID),
			CreatedAt:   command.Path.UpdatedAt,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		replayed := created.RowsAffected == 0
		if replayed {
			var existing idempotencyModel
			if err := tx.Where(
				"principal_id = ? AND operation = ? AND key = ?",
				command.Idempotency.PrincipalID, command.Idempotency.Operation, command.Idempotency.Key,
			).First(&existing).Error; err != nil {
				return err
			}
			if existing.ResourceID != string(command.Path.ID) ||
				!bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
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
		if replayed {
			if current.Name != command.Path.Name {
				return ports.ErrIdempotencyConflict
			}
			result = application.RenameResult{Path: current, Replayed: true}
			return nil
		}
		if current.Name != command.ExpectedName {
			return ports.ErrConflict
		}
		if command.Path.UpdatedAt.Before(current.UpdatedAt) {
			return ports.ErrConflict
		}
		if !sameRenameIdentity(current, command.Path) {
			return ports.ErrInvalidArgument
		}
		updated := tx.Model(&model{}).
			Where("id = ? AND archived_at IS NULL", command.Path.ID).
			Updates(map[string]any{"name": command.Path.Name, "updated_at": command.Path.UpdatedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrConflict
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result = application.RenameResult{Path: command.Path}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

func validRenameCommand(r *Repository, command application.RenameCommand) bool {
	expected := command.Path.Attributes
	expected.Name = command.ExpectedName
	validatedExpected, expectedErr := domain.New(command.Path.ID, command.Path.OwnerUserID, expected)
	return r != nil && r.DB != nil &&
		command.ActorUserID != "" && strings.TrimSpace(command.ActorUserID) == command.ActorUserID &&
		command.ExpectedName != "" && strings.TrimSpace(command.ExpectedName) == command.ExpectedName &&
		expectedErr == nil && validatedExpected.Name == command.ExpectedName &&
		command.Path.ID != "" && command.Path.OwnerUserID != "" && !command.Path.Archived() &&
		validEntityForPersistence(command.Path) && !command.Path.CreatedAt.IsZero() &&
		!command.Path.UpdatedAt.IsZero() && !command.Path.UpdatedAt.Before(command.Path.CreatedAt) &&
		command.Idempotency.PrincipalID == command.ActorUserID &&
		command.Idempotency.Operation == application.RenameOperation &&
		command.Idempotency.Key != "" && len(command.Idempotency.RequestHash) == 32 &&
		validEvent(command.Audit, audit.ResourceUpdated, command.Path) &&
		command.Audit.ActorUserID == command.ActorUserID &&
		command.Audit.OccurredAt.Equal(command.Path.UpdatedAt)
}

func sameRenameIdentity(current, replacement domain.Entity) bool {
	return current.ID == replacement.ID &&
		current.OwnerUserID == replacement.OwnerUserID &&
		current.Visibility == replacement.Visibility &&
		current.IntervalGoal == replacement.IntervalGoal &&
		current.OverallTarget == replacement.OverallTarget &&
		current.CreatedAt.Equal(replacement.CreatedAt)
}
