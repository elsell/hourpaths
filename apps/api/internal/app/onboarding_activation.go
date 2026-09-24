package app

import (
	"context"
	"errors"
	"time"

	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type OnboardingActivationInput struct {
	Username, DisplayName       string
	ProfileVisibility           identity.ProfileVisibility
	TimeZone                    identity.IANATimeZone
	FirstDayOfWeek              identity.FirstDayOfWeek
	AtLeast16                   bool
	TermsAccepted               bool
	PrivacyAcknowledged         bool
	CommunityGuidelinesAccepted bool
	PolicyReviewToken           string
}

func (a App) ActivateOnboarding(ctx context.Context, authorization string, input OnboardingActivationInput) (Session, error) {
	principal, err := a.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return Session{}, err
	}
	if principal.UserID == "" || len(principal.Scopes) != 1 || principal.Scopes[0] != "api:onboarding" {
		return Session{}, ErrUnauthenticated
	}
	if a.PolicyAuthority == nil {
		return Session{}, errors.New("policy authority dependency is invalid")
	}
	policySet, err := a.PolicyAuthority.Current(ctx)
	if err != nil {
		return Session{}, err
	}
	if err := ports.ValidatePolicySet(policySet); err != nil {
		return Session{}, err
	}
	currentVersions := currentPolicyVersions(policySet)
	configuredPolicyAcceptance := identity.PolicyAcceptance{
		TermsOfServiceAcceptedVersion:      currentVersions.TermsOfService,
		PrivacyPolicyAcknowledgedVersion:   currentVersions.PrivacyPolicy,
		CommunityGuidelinesAcceptedVersion: currentVersions.CommunityGuidelines,
		AcceptedAt:                         time.Unix(1, 0).UTC(),
	}
	if err := identity.ValidatePolicyAcceptance(configuredPolicyAcceptance, currentVersions); err != nil {
		return Session{}, ports.ErrInvalidArgument
	}
	policyReview, err := shared.DecodePolicyReviewToken(a.CursorSigningKey, input.PolicyReviewToken)
	if err != nil || policyReview.Owner != principal.UserID {
		return Session{}, ports.ErrInvalidArgument
	}
	if policyReview.PolicyRevision != policySet.Revision || policyReview.Policies != currentVersions {
		return Session{}, ports.ErrPolicySetChanged
	}
	now := a.Clock.Now().UTC()
	activation := identity.OnboardingActivation{
		PolicySetRevision: policySet.Revision,
		UserID:            principal.UserID, Username: input.Username, DisplayName: input.DisplayName,
		ProfileVisibility: input.ProfileVisibility, TimeZone: input.TimeZone, FirstDayOfWeek: input.FirstDayOfWeek,
		PolicyAcceptance: identity.PolicyAcceptance{AcceptedAt: now},
	}
	if input.AtLeast16 {
		activation.AgeAttestation = identity.MinimumAgeAttestedAtLeast16
	}
	if input.TermsAccepted {
		activation.PolicyAcceptance.TermsOfServiceAcceptedVersion = currentVersions.TermsOfService
	}
	if input.PrivacyAcknowledged {
		activation.PolicyAcceptance.PrivacyPolicyAcknowledgedVersion = currentVersions.PrivacyPolicy
	}
	if input.CommunityGuidelinesAccepted {
		activation.PolicyAcceptance.CommunityGuidelinesAcceptedVersion = currentVersions.CommunityGuidelines
	}
	if err := identity.ValidateOnboardingActivation(activation, currentVersions); err != nil {
		return Session{}, ports.ErrInvalidArgument
	}
	if a.AuditRateLimiter == nil || !a.AuditRateLimiter.Allow(principal.UserID, now) {
		return Session{}, ErrRateLimited
	}
	if a.OnboardingActivator == nil || a.SessionTTL <= 0 {
		return Session{}, errors.New("onboarding activation dependencies are invalid")
	}
	correlation := correlationID(ctx)
	event := func(action audit.Action) audit.Event {
		return audit.Event{ID: newID(), OwnerUserID: principal.UserID, ActorUserID: principal.UserID, Action: action, TargetType: "user", TargetID: principal.UserID, Outcome: audit.Succeeded, CorrelationID: correlation, OccurredAt: now}
	}
	completed := event(audit.UserOnboardingCompleted)
	revoked := event(audit.SessionRevoked)
	created := event(audit.SessionCreated)
	token, expiresAt, err := a.OnboardingActivator.ActivateOnboarding(ctx, authorization, activation, now.Add(a.SessionTTL), completed, revoked, created)
	if err != nil {
		return Session{}, err
	}
	return Session{Token: token, ExpiresAt: expiresAt}, nil
}
