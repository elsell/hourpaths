package pathstore

import (
	"context"
	"errors"
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type pathAppearanceModel struct {
	UserID    string `gorm:"primaryKey"`
	PathID    string `gorm:"primaryKey"`
	Color     string
	Emoji     string
	Revision  int64
	UpdatedAt time.Time
}

func (pathAppearanceModel) TableName() string { return "path_appearance_models" }

type pathAppearanceMutationModel struct {
	UserID         string `gorm:"primaryKey"`
	PathID         string `gorm:"primaryKey"`
	IdempotencyKey string `gorm:"primaryKey"`
	RequestHash    []byte
	Color          string
	Emoji          string
	Revision       int64
	UpdatedAt      time.Time
}

func (pathAppearanceMutationModel) TableName() string { return "path_appearance_mutation_models" }

func appearanceMembership(db *gorm.DB, userID string, pathID domain.ID, lock bool) error {
	if userID == "" || pathID == "" {
		return ports.ErrInvalidArgument
	}
	query := db.Table("path_models").Select("id").Where("id = ? AND (owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models member WHERE member.path_id = path_models.id AND member.user_id = ?))", string(pathID), userID, userID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row struct{ ID string }
	err := query.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	return err
}

func (r *Repository) ReadAppearance(ctx context.Context, userID string, pathID domain.ID) (domain.Appearance, error) {
	if r == nil || r.DB == nil {
		return domain.Appearance{}, ports.ErrInvalidArgument
	}
	db := r.DB.WithContext(ctx)
	if err := appearanceMembership(db, userID, pathID, false); err != nil {
		return domain.Appearance{}, err
	}
	var row pathAppearanceModel
	err := db.Where("user_id = ? AND path_id = ?", userID, string(pathID)).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Appearance{}, nil
	}
	if err != nil {
		return domain.Appearance{}, err
	}
	result := domain.Appearance{Color: row.Color, Emoji: row.Emoji, Revision: row.Revision}
	if !result.ValidChoice() || result.Revision < 1 {
		return domain.Appearance{}, errInvalidPersistedPath
	}
	return result, nil
}

func (r *Repository) SaveAppearance(ctx context.Context, command application.AppearanceCommand) (domain.Appearance, error) {
	if r == nil || r.DB == nil || command.UserID == "" || command.PathID == "" || !command.Appearance.ValidChoice() || command.Appearance.Revision < 0 || command.UpdatedAt.IsZero() ||
		command.Idempotency.PrincipalID != command.UserID || command.Idempotency.Operation != "path.appearance.update" || len(command.Idempotency.RequestHash) != 32 || command.Idempotency.Key == "" ||
		!command.Audit.Valid() || command.Audit.ActorUserID != command.UserID || command.Audit.OwnerUserID != command.UserID || command.Audit.TargetType != "path_appearance" || command.Audit.TargetID != string(command.PathID) || command.Audit.Action != audit.ResourceUpdated || command.Audit.Outcome != audit.Succeeded || !command.Audit.OccurredAt.Equal(command.UpdatedAt) {
		return domain.Appearance{}, ports.ErrInvalidArgument
	}
	var result domain.Appearance
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := appearanceMembership(tx, command.UserID, command.PathID, true); err != nil {
			return err
		}
		var receipt pathAppearanceMutationModel
		err := tx.Where("user_id = ? AND path_id = ? AND idempotency_key = ?", command.UserID, string(command.PathID), command.Idempotency.Key).Take(&receipt).Error
		if err == nil {
			if string(receipt.RequestHash) != string(command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = domain.Appearance{Color: receipt.Color, Emoji: receipt.Emoji, Revision: receipt.Revision}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var current pathAppearanceModel
		err = tx.Where("user_id = ? AND path_id = ?", command.UserID, string(command.PathID)).Take(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if current.Revision != command.Appearance.Revision {
			return ports.ErrConflict
		}
		row := pathAppearanceModel{UserID: command.UserID, PathID: string(command.PathID), Color: command.Appearance.Color, Emoji: command.Appearance.Emoji, Revision: current.Revision + 1, UpdatedAt: command.UpdatedAt}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "path_id"}}, DoUpdates: clause.AssignmentColumns([]string{"color", "emoji", "revision", "updated_at"})}).Create(&row).Error; err != nil {
			return err
		}
		receipt = pathAppearanceMutationModel{UserID: row.UserID, PathID: row.PathID, IdempotencyKey: command.Idempotency.Key, RequestHash: command.Idempotency.RequestHash, Color: row.Color, Emoji: row.Emoji, Revision: row.Revision, UpdatedAt: row.UpdatedAt}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result = domain.Appearance{Color: row.Color, Emoji: row.Emoji, Revision: row.Revision}
		return nil
	})
	return result, err
}
