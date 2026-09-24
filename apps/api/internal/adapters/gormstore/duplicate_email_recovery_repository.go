package gormstore

import (
	"context"
	"errors"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type duplicateEmailRecoveryDeclineModel struct {
	ProvisionalUserID string `gorm:"primaryKey"`
	NormalizedEmail   string `gorm:"primaryKey"`
	CreatedAt         time.Time
}

func (duplicateEmailRecoveryDeclineModel) TableName() string {
	return "duplicate_email_recovery_declines"
}

func (s *Store) HasActiveEmailMatch(ctx context.Context, claims ports.Claims) (bool, error) {
	if !claims.EmailVerified {
		return false, nil
	}
	email := normalizeEmail(claims.Email)
	if email == "" {
		return false, nil
	}
	provisionalUserID := identity.UserID(claims.Issuer, claims.Subject)
	var suppressed int64
	if err := s.DB.WithContext(ctx).Model(&duplicateEmailRecoveryDeclineModel{}).
		Where("provisional_user_id = ? AND normalized_email = ?", provisionalUserID, email).
		Limit(1).Count(&suppressed).Error; err != nil {
		return false, err
	}
	if suppressed > 0 {
		return false, nil
	}
	var matches int64
	if err := s.DB.WithContext(ctx).Model(&userModel{}).
		Where("status = ? AND provider_email_verified = ? AND lower(email) = ?", identity.StatusActive, true, email).
		Limit(1).Count(&matches).Error; err != nil {
		return false, err
	}
	return matches > 0, nil
}

func (s *Store) DeclineDuplicateEmailRecovery(ctx context.Context, provisionalUserID string, event audit.Event) error {
	if !validMutationAudit(event, audit.DuplicateEmailRecoveryDeclined, "user", provisionalUserID, provisionalUserID) || event.ActorUserID != provisionalUserID {
		return ports.ErrInvalidArgument
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userModel
		result := tx.Select("id", "email", "provider_email_verified", "status").Where("id = ? AND status = ? AND provider_email_verified = ?", provisionalUserID, identity.StatusProvisional, true).Clauses(clause.Locking{Strength: "UPDATE"}).First(&user)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ports.ErrNotFound
		}
		if result.Error != nil {
			return result.Error
		}
		decline, err := identity.NewDuplicateEmailRecoveryDecline(user.ID, user.Email)
		if err != nil {
			return ports.ErrNotFound
		}
		row := duplicateEmailRecoveryDeclineModel{ProvisionalUserID: decline.ProvisionalUserID, NormalizedEmail: decline.NormalizedEmail, CreatedAt: event.OccurredAt}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return nil
		}
		return appendAuditEvent(tx, event)
	})
}
