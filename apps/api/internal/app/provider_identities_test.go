package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledProviderIdentities struct {
	admissions  []IdentityLinkAdmission
	completions []IdentityLinkCompletion
	unlinks     []IdentityUnlink
}

func (r *controlledProviderIdentities) ListProviderIdentities(context.Context, string) ([]identity.Provider, error) {
	return []identity.Provider{identity.ProviderGoogle}, nil
}
func (r *controlledProviderIdentities) AdmitIdentityLink(_ context.Context, c IdentityLinkAdmission) error {
	r.admissions = append(r.admissions, c)
	return nil
}
func (r *controlledProviderIdentities) CompleteIdentityLink(_ context.Context, c IdentityLinkCompletion) error {
	r.completions = append(r.completions, c)
	return nil
}
func (r *controlledProviderIdentities) UnlinkProviderIdentity(_ context.Context, c IdentityUnlink) error {
	r.unlinks = append(r.unlinks, c)
	return nil
}

func TestProviderLinkRequiresAuthenticatedOwnerAndSignedProviderProof(t *testing.T) {
	for _, tc := range []struct {
		name      string
		provider  identity.Provider
		nonce     string
		authError error
		want      error
	}{
		{"Google proof", identity.ProviderGoogle, identityLinkNoncePrefix + strings.Repeat("a", 64), nil, nil},
		{"Apple proof", identity.ProviderApple, identityLinkNoncePrefix + strings.Repeat("b", 64), nil, nil},
		{"missing provider", "", identityLinkNoncePrefix + strings.Repeat("b", 64), nil, ErrIdentityLinkProofInvalid},
		{"unsupported provider", "github", identityLinkNoncePrefix + strings.Repeat("b", 64), nil, ErrIdentityLinkProofInvalid},
		{"ordinary sign-in token", identity.ProviderApple, "ordinary-login", nil, ErrIdentityLinkProofInvalid},
		{"missing nonce", identity.ProviderApple, "", nil, ErrIdentityLinkProofInvalid},
		{"malformed nonce", identity.ProviderApple, identityLinkNoncePrefix + strings.Repeat("z", 64), nil, ErrIdentityLinkProofInvalid},
		{"invalid application session", identity.ProviderApple, identityLinkNoncePrefix + strings.Repeat("b", 64), ErrUnauthenticated, ErrUnauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &controlledProviderIdentities{}
			application := App{Auth: fakeAuth{err: tc.authError, principal: ports.Principal{UserID: "owner", Scopes: []string{"api:user"}}}, Users: fakeUsers{user: identity.User{ID: "owner", Status: identity.StatusActive}}, ProviderIdentities: repo, IdentityVerifier: fakeIdentityVerifier{claims: ports.Claims{Issuer: "https://broker.example", Subject: "provider-subject", Provider: tc.provider, Nonce: tc.nonce}}, Clock: fakeClock{now: time.Now()}, AuditRateLimiter: fakeAuditRateLimiter{}}
			err := application.FinishIdentityLink(context.Background(), "Bearer session", "challenge", "signed-token")
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if tc.want != nil {
				if len(repo.completions) != 0 {
					t.Fatal("unverified link reached persistence")
				}
				return
			}
			if len(repo.completions) != 1 || repo.completions[0].UserID != "owner" || len(repo.completions[0].NonceHash) != 32 || !repo.completions[0].Audit.Valid() {
				t.Fatal("missing owner-bound audited proof")
			}
		})
	}
}

func TestLinkProofCannotCreateAnOrdinarySession(t *testing.T) {
	sessions := &recordingIdentityExchangeSessions{}
	application := App{IdentityVerifier: fakeIdentityVerifier{claims: ports.Claims{Issuer: "https://broker.example", Subject: "provider-subject", Provider: identity.ProviderApple, Nonce: identityLinkNoncePrefix + strings.Repeat("a", 64)}}, Sessions: sessions, SessionTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour}
	if _, err := application.ExchangeIdentityToken(context.Background(), "signed-link-proof"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("link proof exchanged for a sign-in session: %v", err)
	}
	if sessions.userID != "" {
		t.Fatal("link proof created session")
	}
}

func TestLinkChallengeAndUnlinkRemainBoundToReviewedAccount(t *testing.T) {
	repo := &controlledProviderIdentities{}
	now := time.Now().UTC()
	application := App{Auth: fakeAuth{principal: ports.Principal{UserID: "owner", Scopes: []string{"api:user"}}}, Users: fakeUsers{user: identity.User{ID: "owner", Status: identity.StatusActive}}, ProviderIdentities: repo, Clock: fakeClock{now: now}, AuditRateLimiter: fakeAuditRateLimiter{}}
	first, err := application.BeginIdentityLink(context.Background(), "Bearer session", identity.ProviderApple)
	if err != nil {
		t.Fatal(err)
	}
	second, err := application.BeginIdentityLink(context.Background(), "Bearer session", identity.ProviderApple)
	if err != nil {
		t.Fatal(err)
	}
	if !validIdentityLinkNonce(first.Nonce) || first.Nonce == second.Nonce || first.ID == second.ID || first.ExpiresAt.Sub(repo.admissions[0].CreatedAt) != 10*time.Minute {
		t.Fatal("link challenges must be independent, bounded and unpredictable")
	}
	if err := application.UnlinkIdentity(context.Background(), "Bearer session", "previous-account", identity.ProviderGoogle); !errors.Is(err, ports.ErrConflict) || len(repo.unlinks) != 0 {
		t.Fatal("account-switch review bypassed")
	}
}
