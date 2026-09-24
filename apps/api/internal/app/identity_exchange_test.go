package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type recordingIdentityExchangeSessions struct {
	userID            string
	scopes            []string
	expiresAt         time.Time
	absoluteExpiresAt time.Time
}

type recordingDuplicateAccountHints struct {
	match  bool
	err    error
	claims ports.Claims
	order  *[]string
}

func (f *recordingDuplicateAccountHints) HasActiveEmailMatch(_ context.Context, claims ports.Claims) (bool, error) {
	f.claims = claims
	if f.order != nil {
		*f.order = append(*f.order, "duplicate-email-hint")
	}
	return f.match, f.err
}

type orderedIdentityExchangeUsers struct {
	user  identity.User
	order *[]string
}

func (f orderedIdentityExchangeUsers) ResolveOrCreate(context.Context, ports.Claims, audit.Event, audit.Event, audit.Event, bool) (identity.User, error) {
	if f.order != nil {
		*f.order = append(*f.order, "resolve-or-create")
	}
	return f.user, nil
}

func (f orderedIdentityExchangeUsers) GetUser(context.Context, string) (identity.User, error) {
	return f.user, nil
}

func (f orderedIdentityExchangeUsers) GetProvisionalUser(context.Context, string) (identity.User, error) {
	return f.user, nil
}

func (orderedIdentityExchangeUsers) DisableUser(context.Context, string, audit.Event) error {
	return nil
}

func (f *recordingIdentityExchangeSessions) CreateSession(_ context.Context, userID string, scopes []string, _ []byte, expiresAt, absoluteExpiresAt time.Time, _ audit.Event) (string, error) {
	f.userID = userID
	f.scopes = append([]string(nil), scopes...)
	f.expiresAt = expiresAt
	f.absoluteExpiresAt = absoluteExpiresAt
	return "onboarding-session", nil
}

func (*recordingIdentityExchangeSessions) RotateSession(context.Context, string, string, []string, time.Time, audit.Event, audit.Event) (string, time.Time, error) {
	panic("unexpected session rotation")
}

func (*recordingIdentityExchangeSessions) RevokeSession(context.Context, string, audit.Event) error {
	panic("unexpected session revocation")
}

func TestIdentityExchangeOutcomeVariantsCarryOnlyTheirBoundaryData(t *testing.T) {
	expiresAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	returning := IdentityExchangeOutcome(ReturningUserIdentityExchange{
		UserID:  "active-user",
		Session: Session{Token: "opaque-session", ExpiresAt: expiresAt},
	})
	recovery := IdentityExchangeOutcome(DuplicateEmailRecoveryIdentityExchange{
		ProvisionalUserID: "provisional-user",
	})
	onboarding := IdentityExchangeOutcome(OnboardingIdentityExchange{
		ProvisionalUserID: "provisional-user",
	})

	if got, ok := returning.(ReturningUserIdentityExchange); !ok || got.UserID != "active-user" || got.Session.Token != "opaque-session" {
		t.Fatalf("returning exchange = %#v", returning)
	}
	if got, ok := recovery.(DuplicateEmailRecoveryIdentityExchange); !ok || got.ProvisionalUserID != "provisional-user" {
		t.Fatalf("recovery exchange = %#v", recovery)
	}
	if got, ok := onboarding.(OnboardingIdentityExchange); !ok || got.ProvisionalUserID != "provisional-user" {
		t.Fatalf("onboarding exchange = %#v", onboarding)
	}
}

func TestFirstIdentityExchangeReturnsRestrictedOnboardingSession(t *testing.T) {
	now := time.Date(2026, 7, 21, 13, 30, 0, 0, time.UTC)
	sessions := &recordingIdentityExchangeSessions{}
	application := App{
		Clock:                    fakeClock{now: now},
		IdentityVerifier:         fakeIdentityVerifier{claims: ports.Claims{Issuer: "https://accounts.example", Subject: "first-identity", Email: "person@example.com", EmailVerified: true}},
		Users:                    fakeUsers{user: identity.User{ID: "provisional-user", Status: identity.StatusProvisional}},
		Sessions:                 sessions,
		SessionTTL:               time.Hour,
		SessionAbsoluteTTL:       12 * time.Hour,
		AuditRateLimiter:         fakeAuditRateLimiter{},
		AllowAccountProvisioning: true,
	}

	outcome, err := application.ExchangeIdentityToken(context.Background(), "valid-first-provider-token")
	if err != nil {
		t.Fatalf("exchange first identity: %v", err)
	}
	onboarding, ok := any(outcome).(OnboardingIdentityExchange)
	if !ok {
		t.Fatalf("exchange outcome = %T, want OnboardingIdentityExchange", outcome)
	}
	if onboarding.ProvisionalUserID != "provisional-user" {
		t.Fatalf("provisional user ID = %q, want provisional-user", onboarding.ProvisionalUserID)
	}
	if onboarding.Session.Token != "onboarding-session" || !onboarding.Session.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("onboarding session = %+v", onboarding.Session)
	}
	if sessions.userID != "provisional-user" {
		t.Fatalf("session user ID = %q, want provisional-user", sessions.userID)
	}
	if !reflect.DeepEqual(sessions.scopes, []string{"api:onboarding"}) {
		t.Fatalf("session scopes = %v, want only api:onboarding", sessions.scopes)
	}
	if !sessions.expiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("session expiry = %s, want %s", sessions.expiresAt, now.Add(time.Hour))
	}
	if !sessions.absoluteExpiresAt.Equal(now.Add(12 * time.Hour)) {
		t.Fatalf("absolute session expiry = %s, want %s", sessions.absoluteExpiresAt, now.Add(12*time.Hour))
	}
}

func TestIdentityExchangeFailsClosedForUnknownUserStatus(t *testing.T) {
	now := time.Date(2026, 7, 21, 14, 0, 0, 0, time.UTC)
	sessions := &recordingIdentityExchangeSessions{}
	application := App{
		Clock:                    fakeClock{now: now},
		IdentityVerifier:         fakeIdentityVerifier{claims: ports.Claims{Issuer: "https://accounts.example", Subject: "unknown-status", Email: "person@example.com", EmailVerified: true}},
		Users:                    fakeUsers{user: identity.User{ID: "unknown-user"}},
		Sessions:                 sessions,
		SessionTTL:               time.Hour,
		SessionAbsoluteTTL:       12 * time.Hour,
		AuditRateLimiter:         fakeAuditRateLimiter{},
		AllowAccountProvisioning: true,
	}

	if _, err := application.ExchangeIdentityToken(context.Background(), "valid-provider-token"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unknown account status exchange error = %v, want ErrUnauthenticated", err)
	}
	if sessions.userID != "" {
		t.Fatalf("unknown account status created a session for %q", sessions.userID)
	}
}

func TestVerifiedEmailMatchReturnsRecoveryWithRestrictedContinuationSession(t *testing.T) {
	now := time.Date(2026, 7, 21, 16, 0, 0, 0, time.UTC)
	order := []string{}
	hints := &recordingDuplicateAccountHints{match: true, order: &order}
	sessions := &recordingIdentityExchangeSessions{}
	claims := ports.Claims{
		Issuer: "https://new-provider.example", Subject: "new-provider-subject",
		Email: " Person@Example.COM ", EmailVerified: true,
	}
	application := App{
		Clock:                    fakeClock{now: now},
		IdentityVerifier:         fakeIdentityVerifier{claims: claims},
		DuplicateAccountHints:    hints,
		Users:                    orderedIdentityExchangeUsers{user: identity.User{ID: "provisional-user", Status: identity.StatusProvisional}, order: &order},
		Sessions:                 sessions,
		SessionTTL:               time.Hour,
		SessionAbsoluteTTL:       12 * time.Hour,
		AuditRateLimiter:         fakeAuditRateLimiter{},
		AllowAccountProvisioning: true,
	}

	outcome, err := application.ExchangeIdentityToken(context.Background(), "verified-new-provider-token")
	if err != nil {
		t.Fatalf("exchange matching verified identity: %v", err)
	}
	recovery, ok := any(outcome).(DuplicateEmailRecoveryIdentityExchange)
	if !ok {
		t.Fatalf("exchange outcome = %T, want DuplicateEmailRecoveryIdentityExchange", outcome)
	}
	if recovery.ProvisionalUserID != "provisional-user" {
		t.Fatalf("recovery provisional user = %q, want provisional-user", recovery.ProvisionalUserID)
	}
	if recovery.Session.Token != "onboarding-session" || !recovery.Session.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("recovery continuation session = %+v", recovery.Session)
	}
	if !reflect.DeepEqual(sessions.scopes, []string{"api:onboarding"}) {
		t.Fatalf("recovery session scopes = %v, want only api:onboarding", sessions.scopes)
	}
	if !reflect.DeepEqual(order, []string{"duplicate-email-hint", "resolve-or-create"}) {
		t.Fatalf("duplicate lookup/create order = %v", order)
	}
	if hints.claims != claims {
		t.Fatalf("hint claims = %+v, want verified provider claims %+v", hints.claims, claims)
	}
}

func TestDuplicateRecoveryRequiresVerifiedEmailAndProvisionalIdentity(t *testing.T) {
	tests := []struct {
		name        string
		claims      ports.Claims
		user        identity.User
		hintMatch   bool
		wantHints   int
		wantOutcome any
	}{
		{name: "verified active match", claims: ports.Claims{Issuer: "issuer", Subject: "new", Email: "person@example.com", EmailVerified: true}, user: identity.User{ID: "provisional", Status: identity.StatusProvisional}, hintMatch: true, wantHints: 1, wantOutcome: DuplicateEmailRecoveryIdentityExchange{}},
		{name: "unverified email cannot hint", claims: ports.Claims{Issuer: "issuer", Subject: "new", Email: "person@example.com"}, user: identity.User{ID: "provisional", Status: identity.StatusProvisional}, hintMatch: true, wantOutcome: OnboardingIdentityExchange{}},
		{name: "empty verified email cannot hint", claims: ports.Claims{Issuer: "issuer", Subject: "new", EmailVerified: true}, user: identity.User{ID: "provisional", Status: identity.StatusProvisional}, hintMatch: true, wantOutcome: OnboardingIdentityExchange{}},
		{name: "nonmatching verified email onboards", claims: ports.Claims{Issuer: "issuer", Subject: "new", Email: "other@example.com", EmailVerified: true}, user: identity.User{ID: "provisional", Status: identity.StatusProvisional}, wantHints: 1, wantOutcome: OnboardingIdentityExchange{}},
		{name: "returning active identity ignores hint", claims: ports.Claims{Issuer: "issuer", Subject: "existing", Email: "person@example.com", EmailVerified: true}, user: identity.User{ID: "active", Status: identity.StatusActive}, hintMatch: true, wantHints: 1, wantOutcome: ReturningUserIdentityExchange{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			hints := &recordingDuplicateAccountHints{match: test.hintMatch}
			countingHints := duplicateHintCounter{delegate: hints, calls: &calls}
			application := App{
				Clock: fakeClock{now: time.Date(2026, 7, 21, 16, 30, 0, 0, time.UTC)}, IdentityVerifier: fakeIdentityVerifier{claims: test.claims},
				DuplicateAccountHints: countingHints, Users: orderedIdentityExchangeUsers{user: test.user}, Sessions: &recordingIdentityExchangeSessions{},
				SessionTTL: time.Hour, SessionAbsoluteTTL: 12 * time.Hour, AuditRateLimiter: fakeAuditRateLimiter{}, AllowAccountProvisioning: true,
			}
			outcome, err := application.ExchangeIdentityToken(context.Background(), "identity-token")
			if err != nil {
				t.Fatal(err)
			}
			if calls != test.wantHints {
				t.Fatalf("duplicate hint calls = %d, want %d", calls, test.wantHints)
			}
			if reflect.TypeOf(outcome) != reflect.TypeOf(test.wantOutcome) {
				t.Fatalf("outcome = %T, want %T", outcome, test.wantOutcome)
			}
		})
	}
}

func TestDuplicateRecoveryHintFailureStopsProvisioningAndSessionCreation(t *testing.T) {
	dependencyErr := errors.New("duplicate lookup unavailable")
	order := []string{}
	sessions := &recordingIdentityExchangeSessions{}
	application := App{
		Clock: fakeClock{now: time.Date(2026, 7, 21, 17, 0, 0, 0, time.UTC)},
		IdentityVerifier: fakeIdentityVerifier{claims: ports.Claims{
			Issuer: "issuer", Subject: "new", Email: "person@example.com", EmailVerified: true,
		}},
		DuplicateAccountHints:    &recordingDuplicateAccountHints{err: dependencyErr, order: &order},
		Users:                    orderedIdentityExchangeUsers{user: identity.User{ID: "provisional", Status: identity.StatusProvisional}, order: &order},
		Sessions:                 sessions,
		SessionTTL:               time.Hour,
		SessionAbsoluteTTL:       12 * time.Hour,
		AuditRateLimiter:         fakeAuditRateLimiter{},
		AllowAccountProvisioning: true,
	}

	if _, err := application.ExchangeIdentityToken(context.Background(), "identity-token"); !errors.Is(err, dependencyErr) {
		t.Fatalf("duplicate hint dependency error = %v, want %v", err, dependencyErr)
	}
	if !reflect.DeepEqual(order, []string{"duplicate-email-hint"}) {
		t.Fatalf("work continued after duplicate hint failure: %v", order)
	}
	if sessions.userID != "" {
		t.Fatalf("duplicate hint failure created session for %q", sessions.userID)
	}
}

type duplicateHintCounter struct {
	delegate *recordingDuplicateAccountHints
	calls    *int
}

func (f duplicateHintCounter) HasActiveEmailMatch(ctx context.Context, claims ports.Claims) (bool, error) {
	*f.calls++
	return f.delegate.HasActiveEmailMatch(ctx, claims)
}
