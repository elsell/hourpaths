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

type controlledUsernameSuggestions struct {
	displayName string
	suggestion  string
	err         error
}

type staticPolicyAuthority struct {
	policySet ports.PolicySet
	err       error
}

func (a staticPolicyAuthority) Current(context.Context) (ports.PolicySet, error) {
	return a.policySet, a.err
}

func testPolicySet(versions identity.CurrentPolicyVersions) ports.PolicySet {
	return ports.PolicySet{
		Revision: 7, TermsVersion: versions.TermsOfService, PrivacyPolicyVersion: versions.PrivacyPolicy,
		CommunityGuidelinesVersion: versions.CommunityGuidelines,
		TermsURL:                   "https://app.example/legal/terms", PrivacyPolicyURL: "https://app.example/legal/privacy",
		CommunityGuidelinesURL: "https://app.example/community-guidelines", SupportURL: "https://app.example/support",
		UpdatedAt: time.Date(2026, 7, 21, 15, 0, 0, 0, time.UTC),
	}
}

func (s *controlledUsernameSuggestions) SuggestAvailableUsername(_ context.Context, displayName string) (string, error) {
	s.displayName = displayName
	return s.suggestion, s.err
}

func TestOnboardingProfileReturnsOnlyTheAuthenticatedProvisionalUsersProviderSeeds(t *testing.T) {
	now := time.Date(2026, 7, 21, 16, 0, 0, 0, time.UTC)
	var requestedUserID string
	var events []audit.Event
	suggestions := &controlledUsernameSuggestions{suggestion: "provider.seed"}
	versions := identity.CurrentPolicyVersions{TermsOfService: "terms-v1", PrivacyPolicy: "privacy-v1", CommunityGuidelines: "guidelines-v1"}
	policySet := testPolicySet(versions)
	application := App{
		Auth: fakeAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
		Users: fakeUsers{
			provisional:   identity.User{ID: "provisional-owner", Email: "private@example.com", DisplayName: "Provider Seed", Status: identity.StatusProvisional, InvitationAdmin: true},
			provisionalID: &requestedUserID,
		},
		Audits:              fakeAudits{events: &events},
		AuditRateLimiter:    fakeAuditRateLimiter{},
		Clock:               fakeClock{now: now},
		UsernameSuggestions: suggestions,
		PolicyAuthority:     staticPolicyAuthority{policySet: policySet},
		CursorSigningKey:    []byte("0123456789abcdef0123456789abcdef"),
	}

	profile, err := application.OnboardingProfile(context.Background(), "Bearer onboarding-session")
	if err != nil {
		t.Fatalf("read onboarding profile: %v", err)
	}
	if requestedUserID != "provisional-owner" {
		t.Fatalf("provisional lookup user ID = %q, want authenticated owner", requestedUserID)
	}
	if profile.Email != "private@example.com" || profile.DisplayName != "Provider Seed" || profile.UsernameSuggestion != "provider.seed" {
		t.Fatalf("onboarding provider seeds = %+v", profile)
	}
	if profile.Policies.TermsOfService.Version != "terms-v1" || profile.Policies.TermsOfService.URL != "https://app.example/legal/terms" || profile.Policies.PrivacyPolicy.Version != "privacy-v1" || profile.Policies.CommunityGuidelines.Version != "guidelines-v1" || profile.Policies.SupportURL != "https://app.example/support" {
		t.Fatalf("onboarding policy metadata = %+v", profile.Policies)
	}
	payload, err := shared.DecodePolicyReviewToken(application.CursorSigningKey, profile.PolicyReviewToken)
	if err != nil || payload.Owner != "provisional-owner" || payload.PolicyRevision != policySet.Revision || payload.Policies != versions {
		t.Fatalf("policy review proof = %+v, %v", payload, err)
	}
	if suggestions.displayName != "Provider Seed" {
		t.Fatalf("username suggestion source = %q, want provider display name", suggestions.displayName)
	}
	if len(events) != 1 || events[0].Action != audit.UserViewed || events[0].OwnerUserID != "provisional-owner" || events[0].ActorUserID != "provisional-owner" || events[0].TargetID != "provisional-owner" {
		t.Fatalf("onboarding profile read audit = %+v", events)
	}
}

func TestOnboardingProfileFailsClosedOutsideTheExactProvisionalSessionBoundary(t *testing.T) {
	for _, test := range []struct {
		name      string
		principal ports.Principal
		userErr   error
	}{
		{name: "ordinary application session", principal: ports.Principal{UserID: "active-user", Scopes: []string{"api:user"}}},
		{name: "missing scope", principal: ports.Principal{UserID: "provisional-owner"}},
		{name: "mixed scopes", principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding", "api:user"}}},
		{name: "unknown provisional user", principal: ports.Principal{UserID: "missing-user", Scopes: []string{"api:onboarding"}}, userErr: ports.ErrNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requestedUserID string
			var events []audit.Event
			application := App{
				Auth:             fakeAuth{principal: test.principal},
				Users:            fakeUsers{provisionalErr: test.userErr, provisionalID: &requestedUserID},
				Audits:           fakeAudits{events: &events},
				AuditRateLimiter: fakeAuditRateLimiter{},
				Clock:            fakeClock{now: time.Now().UTC()},
			}

			if _, err := application.OnboardingProfile(context.Background(), "Bearer session"); !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("onboarding profile error = %v, want ErrUnauthenticated", err)
			}
			if test.userErr == nil && requestedUserID != "" {
				t.Fatalf("rejected session reached provisional lookup for %q", requestedUserID)
			}
			if test.userErr == nil && (len(events) != 1 || events[0].Action != audit.ResourceAccessDenied || events[0].Outcome != audit.Denied || events[0].OwnerUserID != test.principal.UserID || events[0].ActorUserID != test.principal.UserID) {
				t.Fatalf("onboarding denial audit = %+v", events)
			}
		})
	}
}

func TestOnboardingProfilePreservesCredentialAndPersistenceFailureClassification(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	application := App{Auth: fakeAuth{err: ports.ErrInvalidCredential}}
	if _, err := application.OnboardingProfile(context.Background(), "Bearer malformed"); !errors.Is(err, ports.ErrInvalidCredential) || errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("invalid credential classification was lost: %v", err)
	}

	application = App{
		Auth:             fakeAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
		Users:            fakeUsers{provisionalErr: databaseErr},
		AuditRateLimiter: fakeAuditRateLimiter{},
		Clock:            fakeClock{now: time.Now().UTC()},
	}
	if _, err := application.OnboardingProfile(context.Background(), "Bearer onboarding-session"); !errors.Is(err, databaseErr) || errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("persistence failure was misclassified: %v", err)
	}

	application = App{
		Auth:             fakeAuth{principal: ports.Principal{UserID: "active-user", Scopes: []string{"api:user"}}},
		Audits:           fakeAudits{err: databaseErr},
		AuditRateLimiter: fakeAuditRateLimiter{},
		Clock:            fakeClock{now: time.Now().UTC()},
	}
	if _, err := application.OnboardingProfile(context.Background(), "Bearer ordinary-session"); !errors.Is(err, databaseErr) || errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("denial audit failure was misclassified: %v", err)
	}
}

func TestOnboardingProfileFailsClosedForUnavailableOrInvalidUsernameSuggestions(t *testing.T) {
	databaseErr := errors.New("username index unavailable")
	for _, test := range []struct {
		name        string
		suggestions *controlledUsernameSuggestions
	}{
		{name: "dependency failure", suggestions: &controlledUsernameSuggestions{err: databaseErr}},
		{name: "invalid adapter value", suggestions: &controlledUsernameSuggestions{suggestion: "not available"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var events []audit.Event
			application := App{
				Auth:                fakeAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
				Users:               fakeUsers{provisional: identity.User{ID: "provisional-owner", DisplayName: "Provider Seed", Status: identity.StatusProvisional}},
				UsernameSuggestions: test.suggestions,
				Audits:              fakeAudits{events: &events},
				AuditRateLimiter:    fakeAuditRateLimiter{},
				Clock:               fakeClock{now: time.Now().UTC()},
			}

			_, err := application.OnboardingProfile(context.Background(), "Bearer onboarding-session")
			if err == nil {
				t.Fatal("invalid suggestion boundary succeeded")
			}
			if test.suggestions.err != nil && !errors.Is(err, databaseErr) {
				t.Fatalf("dependency error = %v, want %v", err, databaseErr)
			}
			if len(events) != 0 {
				t.Fatalf("failed onboarding read appended audit events: %+v", events)
			}
		})
	}
}

func TestOnboardingProfileRateLimitsBeforeUsernameAvailabilityLookup(t *testing.T) {
	suggestions := &controlledUsernameSuggestions{suggestion: "provider.seed"}
	application := App{
		Auth:                fakeAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
		Users:               fakeUsers{provisional: identity.User{ID: "provisional-owner", DisplayName: "Provider Seed", Status: identity.StatusProvisional}},
		UsernameSuggestions: suggestions,
		AuditRateLimiter:    fakeAuditRateLimiter{denied: true},
		Clock:               fakeClock{now: time.Now().UTC()},
	}

	if _, err := application.OnboardingProfile(context.Background(), "Bearer onboarding-session"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("onboarding profile error = %v, want ErrRateLimited", err)
	}
	if suggestions.displayName != "" {
		t.Fatalf("rate-limited request reached username availability lookup for %q", suggestions.displayName)
	}
}
