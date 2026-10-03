package gormstore

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type accountDeletionModel struct {
	UserID       string `gorm:"primaryKey"`
	DeletedAt    time.Time
	AuditEventID string
	ReceiptHash  []byte
}

func (s *Store) DeleteAccount(ctx context.Context, command application.AccountDeletionCommand) error {
	return s.deleteAccount(ctx, command, true)
}

// ReapplyAccountDeletion is used by accepted-request recovery and offline restore, never directly by HTTP.
// Old backups may contain a provisional version of a subsequently deleted user.
func (s *Store) ReapplyAccountDeletion(ctx context.Context, command application.AccountDeletionCommand) error {
	err := s.deleteAccount(ctx, command, false)
	if errors.Is(err, ports.ErrNotFound) {
		return nil
	}
	return err
}

func (s *Store) deleteAccount(ctx context.Context, command application.AccountDeletionCommand, activeOnly bool) error {
	if s == nil || s.DB == nil || command.UserID == "" || strings.TrimSpace(command.UserID) != command.UserID || command.NewID == nil || command.DeletedAt.IsZero() || len(command.ReceiptHash) != 32 ||
		!validMutationAudit(command.Audit, audit.ResourceDeleted, "account", command.UserID, command.UserID) || command.Audit.ActorUserID != command.UserID || !command.Audit.OccurredAt.Equal(command.DeletedAt) {
		return ports.ErrInvalidArgument
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var account userModel
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", command.UserID)
		if activeOnly {
			query = query.Where("status = ?", identity.StatusActive)
		}
		if err := query.First(&account).Error; err != nil {
			return err
		}
		var owned []string
		if err := tx.Table("path_models").Where("owner_user_id = ?", command.UserID).Order("id").Pluck("id", &owned).Error; err != nil {
			return err
		}
		if err := queueAccountDeletionRelationships(tx, command, owned); err != nil {
			return err
		}
		// Restrictive references are removed explicitly; all remaining personal
		// tables use account/path foreign-key cascades, including sessions/timers.
		for _, removal := range []struct {
			table, condition string
			args             []any
		}{
			{"invitation_models", "created_by_user_id = ? OR consumed_by_user_id = ? OR revoked_by_user_id = ?", []any{command.UserID, command.UserID, command.UserID}},
			{"resource_models", "owner_user_id = ?", []any{command.UserID}},
			{"path_models", "owner_user_id = ?", []any{command.UserID}},
		} {
			if err := tx.Table(removal.table).Where(removal.condition, removal.args...).Delete(&struct{}{}).Error; err != nil {
				return err
			}
		}
		markerExists := false
		if !activeOnly {
			var marker accountDeletionModel
			err := tx.Where("user_id = ?", command.UserID).Take(&marker).Error
			if err == nil {
				if !marker.DeletedAt.Equal(command.DeletedAt) || marker.AuditEventID != command.Audit.ID || !bytes.Equal(marker.ReceiptHash, command.ReceiptHash) {
					return ports.ErrConflict
				}
				markerExists = true
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if !markerExists {
			if err := tx.Create(&accountDeletionModel{UserID: command.UserID, DeletedAt: command.DeletedAt, AuditEventID: command.Audit.ID, ReceiptHash: append([]byte(nil), command.ReceiptHash...)}).Error; err != nil {
				return err
			}
		}
		var priorAuditCount int64
		if markerExists {
			if err := tx.Model(&auditEventModel{}).Where("id = ? AND owner_user_id = ? AND actor_user_id = ? AND action = ? AND target_type = ? AND target_id = ?", command.Audit.ID, command.UserID, command.UserID, audit.ResourceDeleted, "account", command.UserID).Count(&priorAuditCount).Error; err != nil {
				return err
			}
		}
		if priorAuditCount == 0 {
			if err := appendAuditEvent(tx, command.Audit); err != nil {
				return err
			}
		}
		removed := tx.Where("id = ?", command.UserID).Delete(&userModel{})
		if removed.Error != nil {
			return removed.Error
		}
		if removed.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	return err
}

func (s *Store) DeletionReceipt(ctx context.Context, userID string, hash []byte, now time.Time) (bool, error) {
	if s == nil || s.DB == nil || userID == "" || len(hash) != 32 || now.IsZero() {
		return false, ports.ErrInvalidArgument
	}
	var count int64
	err := s.DB.WithContext(ctx).Model(&accountDeletionModel{}).Where("user_id = ? AND receipt_hash = ? AND deleted_at > ? AND deleted_at <= ?", userID, hash, now.Add(-30*24*time.Hour), now).Count(&count).Error
	return count == 1, err
}
