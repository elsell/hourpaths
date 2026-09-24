package app

import (
	"context"
	"errors"

	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type OnboardingProfile struct {
	Email              string
	DisplayName        string
	UsernameSuggestion string
	PolicyReviewToken  string
	Policies           OnboardingPolicySet
}

type OnboardingPolicyReference struct {
	Version, URL string
}

type OnboardingPolicySet struct {
	TermsOfService, PrivacyPolicy, CommunityGuidelines OnboardingPolicyReference
	SupportURL                                         string
}

func (a App) OnboardingProfile(ctx context.Context, authorization string) (OnboardingProfile, error) {
	principal, err := a.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return OnboardingProfile{}, err
	}
	if principal.UserID == "" {
		return OnboardingProfile{}, ErrUnauthenticated
	}
	if len(principal.Scopes) != 1 || principal.Scopes[0] != "api:onboarding" {
		if a.AuditRateLimiter == nil || !a.AuditRateLimiter.Allow(principal.UserID, a.Clock.Now().UTC()) {
			return OnboardingProfile{}, ErrRateLimited
		}
		if err := a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "user", principal.UserID, audit.Denied)); err != nil {
			return OnboardingProfile{}, err
		}
		return OnboardingProfile{}, ErrUnauthenticated
	}
	u, err := a.Users.GetProvisionalUser(ctx, principal.UserID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return OnboardingProfile{}, ErrUnauthenticated
		}
		return OnboardingProfile{}, err
	}
	if u.ID != principal.UserID || u.Status != identity.StatusProvisional {
		return OnboardingProfile{}, ErrUnauthenticated
	}
	if a.AuditRateLimiter == nil || !a.AuditRateLimiter.Allow(u.ID, a.Clock.Now().UTC()) {
		return OnboardingProfile{}, ErrRateLimited
	}
	if a.UsernameSuggestions == nil {
		return OnboardingProfile{}, errors.New("username suggestion dependency is invalid")
	}
	usernameSuggestion, err := a.UsernameSuggestions.SuggestAvailableUsername(ctx, u.DisplayName)
	if err != nil {
		return OnboardingProfile{}, err
	}
	if usernameSuggestion != "" {
		if err := identity.ValidateUsername(usernameSuggestion); err != nil {
			return OnboardingProfile{}, err
		}
	}
	if a.PolicyAuthority == nil {
		return OnboardingProfile{}, errors.New("policy authority dependency is invalid")
	}
	policySet, err := a.PolicyAuthority.Current(ctx)
	if err != nil {
		return OnboardingProfile{}, err
	}
	versions := currentPolicyVersions(policySet)
	if err := ports.ValidatePolicySet(policySet); err != nil {
		return OnboardingProfile{}, err
	}
	policyReviewToken, err := shared.EncodePolicyReviewToken(a.CursorSigningKey, u.ID, policySet.Revision, versions)
	if err != nil {
		return OnboardingProfile{}, err
	}
	if err := a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, u.ID, u.ID, audit.UserViewed, "user", u.ID, audit.Succeeded)); err != nil {
		return OnboardingProfile{}, err
	}
	return OnboardingProfile{
		Email: u.Email, DisplayName: u.DisplayName, UsernameSuggestion: usernameSuggestion, PolicyReviewToken: policyReviewToken,
		Policies: OnboardingPolicySet{
			TermsOfService:      OnboardingPolicyReference{Version: policySet.TermsVersion, URL: policySet.TermsURL},
			PrivacyPolicy:       OnboardingPolicyReference{Version: policySet.PrivacyPolicyVersion, URL: policySet.PrivacyPolicyURL},
			CommunityGuidelines: OnboardingPolicyReference{Version: policySet.CommunityGuidelinesVersion, URL: policySet.CommunityGuidelinesURL},
			SupportURL:          policySet.SupportURL,
		},
	}, nil
}

func currentPolicyVersions(policySet ports.PolicySet) identity.CurrentPolicyVersions {
	return identity.CurrentPolicyVersions{
		TermsOfService: policySet.TermsVersion, PrivacyPolicy: policySet.PrivacyPolicyVersion,
		CommunityGuidelines: policySet.CommunityGuidelinesVersion,
	}
}
