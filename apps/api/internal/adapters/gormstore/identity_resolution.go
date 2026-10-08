package gormstore

import (
	"context"
	"errors"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) ResolveOrCreate(ctx context.Context, claims ports.Claims, event, profileEvent, invitationEvent audit.Event, allowCreate bool) (identity.User, error) {
	if claims.Provider != "" && !claims.Provider.Supported() {
		return identity.User{}, ports.ErrInvalidArgument
	}
	userID := identity.UserID(claims.Issuer, claims.Subject)
	if !validMutationAudit(event, audit.UserProvisioned, "user", userID, userID) || event.ActorUserID != userID || !validMutationAudit(profileEvent, audit.UserProfileSynchronized, "user", userID, userID) || profileEvent.ActorUserID != userID {
		return identity.User{}, ports.ErrInvalidArgument
	}
	var found identityModel
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("issuer = ? AND subject = ?", claims.Issuer, claims.Subject).First(&found)
		if result.Error == nil {
			profileEvent = auditForResolvedAccount(profileEvent, found.UserID)
			var existing userModel
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", found.UserID).First(&existing).Error; err != nil {
				return err
			}
			// Recheck the association after acquiring the same account lock used
			// by unlink/deletion. A stale lookup must not authenticate an identity
			// that was removed while this exchange waited for the lock.
			var current identityModel
			if err := tx.Where("issuer = ? AND subject = ? AND user_id = ?", claims.Issuer, claims.Subject, found.UserID).Take(&current).Error; err != nil {
				return err
			}
			if claims.Provider.Supported() && current.Provider.Supported() && claims.Provider != current.Provider {
				return ports.ErrInvalidCredential
			}
			providerChanged := claims.Provider.Supported() && current.Provider == ""
			if providerChanged {
				if err := tx.Model(&identityModel{}).Where("issuer = ? AND subject = ? AND user_id = ? AND provider = ''", claims.Issuer, claims.Subject, found.UserID).Update("provider", claims.Provider).Error; err != nil {
					return err
				}
			}
			normalizedEmail := existing.Email
			providerEmailVerified := existing.ProviderEmailVerified
			if incomingEmail := trustedProviderEmail(claims); incomingEmail != "" {
				normalizedEmail = incomingEmail
				providerEmailVerified = true
			}
			switch existing.Status {
			case identity.StatusProvisional:
				if !providerChanged && existing.Email == normalizedEmail && existing.ProviderEmailVerified == providerEmailVerified && existing.DisplayName == claims.DisplayName && existing.InvitationAdmin == claims.InvitationAdmin {
					return nil
				}
				updated := tx.Model(&userModel{}).Where("id = ? AND status = ?", found.UserID, identity.StatusProvisional).Updates(map[string]any{"email": normalizedEmail, "provider_email_verified": providerEmailVerified, "display_name": claims.DisplayName, "invitation_admin": claims.InvitationAdmin})
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected != 1 {
					return ports.ErrNotFound
				}
				return appendAuditEvent(tx, profileEvent)
			case identity.StatusActive:
				if !providerChanged && existing.Email == normalizedEmail && existing.ProviderEmailVerified == providerEmailVerified && existing.InvitationAdmin == claims.InvitationAdmin {
					return nil
				}
				updated := tx.Model(&userModel{}).Where("id = ? AND status = ?", found.UserID, identity.StatusActive).Updates(map[string]any{"email": normalizedEmail, "provider_email_verified": providerEmailVerified, "invitation_admin": claims.InvitationAdmin})
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected != 1 {
					return ports.ErrNotFound
				}
				return appendAuditEvent(tx, profileEvent)
			default:
				return ports.ErrNotFound
			}
		}
		if result.Error != gorm.ErrRecordNotFound {
			return result.Error
		}
		var claimedInvitation invitationModel
		if claims.EmailVerified {
			var err error
			claimedInvitation, err = activeInvitation(tx, normalizeEmail(claims.Email), event.OccurredAt)
			if err != nil && (!allowCreate || !errors.Is(err, ports.ErrNotFound)) {
				return err
			}
			if err == nil && (!validMutationAudit(invitationEvent, audit.InvitationConsumed, "invitation", normalizeEmail(claims.Email), userID) || invitationEvent.ActorUserID != userID) {
				return ports.ErrInvalidArgument
			}
		} else if !allowCreate {
			return ports.ErrNotFound
		}
		// Account identity must not be recoverable from a provider key after
		// deletion. Existing associations above retain their original IDs.
		userID = uuid.NewString()
		event = auditForResolvedAccount(event, userID)
		profileEvent = auditForResolvedAccount(profileEvent, userID)
		invitationEvent = auditForResolvedAccount(invitationEvent, userID)
		providerEmail := trustedProviderEmail(claims)
		u := userModel{ID: userID, Email: providerEmail, ProviderEmailVerified: providerEmail != "", DisplayName: claims.DisplayName, Status: identity.StatusProvisional, InvitationAdmin: claims.InvitationAdmin}
		createdUser := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&u)
		if createdUser.Error != nil {
			return createdUser.Error
		}
		if createdUser.RowsAffected != 1 {
			return ports.ErrConflict
		}
		candidate := identityModel{Issuer: claims.Issuer, Subject: claims.Subject, UserID: userID, Provider: claims.Provider}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			if err := appendAuditEvent(tx, event); err != nil {
				return err
			}
			if claimedInvitation.ID != "" {
				consumed := tx.Model(&invitationModel{}).Where("id = ? AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at > ?", claimedInvitation.ID, invitationEvent.OccurredAt).Updates(map[string]any{"consumed_by_user_id": userID, "consumed_at": invitationEvent.OccurredAt})
				if consumed.Error != nil {
					return consumed.Error
				}
				if consumed.RowsAffected != 1 {
					return ports.ErrNotFound
				}
				if err := appendAuditEvent(tx, invitationEvent); err != nil {
					return err
				}
			}
		} else {
			// Another exchange won the identity association. Remove our unused
			// provisional row in this transaction and return the winning account.
			if err := tx.Where("id = ?", userID).Delete(&userModel{}).Error; err != nil {
				return err
			}
		}
		return tx.Where("issuer = ? AND subject = ?", claims.Issuer, claims.Subject).First(&found).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.User{}, ports.ErrNotFound
		}
		return identity.User{}, err
	}
	var u userModel
	if err := s.DB.WithContext(ctx).First(&u, "id = ?", found.UserID).Error; err != nil {
		return identity.User{}, err
	}
	if u.Status != identity.StatusProvisional && u.Status != identity.StatusActive {
		return identity.User{}, ports.ErrNotFound
	}
	return identityUserFromModel(u), nil
}

func auditForResolvedAccount(event audit.Event, userID string) audit.Event {
	event.OwnerUserID, event.ActorUserID = userID, userID
	if event.TargetType == "user" {
		event.TargetID = userID
	}
	return event
}

func normalizeEmail(value string) string { return identity.NormalizeEmail(value) }

func trustedProviderEmail(claims ports.Claims) string {
	if !claims.EmailVerified {
		return ""
	}
	return normalizeEmail(claims.Email)
}
