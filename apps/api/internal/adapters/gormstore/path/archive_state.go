package pathstore

import (
	"bytes"
	"context"
	"errors"
	"strings"

	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) SetArchiveState(ctx context.Context, command application.SetArchiveStateCommand) (application.SetArchiveStateResult, error) {
	if !validSetArchiveStateCommand(r, command) {
		return application.SetArchiveStateResult{}, ports.ErrInvalidArgument
	}
	var result application.SetArchiveStateResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{
			PrincipalID: command.Idempotency.PrincipalID,
			Operation:   command.Idempotency.Operation,
			Key:         command.Idempotency.Key,
			RequestHash: append([]byte(nil), command.Idempotency.RequestHash...),
			ResourceID:  string(command.Path.ID),
			CreatedAt:   command.ChangedAt,
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
				command.Idempotency.PrincipalID, command.Idempotency.Operation, command.Idempotency.Key,
			).First(&existingReservation).Error; err != nil {
				return err
			}
			if existingReservation.ResourceID != string(command.Path.ID) || !bytes.Equal(existingReservation.RequestHash, command.Idempotency.RequestHash) {
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
		if replayed {
			if current.Archived() != command.Archived {
				return ports.ErrIdempotencyConflict
			}
			result = application.SetArchiveStateResult{Path: current, Replayed: true}
			return nil
		}
		if current.Archived() != command.ExpectedArchived {
			return ports.ErrConflict
		}
		if !sameArchiveStateIdentity(current, command.Path) || command.Path.Archived() != command.Archived || !command.Path.UpdatedAt.Equal(command.ChangedAt) {
			return ports.ErrInvalidArgument
		}
		if command.Archived {
			if !command.Path.ArchivedAt.Equal(command.ChangedAt) {
				return ports.ErrInvalidArgument
			}
			if err := activitystore.New(tx).StopPathTimersForArchive(ctx, string(command.Path.ID), command.ChangedAt, command.NewActivityID); err != nil {
				return err
			}
		} else if !command.Path.ArchivedAt.IsZero() {
			return ports.ErrInvalidArgument
		}
		replacement := fromEntity(command.Path)
		updated := tx.Model(&model{}).
			Where("id = ? AND owner_user_id = ?", command.Path.ID, command.Path.OwnerUserID).
			Updates(map[string]any{"archived_at": replacement.ArchivedAt, "updated_at": command.Path.UpdatedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result = application.SetArchiveStateResult{Path: command.Path}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

func validSetArchiveStateCommand(r *Repository, command application.SetArchiveStateCommand) bool {
	return r != nil && r.DB != nil &&
		strings.TrimSpace(command.ActorUserID) == command.ActorUserID && command.ActorUserID != "" &&
		command.ExpectedArchived != command.Archived &&
		validEntityForPersistence(command.Path) && command.Path.ID != "" && command.Path.OwnerUserID != "" &&
		!command.Path.CreatedAt.IsZero() && !command.Path.UpdatedAt.IsZero() && !command.ChangedAt.IsZero() &&
		command.Idempotency.PrincipalID == command.ActorUserID &&
		command.Idempotency.Operation == application.SetArchiveStateOperation &&
		strings.TrimSpace(command.Idempotency.Key) != "" && len(command.Idempotency.RequestHash) == 32 &&
		validEvent(command.Audit, audit.ResourceUpdated, command.Path) &&
		command.Audit.ActorUserID == command.ActorUserID && command.Audit.OccurredAt.Equal(command.ChangedAt) &&
		command.NewActivityID != nil
}

func sameArchiveStateIdentity(current, replacement domain.Entity) bool {
	return current.ID == replacement.ID && current.OwnerUserID == replacement.OwnerUserID &&
		current.Attributes == replacement.Attributes && current.CreatedAt.Equal(replacement.CreatedAt)
}
