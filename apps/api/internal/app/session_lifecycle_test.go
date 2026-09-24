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

func TestRevokeSessionAllowsExactOnboardingSessionWithoutGrantingApplicationAccess(t *testing.T) {
	now := time.Date(2026, 7, 21, 15, 0, 0, 0, time.UTC)
	sessions := &fakeSessions{}
	a := App{
		Clock:            fakeClock{now: now},
		Auth:             fakeAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
		Users:            fakeUsers{provisional: identity.User{ID: "provisional-owner", Status: identity.StatusProvisional}},
		Sessions:         sessions,
		AuditRateLimiter: fakeAuditRateLimiter{},
	}
	if err := a.RevokeSession(context.Background(), "Bearer onboarding-session"); err != nil {
		t.Fatalf("revoke onboarding session: %v", err)
	}
	if !sessions.revoked || sessions.revokedEvent.Action != audit.SessionRevoked || sessions.revokedEvent.OwnerUserID != "provisional-owner" || sessions.revokedEvent.ActorUserID != "provisional-owner" {
		t.Fatalf("onboarding revocation was not owner-audited: revoked=%v event=%+v", sessions.revoked, sessions.revokedEvent)
	}
}

func TestRevokeSessionRejectsMixedScopeCredential(t *testing.T) {
	sessions := &fakeSessions{}
	a := App{
		Clock:            fakeClock{now: time.Now().UTC()},
		Auth:             fakeAuth{principal: ports.Principal{UserID: "owner", Scopes: []string{"api:onboarding", "api:user"}}},
		Sessions:         sessions,
		AuditRateLimiter: fakeAuditRateLimiter{},
	}
	if err := a.RevokeSession(context.Background(), "Bearer mixed-session"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("mixed-scope revoke error = %v, want ErrUnauthenticated", err)
	}
	if sessions.revoked {
		t.Fatal("mixed-scope credential reached session revocation")
	}
}
