package gormstore

import (
	"bytes"
	"context"
	"errors"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type identityLinkChallengeModel struct {
	UserID               string            `gorm:"primaryKey"`
	Provider             identity.Provider `gorm:"primaryKey"`
	ID                   string
	NonceHash            []byte
	CreatedAt, ExpiresAt time.Time
}

func lockIdentityOwner(tx *gorm.DB, userID string) error {
	var user userModel
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", userID, identity.StatusActive).Take(&user).Error
}

func identityMutationError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ports.ErrConflict
	}
	return err
}

func (s *Store) ListProviderIdentities(ctx context.Context, userID string) ([]identity.Provider, error) {
	if userID == "" {
		return nil, ports.ErrInvalidArgument
	}
	var rows []identityModel
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userModel
		if err := tx.Where("id = ? AND status = ?", userID, identity.StatusActive).Take(&user).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Order("provider").Find(&rows).Error
	})
	if err != nil {
		return nil, identityMutationError(err)
	}
	providers := make([]identity.Provider, 0, len(rows))
	for _, row := range rows {
		if !row.Provider.Supported() {
			return nil, ports.ErrUnavailable
		}
		providers = append(providers, row.Provider)
	}
	return providers, nil
}

func (s *Store) AdmitIdentityLink(ctx context.Context, c application.IdentityLinkAdmission) error {
	if c.UserID == "" || c.ID == "" || !c.Provider.Supported() || len(c.NonceHash) != 32 || c.CreatedAt.IsZero() || !c.ExpiresAt.Equal(c.CreatedAt.Add(10*time.Minute)) ||
		!validMutationAudit(c.Audit, audit.ResourceCreated, "identity_link_challenge", c.UserID, c.UserID) || c.Audit.ActorUserID != c.UserID || !c.Audit.OccurredAt.Equal(c.CreatedAt) {
		return ports.ErrInvalidArgument
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityOwner(tx, c.UserID); err != nil {
			return err
		}
		var rows []identityModel
		if err := tx.Where("user_id = ?", c.UserID).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 1 {
			return ports.ErrConflict
		}
		if !rows[0].Provider.Supported() {
			return ports.ErrUnavailable
		}
		if rows[0].Provider == c.Provider {
			return ports.ErrConflict
		}
		pending := identityLinkChallengeModel{UserID: c.UserID, Provider: c.Provider, ID: c.ID, NonceHash: append([]byte(nil), c.NonceHash...), CreatedAt: c.CreatedAt, ExpiresAt: c.ExpiresAt}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "provider"}}, DoUpdates: clause.AssignmentColumns([]string{"id", "nonce_hash", "created_at", "expires_at"})}).Create(&pending).Error; err != nil {
			return err
		}
		return appendAuditEvent(tx, c.Audit)
	})
	return identityMutationError(err)
}

func (s *Store) CompleteIdentityLink(ctx context.Context, c application.IdentityLinkCompletion) error {
	if c.UserID == "" || c.ChallengeID == "" || c.Issuer == "" || c.Subject == "" || !c.Provider.Supported() || len(c.NonceHash) != 32 || c.Now.IsZero() ||
		!validMutationAudit(c.Audit, audit.ResourceCreated, "linked_identity", string(c.Provider), c.UserID) || c.Audit.ActorUserID != c.UserID || !c.Audit.OccurredAt.Equal(c.Now) {
		return ports.ErrInvalidArgument
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityOwner(tx, c.UserID); err != nil {
			return err
		}
		var pending identityLinkChallengeModel
		if err := tx.Where("user_id = ? AND provider = ? AND id = ?", c.UserID, c.Provider, c.ChallengeID).Take(&pending).Error; err != nil {
			return err
		}
		if !c.Now.Before(pending.ExpiresAt) || c.Now.Before(pending.CreatedAt) || !bytes.Equal(c.NonceHash, pending.NonceHash) {
			return ports.ErrInvalidCredential
		}
		var rows []identityModel
		if err := tx.Where("user_id = ?", c.UserID).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 1 || !rows[0].Provider.Supported() || rows[0].Provider == c.Provider {
			return ports.ErrConflict
		}
		// The issuer/subject primary key prevents stealing an identity even when two
		// owners finish concurrently. Ordinary linking never transfers provisional or
		// completed accounts; duplicate recovery is a separate dual-proof operation.
		if err := tx.Create(&identityModel{Issuer: c.Issuer, Subject: c.Subject, UserID: c.UserID, Provider: c.Provider}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND id = ?", c.UserID, c.ChallengeID).Delete(&identityLinkChallengeModel{}).Error; err != nil {
			return err
		}
		return appendAuditEvent(tx, c.Audit)
	})
	return identityMutationError(err)
}

func (s *Store) UnlinkProviderIdentity(ctx context.Context, c application.IdentityUnlink) error {
	if c.UserID == "" || !c.Provider.Supported() || !validMutationAudit(c.Audit, audit.ResourceDeleted, "linked_identity", string(c.Provider), c.UserID) || c.Audit.ActorUserID != c.UserID {
		return ports.ErrInvalidArgument
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityOwner(tx, c.UserID); err != nil {
			return err
		}
		var rows []identityModel
		if err := tx.Where("user_id = ?", c.UserID).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) < 2 {
			return ports.ErrConflict
		}
		for _, row := range rows {
			if !row.Provider.Supported() {
				return ports.ErrUnavailable
			}
		}
		result := tx.Where("user_id = ? AND provider = ?", c.UserID, c.Provider).Delete(&identityModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		// An old pending authorization must not reattach an identity after unlink.
		if err := tx.Where("user_id = ?", c.UserID).Delete(&identityLinkChallengeModel{}).Error; err != nil {
			return err
		}
		return appendAuditEvent(tx, c.Audit)
	})
	return identityMutationError(err)
}
