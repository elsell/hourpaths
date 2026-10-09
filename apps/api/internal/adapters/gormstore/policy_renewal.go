package gormstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type policyRenewalMutationModel struct {
	UserID, Operation, IdempotencyKey                              string
	RequestHash                                                    []byte
	PolicyRevision                                                 int64
	TermsVersion, PrivacyPolicyVersion, CommunityGuidelinesVersion string
	AcceptedAt                                                     time.Time
}

func (policyRenewalMutationModel) TableName() string { return "user_policy_renewal_mutation_models" }
func validRenewalVersions(v identity.CurrentPolicyVersions) bool {
	return identity.ValidatePolicyAcceptance(identity.PolicyAcceptance{TermsOfServiceAcceptedVersion: v.TermsOfService, PrivacyPolicyAcknowledgedVersion: v.PrivacyPolicy, CommunityGuidelinesAcceptedVersion: v.CommunityGuidelines, AcceptedAt: time.Unix(1, 0).UTC()}, v) == nil
}
func (s *Store) HasAcceptedPolicies(ctx context.Context, owner string, v identity.CurrentPolicyVersions) (bool, error) {
	if s == nil || s.DB == nil || owner == "" || !validRenewalVersions(v) {
		return false, ports.ErrInvalidArgument
	}
	var user struct{ ID string }
	err := s.DB.WithContext(ctx).Table("user_models").Select("id").Where("id = ? AND status = ?", owner, identity.StatusActive).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, ports.ErrNotFound
	}
	if err != nil {
		return false, classifyPolicyRenewalError(err)
	}
	var count int64
	err = s.DB.WithContext(ctx).Model(&userPolicyAcceptanceModel{}).Where("user_id = ?", owner).Where("(policy = ? AND version = ? AND acknowledgement = ?) OR (policy = ? AND version = ? AND acknowledgement = ?) OR (policy = ? AND version = ? AND acknowledgement = ?)", "terms", v.TermsOfService, "accepted", "privacy", v.PrivacyPolicy, "acknowledged", "community_guidelines", v.CommunityGuidelines, "accepted").Count(&count).Error
	return count == 3, classifyPolicyRenewalError(err)
}
func (s *Store) RenewPolicyAcceptance(ctx context.Context, c application.PolicyRenewalCommand) (application.PolicyRenewalResult, error) {
	if s == nil || s.DB == nil || !validPolicyRenewalCommand(c) {
		return application.PolicyRenewalResult{}, ports.ErrInvalidArgument
	}
	var result application.PolicyRenewalResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner struct{ ID string }
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("user_models").Select("id").Where("id = ? AND status = ?", c.ActorUserID, identity.StatusActive).Take(&owner).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrNotFound
		}
		if err != nil {
			return err
		}
		var receipt policyRenewalMutationModel
		err = tx.Where("user_id = ? AND operation = ? AND idempotency_key = ?", c.ActorUserID, c.Idempotency.Operation, c.Idempotency.Key).Take(&receipt).Error
		if err == nil {
			if !bytes.Equal(receipt.RequestHash, c.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = application.PolicyRenewalResult{Policies: identity.CurrentPolicyVersions{TermsOfService: receipt.TermsVersion, PrivacyPolicy: receipt.PrivacyPolicyVersion, CommunityGuidelines: receipt.CommunityGuidelinesVersion}, AcceptedAt: receipt.AcceptedAt, Replayed: true}
			if !validRenewalVersions(result.Policies) || result.AcceptedAt.IsZero() {
				return ports.ErrUnavailable
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Publication and acceptance serialize on the authority's existing lock.
		if err = tx.Exec("SELECT pg_advisory_xact_lock(?)", policyPublisherAdvisoryLock).Error; err != nil { // hourpaths-direct-sql: allow PostgreSQL transaction advisory lock
			return err
		}
		var current currentPolicySetModel
		if err = tx.Where("singleton = ?", true).Take(&current).Error; err != nil {
			return err
		}
		authority := policySetFromModel(current)
		if ports.ValidatePolicySet(authority) != nil {
			return ports.ErrUnavailable
		}
		if current.Revision != c.PolicyRevision || current.TermsVersion != c.Policies.TermsOfService || current.PrivacyPolicyVersion != c.Policies.PrivacyPolicy || current.CommunityGuidelinesVersion != c.Policies.CommunityGuidelines {
			return ports.ErrPolicySetChanged
		}
		evidence := []userPolicyAcceptanceModel{
			{UserID: c.ActorUserID, Policy: "terms", Version: c.Policies.TermsOfService, Acknowledgement: "accepted", AcceptedAt: c.AcceptedAt},
			{UserID: c.ActorUserID, Policy: "privacy", Version: c.Policies.PrivacyPolicy, Acknowledgement: "acknowledged", AcceptedAt: c.AcceptedAt},
			{UserID: c.ActorUserID, Policy: "community_guidelines", Version: c.Policies.CommunityGuidelines, Acknowledgement: "accepted", AcceptedAt: c.AcceptedAt},
		}
		if err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&evidence).Error; err != nil {
			return err
		}
		receipt = policyRenewalMutationModel{UserID: c.ActorUserID, Operation: c.Idempotency.Operation, IdempotencyKey: c.Idempotency.Key, RequestHash: append([]byte(nil), c.Idempotency.RequestHash...), PolicyRevision: c.PolicyRevision, TermsVersion: c.Policies.TermsOfService, PrivacyPolicyVersion: c.Policies.PrivacyPolicy, CommunityGuidelinesVersion: c.Policies.CommunityGuidelines, AcceptedAt: c.AcceptedAt}
		if err = tx.Create(&receipt).Error; err != nil {
			return err
		}
		if err = appendAuditEvent(tx, c.Audit); err != nil {
			return err
		}
		result = application.PolicyRenewalResult{Policies: c.Policies, AcceptedAt: c.AcceptedAt}
		return nil
	})
	return result, classifyPolicyRenewalError(err)
}
func validPolicyRenewalCommand(c application.PolicyRenewalCommand) bool {
	return c.ActorUserID != "" && c.PolicyRevision > 0 && validRenewalVersions(c.Policies) && !c.AcceptedAt.IsZero() && c.AcceptedAt.Location() == time.UTC && c.AcceptedAt.Equal(c.AcceptedAt.Truncate(time.Microsecond)) && c.Idempotency.PrincipalID == c.ActorUserID && c.Idempotency.Operation == application.RenewPolicyAcceptanceOperation && len(c.Idempotency.Key) >= 16 && len(c.Idempotency.Key) <= 128 && len(c.Idempotency.RequestHash) == 32 && c.Audit.Valid() && c.Audit.OwnerUserID == c.ActorUserID && c.Audit.ActorUserID == c.ActorUserID && c.Audit.TargetID == c.ActorUserID && c.Audit.TargetType == "policy_acceptance" && c.Audit.Action == audit.ResourceUpdated && c.Audit.Outcome == audit.Succeeded && c.Audit.OccurredAt == c.AcceptedAt
}
func classifyPolicyRenewalError(err error) error {
	switch {
	case err == nil, errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrIdempotencyConflict), errors.Is(err, ports.ErrPolicySetChanged), errors.Is(err, ports.ErrUnavailable):
		return err
	default:
		return fmt.Errorf("policy acceptance persistence: %w: %v", ports.ErrUnavailable, err)
	}
}
