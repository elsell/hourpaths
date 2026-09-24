package gormstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

const policyPublisherAdvisoryLock int64 = 0x48504f4c494359

type currentPolicySetModel struct {
	Singleton                  bool `gorm:"primaryKey"`
	Revision                   int64
	TermsVersion               string
	PrivacyPolicyVersion       string
	CommunityGuidelinesVersion string
	TermsURL                   string
	PrivacyPolicyURL           string
	CommunityGuidelinesURL     string
	SupportURL                 string
	UpdatedAt                  time.Time
}

// PolicyAuthorityHealth keeps API replicas out of readiness until policy
// publication has completed and the shared authority is valid.
type PolicyAuthorityHealth struct{ Authority ports.PolicyAuthority }

func (health PolicyAuthorityHealth) Health(ctx context.Context) error {
	if health.Authority == nil {
		return errors.New("policy authority health dependency is missing")
	}
	policySet, err := health.Authority.Current(ctx)
	if err != nil {
		return fmt.Errorf("current policy authority is unavailable: %w", err)
	}
	if err := ports.ValidatePolicySet(policySet); err != nil {
		return fmt.Errorf("current policy authority is invalid: %w", err)
	}
	return nil
}

func (currentPolicySetModel) TableName() string { return "current_policy_set_models" }

// Current returns only a complete, valid singleton. A missing or malformed
// authority fails closed so onboarding cannot present invented policy values.
func (s *Store) Current(ctx context.Context) (ports.PolicySet, error) {
	var row currentPolicySetModel
	err := s.DB.WithContext(ctx).Where("singleton = ?", true).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.PolicySet{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.PolicySet{}, err
	}
	value := policySetFromModel(row)
	if err := ports.ValidatePolicySet(value); err != nil {
		return ports.PolicySet{}, fmt.Errorf("stored policy authority is invalid: %w", err)
	}
	return value, nil
}

// Publish serializes every publisher, including the initially empty authority,
// and makes a revision's complete policy set visible in one transaction.
func (s *Store) Publish(ctx context.Context, requested ports.PolicySet) (ports.PolicySet, error) {
	if err := ports.ValidatePolicySet(requested); err != nil {
		return ports.PolicySet{}, err
	}
	var shared ports.PolicySet
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked := tx.Exec("SELECT pg_advisory_xact_lock(?)", policyPublisherAdvisoryLock) // hourpaths-direct-sql: allow PostgreSQL transaction advisory lock
		if err := locked.Error; err != nil {
			return err
		}
		var row currentPolicySetModel
		err := tx.Where("singleton = ?", true).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = policySetModel(requested)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			shared = requested
			return nil
		}
		if err != nil {
			return err
		}
		current := policySetFromModel(row)
		if err := ports.ValidatePolicySet(current); err != nil {
			return fmt.Errorf("stored policy authority is invalid: %w", err)
		}
		shared = current
		switch {
		case requested.Revision < current.Revision:
			return nil
		case requested.Revision == current.Revision:
			retry := requested
			retry.UpdatedAt = current.UpdatedAt
			if ports.PolicySetsEqual(retry, current) {
				return nil
			}
			return ports.ErrConflict
		default:
			row = policySetModel(requested)
			result := tx.Model(&currentPolicySetModel{}).Where("singleton = ?", true).Updates(map[string]any{
				"revision":                     requested.Revision,
				"terms_version":                requested.TermsVersion,
				"privacy_policy_version":       requested.PrivacyPolicyVersion,
				"community_guidelines_version": requested.CommunityGuidelinesVersion,
				"terms_url":                    requested.TermsURL,
				"privacy_policy_url":           requested.PrivacyPolicyURL,
				"community_guidelines_url":     requested.CommunityGuidelinesURL,
				"support_url":                  requested.SupportURL,
				"updated_at":                   requested.UpdatedAt,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("policy authority singleton disappeared during publication")
			}
			shared = requested
			return nil
		}
	})
	return shared, err
}

func policySetModel(value ports.PolicySet) currentPolicySetModel {
	return currentPolicySetModel{
		Singleton: true, Revision: value.Revision, TermsVersion: value.TermsVersion,
		PrivacyPolicyVersion: value.PrivacyPolicyVersion, CommunityGuidelinesVersion: value.CommunityGuidelinesVersion,
		TermsURL: value.TermsURL, PrivacyPolicyURL: value.PrivacyPolicyURL,
		CommunityGuidelinesURL: value.CommunityGuidelinesURL, SupportURL: value.SupportURL, UpdatedAt: value.UpdatedAt,
	}
}

func policySetFromModel(value currentPolicySetModel) ports.PolicySet {
	return ports.PolicySet{
		Revision: value.Revision, TermsVersion: value.TermsVersion,
		PrivacyPolicyVersion: value.PrivacyPolicyVersion, CommunityGuidelinesVersion: value.CommunityGuidelinesVersion,
		TermsURL: value.TermsURL, PrivacyPolicyURL: value.PrivacyPolicyURL,
		CommunityGuidelinesURL: value.CommunityGuidelinesURL, SupportURL: value.SupportURL, UpdatedAt: value.UpdatedAt,
	}
}

var _ ports.PolicyAuthority = (*Store)(nil)
var _ ports.PolicyPublisher = (*Store)(nil)
var _ ports.HealthChecker = PolicyAuthorityHealth{}
