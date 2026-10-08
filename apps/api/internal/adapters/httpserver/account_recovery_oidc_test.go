package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/oidcauth"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Exercise the new HTTP operation with real discovery, JWKS and signed tokens.
// The completion port is controlled; its persistence races are PostgreSQL tests.
func TestAccountRecoverySignedProviderBoundary(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var unavailable atomic.Bool
	var issuer string
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "jwks_uri": issuer + "/keys", "authorization_endpoint": issuer + "/auth", "token_endpoint": issuer + "/token"})
		case "/keys":
			if unavailable.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "recovery", Algorithm: "RS256", Use: "sig"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer broker.Close()
	issuer = broker.URL
	now := time.Now().UTC()
	for _, tc := range []struct {
		name                            string
		mutate                          func(map[string]any)
		key                             *rsa.PrivateKey
		malformed, unavailable, allowed bool
	}{
		{name: "legitimate signed proof", allowed: true},
		{name: "wrong issuer", mutate: func(c map[string]any) { c["iss"] = "https://untrusted.example" }},
		{name: "wrong audience", mutate: func(c map[string]any) { c["aud"] = "other-client" }},
		{name: "expired", mutate: func(c map[string]any) { c["exp"] = now.Add(-time.Hour).Unix() }},
		{name: "wrong signature", key: other},
		{name: "malformed", malformed: true},
		{name: "ordinary sign in", mutate: func(c map[string]any) { delete(c, "nonce") }},
		{name: "link proof", mutate: func(c map[string]any) { c["nonce"] = "hourpaths-link:" + strings.Repeat("a", 64) }},
		{name: "missing provider", mutate: func(c map[string]any) { delete(c, "identities") }},
		{name: "ambiguous provider", mutate: func(c map[string]any) {
			c["identities"] = map[string]any{"google": map[string]any{"userId": "g"}, "apple": map[string]any{"userId": "a"}}
		}},
		{name: "key service unavailable", unavailable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unavailable.Store(tc.unavailable)
			verifier, err := oidcauth.New(context.Background(), issuer, "", []string{"web", "mobile"}, true)
			if err != nil {
				t.Fatal(err)
			}
			claims := map[string]any{"iss": issuer, "sub": "existing-google", "aud": "mobile", "exp": now.Add(time.Minute).Unix(), "nonce": "hourpaths-recovery:" + strings.Repeat("a", 64), "identities": map[string]any{"google": map[string]any{"userId": "g"}}}
			if tc.mutate != nil {
				tc.mutate(claims)
			}
			signingKey := key
			if tc.key != nil {
				signingKey = tc.key
			}
			signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: signingKey, KeyID: "recovery"}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			proof, err := jwt.Signed(signer).Claims(claims).Serialize()
			if err != nil {
				t.Fatal(err)
			}
			if tc.malformed {
				proof = "not-a-token"
			}
			state := &recoveryHTTPState{}
			application := app.App{Auth: recoveryHTTPAuth{}, IdentityVerifier: verifier, Users: onboardingHTTPUsers{provisional: identity.User{ID: "enrollment", Status: identity.StatusProvisional}}, Audits: onboardingHTTPAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: now}, SessionTTL: 30 * 24 * time.Hour, AccountRecoveryAdmissions: state, AccountRecoveryCompleter: state}
			handler, _ := New(application, nil, Options{})
			body, _ := json.Marshal(map[string]string{"challengeId": "challenge", "identityToken": proof})
			request := httptest.NewRequest(http.MethodPost, "/v1/onboarding/duplicate-email-recovery/complete", strings.NewReader(string(body)))
			request.Header.Set("Authorization", "Bearer enrollment")
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if tc.allowed {
				if response.Code != http.StatusOK || state.completed != 1 {
					t.Fatalf("legitimate proof rejected: %d", response.Code)
				}
			} else {
				if response.Code != http.StatusBadRequest || state.completed != 0 {
					t.Fatalf("untrusted proof reached completion: %d", response.Code)
				}
			}
		})
	}
}
