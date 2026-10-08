package gormstore

import (
	"bytes"
	"context"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// The injected access check preserves the external deletion fence for both
// owners. It must be supplied by the session boundary, never by an HTTP input.
func (s *Store) CompleteAccountRecovery(ctx context.Context, c application.AccountRecoveryCompletion, access func(context.Context, string) error) (application.RecoveredAccount, error) {
	if access == nil || !validRecoveryCompletion(c) {
		return application.RecoveredAccount{}, ports.ErrInvalidArgument
	}
	var result application.RecoveredAccount
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var targetIdentity identityModel
		if err := tx.Where("issuer = ? AND subject = ? AND provider = ?", c.Issuer, c.Subject, c.Provider).Take(&targetIdentity).Error; err != nil {
			return err
		}
		if targetIdentity.UserID == c.UserID {
			return ports.ErrConflict
		}
		var owners []userModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", []string{c.UserID, targetIdentity.UserID}).Order("id").Find(&owners).Error; err != nil {
			return err
		}
		if len(owners) != 2 {
			return ports.ErrNotFound
		}
		for _, user := range owners {
			if (user.ID == c.UserID && user.Status != identity.StatusProvisional) || (user.ID == targetIdentity.UserID && user.Status != identity.StatusActive) {
				return ports.ErrConflict
			}
			if err := access(ctx, user.ID); err != nil {
				return err
			}
		}
		var pending accountRecoveryChallengeModel
		if err := tx.Where("user_id = ? AND id = ?", c.UserID, c.ChallengeID).Take(&pending).Error; err != nil {
			return err
		}
		if c.Now.Before(pending.CreatedAt) || !c.Now.Before(pending.ExpiresAt) || !bytes.Equal(c.NonceHash, pending.NonceHash) || !bytes.Equal(c.SessionHash, pending.SessionHash) || pending.SourceIssuer != c.Issuer || !pending.SourceProvider.Supported() || pending.SourceProvider == c.Provider {
			return ports.ErrInvalidCredential
		}
		var old sessionModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ? AND user_id = ? AND scopes = ? AND revoked_at IS NULL AND expires_at > ? AND absolute_expires_at > ?", c.SessionHash, c.UserID, "api:onboarding", c.Now, c.Now).Take(&old).Error; err != nil {
			return err
		}
		var sourceRows, targetRows []identityModel
		if err := tx.Where("user_id = ?", c.UserID).Find(&sourceRows).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", targetIdentity.UserID).Find(&targetRows).Error; err != nil {
			return err
		}
		if len(sourceRows) != 1 || len(targetRows) != 1 {
			return ports.ErrConflict
		}
		source, target := sourceRows[0], targetRows[0]
		if source.Issuer != pending.SourceIssuer || source.Subject != pending.SourceSubject || source.Provider != pending.SourceProvider || target.Issuer != c.Issuer || target.Subject != c.Subject || target.Provider != c.Provider {
			return ports.ErrConflict
		}
		moved := tx.Model(&identityModel{}).Where("issuer = ? AND subject = ? AND user_id = ? AND provider = ?", source.Issuer, source.Subject, c.UserID, source.Provider).Update("user_id", target.UserID)
		if moved.Error != nil {
			return moved.Error
		}
		if moved.RowsAffected != 1 {
			return ports.ErrConflict
		}
		expires := c.ExpiresAt
		if expires.After(old.AbsoluteExpiresAt) {
			expires = old.AbsoluteExpiresAt
		}
		if err := tx.Create(&sessionModel{TokenHash: append([]byte(nil), c.NewSessionHash...), UserID: target.UserID, Scopes: "api:user", CreatedAt: c.Now, ExpiresAt: expires, AbsoluteExpiresAt: old.AbsoluteExpiresAt}).Error; err != nil {
			return err
		}
		linked := auditForResolvedAccount(c.Linked, target.UserID)
		linked.TargetID = string(source.Provider)
		created := auditForResolvedAccount(c.Created, target.UserID)
		for _, event := range []audit.Event{c.Completed, c.Revoked, linked, created} {
			if err := appendAuditEvent(tx, event); err != nil {
				return err
			}
		}
		// Consumed provider-token hashes are the replay ledger. Retain those
		// rows under the surviving account, but revoke their opaque credentials
		// before the enrollment cascade. No old credential gains target access.
		if err := tx.Model(&sessionModel{}).Where("user_id = ? AND identity_token_hash IS NOT NULL", c.UserID).Updates(map[string]any{
			"user_id": target.UserID, "revoked_at": gorm.Expr("COALESCE(revoked_at, ?)", c.Now),
		}).Error; err != nil {
			return err
		}
		// Enrollment invitations contain personal enrollment data and restrictive
		// references. No product data or administrator rights move to the recipient.
		if err := tx.Where("created_by_user_id = ? OR consumed_by_user_id = ? OR revoked_by_user_id = ?", c.UserID, c.UserID, c.UserID).Delete(&invitationModel{}).Error; err != nil {
			return err
		}
		removed := tx.Where("id = ? AND status = ?", c.UserID, identity.StatusProvisional).Delete(&userModel{})
		if removed.Error != nil {
			return removed.Error
		}
		if removed.RowsAffected != 1 {
			return ports.ErrConflict
		}
		result = application.RecoveredAccount{UserID: target.UserID, ExpiresAt: expires}
		return nil
	})
	if err != nil {
		return application.RecoveredAccount{}, identityMutationError(err)
	}
	return result, nil
}

func validRecoveryCompletion(c application.AccountRecoveryCompletion) bool {
	if c.UserID == "" || c.ChallengeID == "" || c.Issuer == "" || c.Subject == "" || !c.Provider.Supported() || len(c.NonceHash) != 32 || len(c.SessionHash) != 32 || len(c.NewSessionHash) != 32 || bytes.Equal(c.SessionHash, c.NewSessionHash) || c.Now.IsZero() || !c.ExpiresAt.After(c.Now) {
		return false
	}
	for _, item := range []struct {
		event  audit.Event
		action audit.Action
		kind   string
	}{
		{c.Completed, audit.ResourceDeleted, "account_enrollment"}, {c.Revoked, audit.SessionRevoked, "user"}, {c.Linked, audit.ResourceCreated, "linked_identity"}, {c.Created, audit.SessionCreated, "user"},
	} {
		e := item.event
		if !validMutationAudit(e, item.action, item.kind, c.UserID, c.UserID) || e.ActorUserID != c.UserID || e.OccurredAt.After(c.Now) || !e.OccurredAt.Equal(c.Completed.OccurredAt) || e.CorrelationID == "" || e.CorrelationID != c.Completed.CorrelationID {
			return false
		}
	}
	return true
}
