package gormstore

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userAccountActivationModel struct {
	UserID             string `gorm:"primaryKey"`
	PolicySetRevision  int64
	MinimumAgeAttested int16
	AgeAttestedAt      time.Time
	CompletedAt        time.Time
}

type userPolicyAcceptanceModel struct {
	UserID          string `gorm:"primaryKey"`
	Policy          string `gorm:"primaryKey"`
	Version         string `gorm:"primaryKey"`
	Acknowledgement string
	AcceptedAt      time.Time
}

type userPreferenceModel struct {
	UserID          string `gorm:"primaryKey"`
	FirstDayOfWeek  int16
	CurrentTimeZone string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type userTimeZoneHistoryModel struct {
	UserID      string    `gorm:"primaryKey"`
	EffectiveAt time.Time `gorm:"primaryKey"`
	TimeZone    string
}

var _ ports.OnboardingActivationRepository = (*Store)(nil)

// ActivateOnboarding commits the lifecycle transition and its credential
// rotation as one transaction. The user row is always locked before the
// presented session row so concurrent activation attempts use one lock order.
func (s *Store) ActivateOnboarding(
	ctx context.Context,
	activation identity.OnboardingActivation,
	oldHash []byte,
	now time.Time,
	newRecord ports.SessionRecord,
	completed audit.Event,
	revoked audit.Event,
	created audit.Event,
) (time.Time, error) {
	if err := validateActivationPersistenceInput(activation, oldHash, now, newRecord, completed, revoked, created); err != nil {
		return time.Time{}, err
	}

	actualExpiresAt := newRecord.ExpiresAt
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userModel
		if err := tx.Select("id", "status").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", activation.UserID).Take(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if user.Status != identity.StatusProvisional {
			return ports.ErrNotFound
		}

		var old sessionModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"token_hash = ? AND user_id = ? AND scopes = ? AND revoked_at IS NULL AND expires_at > ? AND absolute_expires_at > ?",
			oldHash, activation.UserID, "api:onboarding", now, now,
		).Take(&old).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if actualExpiresAt.After(old.AbsoluteExpiresAt) {
			actualExpiresAt = old.AbsoluteExpiresAt
		}

		acceptance := activation.PolicyAcceptance
		// Serialize against policy publication without taking a row lock. PostgreSQL
		// requires UPDATE privilege for SELECT FOR SHARE, while the runtime role is
		// intentionally limited to SELECT on the shared policy authority.
		if err := tx.Exec("SELECT pg_advisory_xact_lock_shared(?)", policyPublisherAdvisoryLock).Error; err != nil { // hourpaths-direct-sql: allow PostgreSQL transaction advisory lock
			return err
		}
		var authority currentPolicySetModel
		if err := tx.Select("revision").Where(
			"singleton = ? AND revision = ? AND terms_version = ? AND privacy_policy_version = ? AND community_guidelines_version = ?",
			true, activation.PolicySetRevision, acceptance.TermsOfServiceAcceptedVersion,
			acceptance.PrivacyPolicyAcknowledgedVersion, acceptance.CommunityGuidelinesAcceptedVersion,
		).Take(&authority).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrPolicySetChanged
			}
			return err
		}

		if err := tx.Create(&userAccountActivationModel{
			UserID: activation.UserID, PolicySetRevision: activation.PolicySetRevision,
			MinimumAgeAttested: 16, AgeAttestedAt: now, CompletedAt: now,
		}).Error; err != nil {
			return err
		}
		policyRows := []userPolicyAcceptanceModel{
			{UserID: activation.UserID, Policy: "terms", Version: acceptance.TermsOfServiceAcceptedVersion, Acknowledgement: "accepted", AcceptedAt: acceptance.AcceptedAt},
			{UserID: activation.UserID, Policy: "privacy", Version: acceptance.PrivacyPolicyAcknowledgedVersion, Acknowledgement: "acknowledged", AcceptedAt: acceptance.AcceptedAt},
			{UserID: activation.UserID, Policy: "community_guidelines", Version: acceptance.CommunityGuidelinesAcceptedVersion, Acknowledgement: "accepted", AcceptedAt: acceptance.AcceptedAt},
		}
		if err := tx.Create(&policyRows).Error; err != nil {
			return err
		}
		if err := tx.Create(&userPreferenceModel{
			UserID: activation.UserID, FirstDayOfWeek: int16(activation.FirstDayOfWeek), CurrentTimeZone: string(activation.TimeZone), CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&userTimeZoneHistoryModel{UserID: activation.UserID, EffectiveAt: now, TimeZone: string(activation.TimeZone)}).Error; err != nil {
			return err
		}

		username := activation.Username
		visibility := activation.ProfileVisibility
		updated := tx.Model(&userModel{}).Where("id = ? AND status = ?", activation.UserID, identity.StatusProvisional).Updates(map[string]any{
			"username": username, "display_name": activation.DisplayName, "profile_visibility": visibility,
			"status": identity.StatusActive, "updated_at": now,
		})
		if updated.Error != nil {
			if errors.Is(updated.Error, gorm.ErrDuplicatedKey) {
				return ports.ErrUsernameUnavailable
			}
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrNotFound
		}

		revokedSessions := tx.Model(&sessionModel{}).Where("user_id = ? AND scopes = ? AND revoked_at IS NULL", activation.UserID, "api:onboarding").Update("revoked_at", now)
		if revokedSessions.Error != nil {
			return revokedSessions.Error
		}
		if revokedSessions.RowsAffected < 1 {
			return ports.ErrNotFound
		}
		if err := tx.Create(&sessionModel{
			TokenHash: append([]byte(nil), newRecord.TokenHash...), UserID: activation.UserID, Scopes: "api:user",
			ExpiresAt: actualExpiresAt, AbsoluteExpiresAt: old.AbsoluteExpiresAt, CreatedAt: now,
		}).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ports.ErrInvalidCredential
			}
			return err
		}
		for _, event := range []audit.Event{completed, revoked, created} {
			if err := appendAuditEvent(tx, event); err != nil {
				return err
			}
		}
		return nil
	})
	return actualExpiresAt, err
}

func validateActivationPersistenceInput(
	activation identity.OnboardingActivation,
	oldHash []byte,
	now time.Time,
	newRecord ports.SessionRecord,
	completed audit.Event,
	revoked audit.Event,
	created audit.Event,
) error {
	acceptance := activation.PolicyAcceptance
	versions := identity.CurrentPolicyVersions{
		TermsOfService: acceptance.TermsOfServiceAcceptedVersion, PrivacyPolicy: acceptance.PrivacyPolicyAcknowledgedVersion,
		CommunityGuidelines: acceptance.CommunityGuidelinesAcceptedVersion,
	}
	if err := identity.ValidateOnboardingActivation(activation, versions); err != nil {
		return ports.ErrInvalidArgument
	}
	if len(oldHash) != 32 || now.IsZero() || acceptance.AcceptedAt.After(now) ||
		len(newRecord.TokenHash) != 32 || string(newRecord.TokenHash) == string(oldHash) || len(newRecord.IdentityTokenHash) != 0 ||
		newRecord.UserID != activation.UserID || len(newRecord.Scopes) != 1 || newRecord.Scopes[0] != "api:user" || !newRecord.ExpiresAt.After(now) || !newRecord.AbsoluteExpiresAt.IsZero() {
		return ports.ErrInvalidArgument
	}
	if !validActivationAudit(completed, audit.UserOnboardingCompleted, activation.UserID) ||
		!validActivationAudit(revoked, audit.SessionRevoked, activation.UserID) ||
		!validActivationAudit(created, audit.SessionCreated, activation.UserID) ||
		completed.OccurredAt.After(now) ||
		!completed.OccurredAt.Equal(revoked.OccurredAt) || !completed.OccurredAt.Equal(created.OccurredAt) ||
		completed.CorrelationID != revoked.CorrelationID || completed.CorrelationID != created.CorrelationID {
		return ports.ErrInvalidArgument
	}
	return nil
}

func validActivationAudit(event audit.Event, action audit.Action, userID string) bool {
	return validMutationAudit(event, action, "user", userID, userID) && event.ActorUserID == userID && strings.TrimSpace(event.CorrelationID) != ""
}
