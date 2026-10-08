package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledRecoveryAdmissions struct {
	admissions []AccountRecoveryAdmission
	provider   identity.Provider
	err        error
}

type controlledRecoveryCompleter struct{ commands []AccountRecoveryCompletion }

func (r *controlledRecoveryCompleter) CompleteAccountRecovery(_ context.Context, _ string, c AccountRecoveryCompletion) (Session, error) {
	r.commands = append(r.commands, c)
	return Session{Token: "recovered-session", ExpiresAt: c.ExpiresAt}, nil
}

func TestRecoveryCompletionRequiresPurposeBoundSignedProof(t *testing.T) {
	for _, tc := range []struct {
		name, nonce string
		provider    identity.Provider
		accepted    bool
	}{
		{"recovery", "hourpaths-recovery:" + strings.Repeat("a", 64), identity.ProviderGoogle, true},
		{"ordinary sign-in", "ordinary", identity.ProviderGoogle, false},
		{"ordinary linking", "hourpaths-link:" + strings.Repeat("a", 64), identity.ProviderGoogle, false},
		{"malformed nonce", "hourpaths-recovery:" + strings.Repeat("z", 64), identity.ProviderGoogle, false},
		{"unknown provider", "hourpaths-recovery:" + strings.Repeat("a", 64), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &controlledRecoveryCompleter{}
			var events []audit.Event
			now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
			a := App{Auth: fakeAuth{principal: ports.Principal{UserID: "enrollment", Scopes: []string{"api:onboarding"}}}, Users: fakeUsers{provisional: identity.User{ID: "enrollment", Status: identity.StatusProvisional}}, Audits: fakeAudits{events: &events}, AuditRateLimiter: fakeAuditRateLimiter{}, Clock: fakeClock{now: now}, SessionTTL: 30 * 24 * time.Hour, AccountRecoveryCompleter: r, IdentityVerifier: fakeIdentityVerifier{claims: ports.Claims{Issuer: "https://broker.example", Subject: "existing-google", Provider: tc.provider, Nonce: tc.nonce}}}
			session, err := a.FinishAccountRecovery(context.Background(), "Bearer onboarding", "challenge", "signed-proof")
			if !tc.accepted {
				if !errors.Is(err, ErrAccountRecoveryProofInvalid) || len(r.commands) != 0 {
					t.Fatalf("unproven recovery reached completion: %v", err)
				}
				if len(events) != 1 || events[0].Action != audit.ResourceAccessDenied || events[0].OwnerUserID != "enrollment" {
					t.Fatal("denied recovery was not owner-audited")
				}
				return
			}
			if err != nil || session.Token != "recovered-session" || len(r.commands) != 1 {
				t.Fatalf("recovery failed: %v", err)
			}
			c := r.commands[0]
			hash := sha256.Sum256([]byte(tc.nonce))
			if c.UserID != "enrollment" || c.Subject != "existing-google" || string(c.NonceHash) != string(hash[:]) || c.Created.CorrelationID != c.Completed.CorrelationID || !c.Created.OccurredAt.Equal(c.Completed.OccurredAt) {
				t.Fatal("recovery lost owner, proof or atomic audit context")
			}
		})
	}
}

func (r *controlledRecoveryAdmissions) AdmitAccountRecovery(_ context.Context, _ string, c AccountRecoveryAdmission) (identity.Provider, error) {
	if r.err != nil {
		return "", r.err
	}
	r.admissions = append(r.admissions, c)
	return r.provider, nil
}

func TestRecoveryChallengeRequiresExactProvisionalOwner(t *testing.T) {
	for _, tc := range []struct {
		name     string
		scopes   []string
		user     identity.User
		accepted bool
	}{
		{"provisional", []string{"api:onboarding"}, identity.User{ID: "enrollment", Status: identity.StatusProvisional}, true},
		{"ordinary session", []string{"api:user"}, identity.User{ID: "enrollment", Status: identity.StatusProvisional}, false},
		{"mixed scopes", []string{"api:onboarding", "api:user"}, identity.User{ID: "enrollment", Status: identity.StatusProvisional}, false},
		{"completed enrollment", []string{"api:onboarding"}, identity.User{ID: "enrollment", Status: identity.StatusActive}, false},
		{"wrong owner", []string{"api:onboarding"}, identity.User{ID: "another", Status: identity.StatusProvisional}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
			repo := &controlledRecoveryAdmissions{provider: identity.ProviderGoogle}
			a := App{Auth: fakeAuth{principal: ports.Principal{UserID: "enrollment", Scopes: tc.scopes}}, Users: fakeUsers{provisional: tc.user},
				Clock: fakeClock{now: now}, AuditRateLimiter: fakeAuditRateLimiter{}, Audits: fakeAudits{}, AccountRecoveryAdmissions: repo}
			challenge, err := a.BeginAccountRecovery(context.Background(), "Bearer current-onboarding-session")
			if !tc.accepted {
				if !errors.Is(err, ErrUnauthenticated) || len(repo.admissions) != 0 {
					t.Fatalf("invalid enrollment admitted: %v", err)
				}
				return
			}
			if err != nil || len(repo.admissions) != 1 {
				t.Fatalf("admission failed: %v", err)
			}
			second, err := a.BeginAccountRecovery(context.Background(), "Bearer current-onboarding-session")
			if err != nil || second.Nonce == challenge.Nonce || second.ID == challenge.ID {
				t.Fatal("recovery challenges must be independent")
			}
			hash := sha256.Sum256([]byte(challenge.Nonce))
			c := repo.admissions[0]
			if c.UserID != "enrollment" || c.ID != challenge.ID || string(c.NonceHash) != string(hash[:]) || !challenge.ExpiresAt.Equal(now.Add(10*time.Minute)) || challenge.Provider != identity.ProviderGoogle || !strings.HasPrefix(challenge.Nonce, "hourpaths-recovery:") || !c.Audit.Valid() {
				t.Fatal("recovery challenge lost owner, nonce, expiry, provider, or audit binding")
			}
		})
	}
}

func TestRecoveryProofIsNotAnOrdinarySignInCredential(t *testing.T) {
	for _, tc := range []struct {
		name, nonce string
		accepted    bool
	}{
		{"ordinary sign-in", "ordinary-random-nonce", true},
		{"recovery proof", "hourpaths-recovery:" + strings.Repeat("a", 64), false},
		{"malformed recovery proof", "hourpaths-recovery:invalid", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sessions := &recordingIdentityExchangeSessions{}
			a := App{
				IdentityVerifier: fakeIdentityVerifier{claims: ports.Claims{Issuer: "https://broker.example", Subject: "existing-google", Provider: identity.ProviderGoogle, Nonce: tc.nonce}},
				Sessions:         sessions, Users: fakeUsers{user: identity.User{ID: "retained-account", Status: identity.StatusActive}},
				Clock: fakeClock{now: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)}, AuditRateLimiter: fakeAuditRateLimiter{},
				SessionTTL: time.Hour, SessionAbsoluteTTL: 30 * 24 * time.Hour,
			}
			_, err := a.ExchangeIdentityToken(context.Background(), "verified-provider-token")
			if tc.accepted {
				if err != nil || sessions.userID != "retained-account" {
					t.Fatalf("ordinary sign-in failed: %v", err)
				}
			} else if !errors.Is(err, ErrUnauthenticated) || sessions.userID != "" {
				t.Fatalf("recovery proof escaped its purpose: session=%q error=%v", sessions.userID, err)
			}
		})
	}
}
