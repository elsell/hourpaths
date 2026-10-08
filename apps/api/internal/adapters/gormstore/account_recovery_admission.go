package gormstore

import (
	"context"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type accountRecoveryChallengeModel struct {
	UserID                          string `gorm:"primaryKey"`
	ID, SourceIssuer, SourceSubject string
	SourceProvider                  identity.Provider
	NonceHash                       []byte
	SessionHash                     []byte
	CreatedAt, ExpiresAt            time.Time
}

func (s *Store) AdmitAccountRecovery(ctx context.Context, c application.AccountRecoveryAdmission) (identity.Provider, error) {
	if c.UserID == "" || c.ID == "" || len(c.NonceHash) != 32 || len(c.SessionHash) != 32 || c.CreatedAt.IsZero() || !c.ExpiresAt.Equal(c.CreatedAt.Add(10*time.Minute)) ||
		!validMutationAudit(c.Audit, audit.ResourceCreated, "account_recovery_challenge", c.UserID, c.UserID) || c.Audit.ActorUserID != c.UserID || !c.Audit.OccurredAt.Equal(c.CreatedAt) {
		return "", ports.ErrInvalidArgument
	}
	var expected identity.Provider
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ? AND provider_email_verified = ?", c.UserID, identity.StatusProvisional, true).Take(&user).Error; err != nil {
			return err
		}
		var session sessionModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ? AND user_id = ? AND scopes = ? AND revoked_at IS NULL AND expires_at > ? AND absolute_expires_at > ?", c.SessionHash, c.UserID, "api:onboarding", c.CreatedAt, c.CreatedAt).Take(&session).Error; err != nil {
			return err
		}
		email := normalizeEmail(user.Email)
		if email == "" {
			return ports.ErrConflict
		}
		var matching int64
		if err := tx.Model(&userModel{}).Where("id <> ? AND status = ? AND provider_email_verified = ? AND lower(email) = ?", c.UserID, identity.StatusActive, true, email).Limit(1).Count(&matching).Error; err != nil {
			return err
		}
		if matching == 0 {
			return ports.ErrConflict
		}
		var rows []identityModel
		if err := tx.Where("user_id = ?", c.UserID).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 1 || !rows[0].Provider.Supported() {
			return ports.ErrConflict
		}
		source := rows[0]
		expected = identity.ProviderGoogle
		if source.Provider == identity.ProviderGoogle {
			expected = identity.ProviderApple
		}
		pending := accountRecoveryChallengeModel{UserID: c.UserID, ID: c.ID, SourceIssuer: source.Issuer, SourceSubject: source.Subject, SourceProvider: source.Provider, NonceHash: append([]byte(nil), c.NonceHash...), SessionHash: append([]byte(nil), c.SessionHash...), CreatedAt: c.CreatedAt, ExpiresAt: c.ExpiresAt}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"id", "source_issuer", "source_subject", "source_provider", "nonce_hash", "session_hash", "created_at", "expires_at"})}).Create(&pending).Error; err != nil {
			return err
		}
		return appendAuditEvent(tx, c.Audit)
	})
	if err != nil {
		return "", identityMutationError(err)
	}
	return expected, nil
}
