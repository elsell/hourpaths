package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

const RenewPolicyAcceptanceOperation = "account.policies.accept"

type PolicyRenewalReview struct {
	Required    bool
	ReviewToken string
	Policies    OnboardingPolicySet
}
type PolicyRenewalInput struct {
	ReviewToken                                            string
	TermsAccepted, PrivacyAcknowledged, GuidelinesAccepted bool
}
type PolicyRenewalCommand struct {
	ActorUserID    string
	PolicyRevision int64
	Policies       identity.CurrentPolicyVersions
	AcceptedAt     time.Time
	Idempotency    ports.Idempotency
	Audit          audit.Event
}
type PolicyRenewalResult struct {
	Policies   identity.CurrentPolicyVersions
	AcceptedAt time.Time
	Replayed   bool
}
type PolicyAcceptanceRepository interface {
	HasAcceptedPolicies(context.Context, string, identity.CurrentPolicyVersions) (bool, error)
	RenewPolicyAcceptance(context.Context, PolicyRenewalCommand) (PolicyRenewalResult, error)
}

func (a App) ReviewCurrentPolicies(ctx context.Context, authorization string) (PolicyRenewalReview, error) {
	if a.Clock == nil || a.PolicyAuthority == nil || a.PolicyAcceptances == nil || a.Audits == nil {
		return PolicyRenewalReview{}, ports.ErrUnavailable
	}
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return PolicyRenewalReview{}, err
	}
	if user.Status != identity.StatusActive {
		return PolicyRenewalReview{}, ErrUnauthenticated
	}
	current, err := a.PolicyAuthority.Current(ctx)
	if err != nil {
		return PolicyRenewalReview{}, err
	}
	if ports.ValidatePolicySet(current) != nil {
		return PolicyRenewalReview{}, ports.ErrUnavailable
	}
	accepted, err := a.PolicyAcceptances.HasAcceptedPolicies(ctx, user.ID, currentPolicyVersions(current))
	if err != nil {
		return PolicyRenewalReview{}, err
	}
	token, err := shared.EncodePolicyRenewalReviewToken(a.CursorSigningKey, user.ID, current.Revision, currentPolicyVersions(current))
	if err != nil {
		return PolicyRenewalReview{}, err
	}
	if err = a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, user.ID, user.ID, audit.ResourceViewed, "policy_acceptance", user.ID, audit.Succeeded)); err != nil {
		return PolicyRenewalReview{}, err
	}
	return PolicyRenewalReview{Required: !accepted, ReviewToken: token, Policies: OnboardingPolicySet{
		TermsOfService:      OnboardingPolicyReference{Version: current.TermsVersion, URL: current.TermsURL},
		PrivacyPolicy:       OnboardingPolicyReference{Version: current.PrivacyPolicyVersion, URL: current.PrivacyPolicyURL},
		CommunityGuidelines: OnboardingPolicyReference{Version: current.CommunityGuidelinesVersion, URL: current.CommunityGuidelinesURL},
		SupportURL:          current.SupportURL,
	}}, nil
}

func (a App) AcceptCurrentPolicies(ctx context.Context, authorization, key string, input PolicyRenewalInput) (PolicyRenewalResult, error) {
	if a.Clock == nil || a.PolicyAcceptances == nil {
		return PolicyRenewalResult{}, ports.ErrUnavailable
	}
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return PolicyRenewalResult{}, err
	}
	if user.Status != identity.StatusActive {
		return PolicyRenewalResult{}, ErrUnauthenticated
	}
	if !validTimeZoneMutationKey(key) || !input.TermsAccepted || !input.PrivacyAcknowledged || !input.GuidelinesAccepted {
		return PolicyRenewalResult{}, ports.ErrInvalidArgument
	}
	reviewed, err := shared.DecodePolicyRenewalReviewToken(a.CursorSigningKey, input.ReviewToken)
	if err != nil || reviewed.Owner != user.ID {
		return PolicyRenewalResult{}, ports.ErrInvalidArgument
	}
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return PolicyRenewalResult{}, ports.ErrUnavailable
	}
	// Hash reviewed evidence, never a retry's new timestamp. The repository checks
	// a committed receipt before current publication so a lost response is safe.
	body, err := json.Marshal(reviewed)
	if err != nil {
		return PolicyRenewalResult{}, ports.ErrInvalidArgument
	}
	hash := sha256.Sum256(body)
	event := a.auditEvent(ctx, user.ID, user.ID, audit.ResourceUpdated, "policy_acceptance", user.ID, audit.Succeeded)
	event.OccurredAt = now
	result, err := a.PolicyAcceptances.RenewPolicyAcceptance(ctx, PolicyRenewalCommand{
		ActorUserID: user.ID, PolicyRevision: reviewed.PolicyRevision, Policies: reviewed.Policies, AcceptedAt: now,
		Idempotency: ports.Idempotency{PrincipalID: user.ID, Operation: RenewPolicyAcceptanceOperation, Key: key, RequestHash: hash[:]}, Audit: event,
	})
	if err != nil {
		return PolicyRenewalResult{}, err
	}
	if result.Policies != reviewed.Policies || result.AcceptedAt.IsZero() {
		return PolicyRenewalResult{}, ports.ErrUnavailable
	}
	return result, nil
}

// PolicyAdmission is applied by the transport to every protected operation,
// including domains whose services are composed independently of App.
type PolicyAdmission interface {
	AdmitPolicyUse(context.Context, string) error
}

func (a App) AdmitPolicyUse(ctx context.Context, authorization string) error {
	if a.Auth == nil || a.Users == nil || a.Clock == nil || a.PolicyAuthority == nil || a.PolicyAcceptances == nil || a.Audits == nil || a.AuditRateLimiter == nil {
		return ports.ErrUnavailable
	}
	user, err := a.authenticateCurrentUser(ctx, authorization)
	if err != nil {
		return err
	}
	if user.Status != identity.StatusActive {
		return ErrUnauthenticated
	}
	current, err := a.PolicyAuthority.Current(ctx)
	if err != nil {
		return err
	}
	if ports.ValidatePolicySet(current) != nil {
		return ports.ErrUnavailable
	}
	accepted, err := a.PolicyAcceptances.HasAcceptedPolicies(ctx, user.ID, currentPolicyVersions(current))
	if err != nil {
		return err
	}
	if accepted {
		return nil
	}
	// Successful admission is followed by the operation's existing audit and
	// principal limiter. Denial must itself be bounded and audited.
	if !a.AuditRateLimiter.Allow(user.ID, a.Clock.Now().UTC()) {
		return ErrRateLimited
	}
	if err = a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, user.ID, user.ID, audit.ResourceAccessDenied, "policy_acceptance", user.ID, audit.Denied)); err != nil {
		return err
	}
	return ports.ErrPolicyAcceptanceRequired
}
