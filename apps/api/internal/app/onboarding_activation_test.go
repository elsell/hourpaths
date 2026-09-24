package app

import (
	"context"
	"errors"
	"testing"
	"time"

	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledOnboardingActivator struct {
	authorization string
	activation    identity.OnboardingActivation
	expiresAt     time.Time
	events        []audit.Event
	token         string
	actualExpiry  time.Time
	err           error
}

type advancingActivationClock struct {
	now   time.Time
	calls int
}

func (c *advancingActivationClock) Now() time.Time {
	value := c.now.Add(time.Duration(c.calls) * time.Millisecond)
	c.calls++
	return value
}

func (a *controlledOnboardingActivator) ActivateOnboarding(_ context.Context, authorization string, activation identity.OnboardingActivation, expiresAt time.Time, completed, revoked, created audit.Event) (string, time.Time, error) {
	a.authorization = authorization
	a.activation = activation
	a.expiresAt = expiresAt
	a.events = []audit.Event{completed, revoked, created}
	return a.token, a.actualExpiry, a.err
}

func validActivation(now time.Time) (OnboardingActivationInput, identity.OnboardingActivation, identity.CurrentPolicyVersions) {
	versions := identity.CurrentPolicyVersions{TermsOfService: "terms-v1", PrivacyPolicy: "privacy-v1", CommunityGuidelines: "community-v1"}
	input := OnboardingActivationInput{
		Username: "practice.time", DisplayName: "Practice Time", ProfileVisibility: identity.ProfileVisibilityPrivate,
		TimeZone: "America/New_York", FirstDayOfWeek: identity.FirstDayMonday,
		AtLeast16: true, TermsAccepted: true, PrivacyAcknowledged: true, CommunityGuidelinesAccepted: true,
	}
	activation := identity.OnboardingActivation{
		PolicySetRevision: 7,
		UserID:            "provisional-owner", Username: "practice.time", DisplayName: "Practice Time",
		ProfileVisibility: identity.ProfileVisibilityPrivate, TimeZone: "America/New_York", FirstDayOfWeek: identity.FirstDayMonday,
		AgeAttestation:   identity.MinimumAgeAttestedAtLeast16,
		PolicyAcceptance: identity.PolicyAcceptance{TermsOfServiceAcceptedVersion: versions.TermsOfService, PrivacyPolicyAcknowledgedVersion: versions.PrivacyPolicy, CommunityGuidelinesAcceptedVersion: versions.CommunityGuidelines, AcceptedAt: now},
	}
	return input, activation, versions
}

func signPolicyReview(t *testing.T, owner string, revision int64, versions identity.CurrentPolicyVersions) string {
	t.Helper()
	token, err := shared.EncodePolicyReviewToken([]byte("0123456789abcdef0123456789abcdef"), owner, revision, versions)
	if err != nil {
		t.Fatalf("sign policy review: %v", err)
	}
	return token
}

func TestActivateOnboardingValidatesTheExactOwnerAndAtomicallyRotatesToApplicationScope(t *testing.T) {
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	input, activation, versions := validActivation(now)
	input.PolicyReviewToken = signPolicyReview(t, activation.UserID, 7, versions)
	actualExpiry := now.Add(45 * time.Minute)
	activator := &controlledOnboardingActivator{token: "application-session", actualExpiry: actualExpiry}
	clock := &advancingActivationClock{now: now}
	a := App{
		Auth:                fakeAuth{principal: ports.Principal{UserID: activation.UserID, Scopes: []string{"api:onboarding"}}},
		OnboardingActivator: activator, PolicyAuthority: staticPolicyAuthority{policySet: testPolicySet(versions)}, SessionTTL: time.Hour,
		AuditRateLimiter: fakeAuditRateLimiter{}, Clock: clock,
		CursorSigningKey: []byte("0123456789abcdef0123456789abcdef"),
	}

	session, err := a.ActivateOnboarding(context.Background(), "Bearer onboarding-session", input)
	if err != nil {
		t.Fatalf("activate onboarding: %v", err)
	}
	if session.Token != "application-session" || !session.ExpiresAt.Equal(actualExpiry) {
		t.Fatalf("activation session = %+v", session)
	}
	if activator.authorization != "Bearer onboarding-session" || activator.activation != activation || !activator.expiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("activation boundary = auth %q activation %+v expiry %v", activator.authorization, activator.activation, activator.expiresAt)
	}
	wantActions := []audit.Action{audit.UserOnboardingCompleted, audit.SessionRevoked, audit.SessionCreated}
	if len(activator.events) != len(wantActions) {
		t.Fatalf("activation events = %+v", activator.events)
	}
	for i, event := range activator.events {
		if event.Action != wantActions[i] || event.OwnerUserID != activation.UserID || event.ActorUserID != activation.UserID || event.TargetType != "user" || event.TargetID != activation.UserID || event.Outcome != audit.Succeeded || !event.OccurredAt.Equal(now) {
			t.Fatalf("activation event %d = %+v", i, event)
		}
	}
	if clock.calls != 1 {
		t.Fatalf("one logical activation sampled the application clock %d times", clock.calls)
	}
}

func TestActivateOnboardingFailsClosedBeforePersistence(t *testing.T) {
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	valid, _, versions := validActivation(now)
	valid.PolicyReviewToken = signPolicyReview(t, "provisional-owner", 7, versions)
	missingPolicyConfig := testPolicySet(versions)
	missingPolicyConfig.TermsVersion = ""

	tests := []struct {
		name      string
		principal ports.Principal
		input     OnboardingActivationInput
		policySet ports.PolicySet
		denied    bool
		want      error
	}{
		{name: "application scope", principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:user"}}, input: valid, policySet: testPolicySet(versions), want: ErrUnauthenticated},
		{name: "mixed scopes", principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding", "api:user"}}, input: valid, policySet: testPolicySet(versions), want: ErrUnauthenticated},
		{name: "missing server policy version", principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}, input: valid, policySet: missingPolicyConfig, want: ports.ErrInvalidArgument},
		{name: "rate limited", principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}, input: valid, policySet: testPolicySet(versions), denied: true, want: ErrRateLimited},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			activator := &controlledOnboardingActivator{}
			a := App{Auth: fakeAuth{principal: test.principal}, OnboardingActivator: activator, PolicyAuthority: staticPolicyAuthority{policySet: test.policySet}, SessionTTL: time.Hour, AuditRateLimiter: fakeAuditRateLimiter{denied: test.denied}, Clock: fakeClock{now: now}, CursorSigningKey: []byte("0123456789abcdef0123456789abcdef")}
			_, err := a.ActivateOnboarding(context.Background(), "Bearer onboarding-session", test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("activation error = %v, want %v", err, test.want)
			}
			if activator.authorization != "" {
				t.Fatalf("rejected activation reached persistence with %q", activator.authorization)
			}
		})
	}

	for name, mutate := range map[string]func(*OnboardingActivationInput){
		"missing age affirmation":         func(value *OnboardingActivationInput) { value.AtLeast16 = false },
		"missing terms acceptance":        func(value *OnboardingActivationInput) { value.TermsAccepted = false },
		"missing privacy acknowledgement": func(value *OnboardingActivationInput) { value.PrivacyAcknowledged = false },
		"missing guidelines acceptance":   func(value *OnboardingActivationInput) { value.CommunityGuidelinesAccepted = false },
	} {
		t.Run(name, func(t *testing.T) {
			input := valid
			mutate(&input)
			activator := &controlledOnboardingActivator{}
			a := App{Auth: fakeAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}}, OnboardingActivator: activator, PolicyAuthority: staticPolicyAuthority{policySet: testPolicySet(versions)}, SessionTTL: time.Hour, AuditRateLimiter: fakeAuditRateLimiter{}, Clock: fakeClock{now: now}, CursorSigningKey: []byte("0123456789abcdef0123456789abcdef")}
			if _, err := a.ActivateOnboarding(context.Background(), "Bearer onboarding-session", input); !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("missing affirmation error = %v", err)
			}
			if activator.authorization != "" {
				t.Fatal("missing affirmation reached persistence")
			}
		})
	}
}

func TestActivateOnboardingPreservesCredentialConflictAndPersistenceErrors(t *testing.T) {
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	input, activation, versions := validActivation(now)
	input.PolicyReviewToken = signPolicyReview(t, activation.UserID, 7, versions)
	for _, dependencyErr := range []error{ports.ErrInvalidCredential, ports.ErrUsernameUnavailable, ports.ErrPolicySetChanged, errors.New("database unavailable")} {
		a := App{
			Auth:                fakeAuth{principal: ports.Principal{UserID: activation.UserID, Scopes: []string{"api:onboarding"}}},
			OnboardingActivator: &controlledOnboardingActivator{err: dependencyErr}, PolicyAuthority: staticPolicyAuthority{policySet: testPolicySet(versions)}, SessionTTL: time.Hour,
			AuditRateLimiter: fakeAuditRateLimiter{}, Clock: fakeClock{now: now},
			CursorSigningKey: []byte("0123456789abcdef0123456789abcdef"),
		}
		if _, err := a.ActivateOnboarding(context.Background(), "Bearer onboarding-session", input); !errors.Is(err, dependencyErr) {
			t.Fatalf("activation error = %v, want %v", err, dependencyErr)
		}
	}
}

func TestActivateOnboardingRejectsInvalidOwnerAndChangedPolicyReviewProofBeforePersistence(t *testing.T) {
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	valid, _, versions := validActivation(now)
	key := []byte("0123456789abcdef0123456789abcdef")
	prior := versions
	prior.TermsOfService = "terms-prior"

	for _, test := range []struct {
		name  string
		token string
		want  error
	}{
		{name: "missing", want: ports.ErrInvalidArgument},
		{name: "malformed", token: "malformed", want: ports.ErrInvalidArgument},
		{name: "wrong owner", token: signPolicyReview(t, "another-owner", 7, versions), want: ports.ErrInvalidArgument},
		{name: "changed policy set", token: signPolicyReview(t, "provisional-owner", 7, prior), want: ports.ErrPolicySetChanged},
		{name: "changed policy revision", token: signPolicyReview(t, "provisional-owner", 6, versions), want: ports.ErrPolicySetChanged},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			input.PolicyReviewToken = test.token
			activator := &controlledOnboardingActivator{}
			a := App{Auth: fakeAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}}, OnboardingActivator: activator, PolicyAuthority: staticPolicyAuthority{policySet: testPolicySet(versions)}, SessionTTL: time.Hour, AuditRateLimiter: fakeAuditRateLimiter{}, Clock: fakeClock{now: now}, CursorSigningKey: key}
			if _, err := a.ActivateOnboarding(context.Background(), "Bearer onboarding-session", input); !errors.Is(err, test.want) {
				t.Fatalf("activation error = %v, want %v", err, test.want)
			}
			if activator.authorization != "" {
				t.Fatal("invalid policy review proof reached persistence")
			}
		})
	}
}
