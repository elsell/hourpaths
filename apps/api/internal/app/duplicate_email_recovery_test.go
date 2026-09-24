package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type recordingRecoveryDeclines struct {
	userID string
	event  audit.Event
	err    error
}

type statefulRecoveryHints struct {
	currentEmail string
	decline      identity.DuplicateEmailRecoveryDecline
}

func (s *statefulRecoveryHints) HasActiveEmailMatch(_ context.Context, claims ports.Claims) (bool, error) {
	if s.decline.AppliesTo(identity.UserID(claims.Issuer, claims.Subject), claims.Email) {
		return false, nil
	}
	return claims.EmailVerified && identity.NormalizeEmail(claims.Email) != "", nil
}

func (s *statefulRecoveryHints) DeclineDuplicateEmailRecovery(_ context.Context, userID string, _ audit.Event) error {
	decline, err := identity.NewDuplicateEmailRecoveryDecline(userID, s.currentEmail)
	if err != nil {
		return err
	}
	s.decline = decline
	return nil
}

func (r *recordingRecoveryDeclines) DeclineDuplicateEmailRecovery(_ context.Context, userID string, event audit.Event) error {
	r.userID = userID
	r.event = event
	return r.err
}

func TestProvisionalOwnerDeclinesDuplicateEmailRecovery(t *testing.T) {
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	declines := &recordingRecoveryDeclines{}
	application := App{
		Auth:                             fakeAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
		Users:                            fakeUsers{provisional: identity.User{ID: "provisional-owner", Email: "person@example.com", Status: identity.StatusProvisional}},
		DuplicateAccountRecoveryDeclines: declines,
		AuditRateLimiter:                 fakeAuditRateLimiter{},
		Clock:                            fakeClock{now: now},
	}

	if err := application.DeclineDuplicateEmailRecovery(context.Background(), "Bearer onboarding-session"); err != nil {
		t.Fatal(err)
	}
	if declines.userID != "provisional-owner" {
		t.Fatalf("decline owner = %q", declines.userID)
	}
	event := declines.event
	if event.Action != audit.DuplicateEmailRecoveryDeclined || event.OwnerUserID != "provisional-owner" || event.ActorUserID != "provisional-owner" || event.TargetType != "user" || event.TargetID != "provisional-owner" || event.Outcome != audit.Succeeded || !event.OccurredAt.Equal(now) {
		t.Fatalf("decline audit = %+v", event)
	}
}

func TestDuplicateEmailRecoveryDeclineRejectsEveryOtherCredential(t *testing.T) {
	for _, test := range []struct {
		name      string
		principal ports.Principal
		authErr   error
		user      identity.User
		userErr   error
		wantErr   error
	}{
		{name: "ordinary active session", principal: ports.Principal{UserID: "active-user", Scopes: []string{"api:user"}}},
		{name: "mixed scopes", principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding", "api:user"}}},
		{name: "missing owner", principal: ports.Principal{Scopes: []string{"api:onboarding"}}},
		{name: "malformed credential", authErr: ports.ErrInvalidCredential, wantErr: ports.ErrInvalidCredential},
		{name: "unknown provisional owner", principal: ports.Principal{UserID: "missing", Scopes: []string{"api:onboarding"}}, userErr: ports.ErrNotFound},
		{name: "mismatched repository owner", principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}, user: identity.User{ID: "other", Status: identity.StatusProvisional}},
		{name: "active account behind onboarding scope", principal: ports.Principal{UserID: "active-user", Scopes: []string{"api:onboarding"}}, user: identity.User{ID: "active-user", Status: identity.StatusActive}},
	} {
		t.Run(test.name, func(t *testing.T) {
			wantErr := test.wantErr
			if wantErr == nil {
				wantErr = ErrUnauthenticated
			}
			declines := &recordingRecoveryDeclines{}
			application := App{
				Auth:                             fakeAuth{principal: test.principal, err: test.authErr},
				Users:                            fakeUsers{provisional: test.user, provisionalErr: test.userErr},
				DuplicateAccountRecoveryDeclines: declines,
				Audits:                           &fakeAudits{},
				AuditRateLimiter:                 fakeAuditRateLimiter{},
				Clock:                            fakeClock{now: time.Now().UTC()},
			}
			if err := application.DeclineDuplicateEmailRecovery(context.Background(), "Bearer rejected"); !errors.Is(err, wantErr) {
				t.Fatalf("decline error = %v, want %v", err, wantErr)
			}
			if declines.userID != "" {
				t.Fatalf("rejected credential reached decline persistence for %q", declines.userID)
			}
		})
	}
}

func TestDuplicateEmailRecoveryDeclinePreservesAuthenticationDependencyFailure(t *testing.T) {
	dependencyErr := errors.New("session repository unavailable")
	declines := &recordingRecoveryDeclines{}
	application := App{
		Auth:                             fakeAuth{err: dependencyErr},
		DuplicateAccountRecoveryDeclines: declines,
	}
	if err := application.DeclineDuplicateEmailRecovery(context.Background(), "Bearer locally-valid-session"); !errors.Is(err, dependencyErr) {
		t.Fatalf("authentication dependency error = %v, want %v", err, dependencyErr)
	}
	if declines.userID != "" {
		t.Fatalf("authentication dependency failure reached decline persistence for %q", declines.userID)
	}
}

func TestDuplicateEmailRecoveryDeclineFailsClosedOnDependencyError(t *testing.T) {
	dependencyErr := errors.New("decline persistence unavailable")
	application := App{
		Auth:                             fakeAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
		Users:                            fakeUsers{provisional: identity.User{ID: "provisional-owner", Status: identity.StatusProvisional}},
		DuplicateAccountRecoveryDeclines: &recordingRecoveryDeclines{err: dependencyErr},
		AuditRateLimiter:                 fakeAuditRateLimiter{},
		Clock:                            fakeClock{now: time.Now().UTC()},
	}
	if err := application.DeclineDuplicateEmailRecovery(context.Background(), "Bearer onboarding-session"); !errors.Is(err, dependencyErr) {
		t.Fatalf("decline error = %v, want dependency failure", err)
	}
}

func TestDeclinedEmailOnboardsOnLaterExchangeButChangedEmailMayOfferRecoveryAgain(t *testing.T) {
	now := time.Date(2026, 7, 21, 19, 0, 0, 0, time.UTC)
	claims := ports.Claims{Issuer: "https://provider.example", Subject: "provisional-subject", Email: " Person@Example.COM ", EmailVerified: true}
	userID := identity.UserID(claims.Issuer, claims.Subject)
	hints := &statefulRecoveryHints{currentEmail: "person@example.com"}
	application := App{
		Auth:                             fakeAuth{principal: ports.Principal{UserID: userID, Scopes: []string{"api:onboarding"}}},
		IdentityVerifier:                 fakeIdentityVerifier{claims: claims},
		Users:                            fakeUsers{user: identity.User{ID: userID, Status: identity.StatusProvisional}, provisional: identity.User{ID: userID, Email: "person@example.com", Status: identity.StatusProvisional}},
		DuplicateAccountHints:            hints,
		DuplicateAccountRecoveryDeclines: hints,
		Sessions:                         &recordingIdentityExchangeSessions{},
		SessionTTL:                       time.Hour,
		SessionAbsoluteTTL:               12 * time.Hour,
		AuditRateLimiter:                 fakeAuditRateLimiter{},
		AllowAccountProvisioning:         true,
		Clock:                            fakeClock{now: now},
	}
	if err := application.DeclineDuplicateEmailRecovery(context.Background(), "Bearer onboarding-session"); err != nil {
		t.Fatal(err)
	}
	outcome, err := application.ExchangeIdentityToken(context.Background(), "same-email-provider-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := outcome.(OnboardingIdentityExchange); !ok {
		t.Fatalf("same-email exchange after decline = %T, want onboarding", outcome)
	}

	changedClaims := claims
	changedClaims.Email = "changed@example.com"
	application.IdentityVerifier = fakeIdentityVerifier{claims: changedClaims}
	outcome, err = application.ExchangeIdentityToken(context.Background(), "changed-email-provider-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := outcome.(DuplicateEmailRecoveryIdentityExchange); !ok {
		t.Fatalf("changed matching email exchange = %T, want recovery offer", outcome)
	}
}
