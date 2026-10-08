package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type recoveryHTTPAuth struct{}

func (recoveryHTTPAuth) Authenticate(_ context.Context, header string) (ports.Principal, error) {
	if header == "Bearer enrollment" {
		return ports.Principal{UserID: "enrollment", Scopes: []string{"api:onboarding"}}, nil
	}
	if header == "Bearer active" {
		return ports.Principal{UserID: "active", Scopes: []string{"api:user"}}, nil
	}
	return ports.Principal{}, ports.ErrInvalidCredential
}

type recoveryHTTPVerifier struct{}

func (recoveryHTTPVerifier) Verify(_ context.Context, token string) (ports.Claims, error) {
	if token != "valid-target-proof" {
		return ports.Claims{}, ports.ErrInvalidCredential
	}
	return ports.Claims{Issuer: "https://broker.example", Subject: "existing-google", Provider: identity.ProviderGoogle, Nonce: "hourpaths-recovery:" + strings.Repeat("a", 64)}, nil
}

type recoveryHTTPState struct{ admitted, completed int }

func (r *recoveryHTTPState) AdmitAccountRecovery(_ context.Context, _ string, c app.AccountRecoveryAdmission) (identity.Provider, error) {
	r.admitted++
	return identity.ProviderGoogle, nil
}
func (r *recoveryHTTPState) CompleteAccountRecovery(_ context.Context, _ string, c app.AccountRecoveryCompletion) (app.Session, error) {
	r.completed++
	return app.Session{Token: "recovered-account-session", ExpiresAt: c.ExpiresAt}, nil
}

func TestAccountRecoveryRoutesRequireOnboardingSessionAndKeepRejectedProofSeparate(t *testing.T) {
	r := &recoveryHTTPState{}
	a := app.App{Auth: recoveryHTTPAuth{}, IdentityVerifier: recoveryHTTPVerifier{}, Users: onboardingHTTPUsers{provisional: identity.User{ID: "enrollment", Status: identity.StatusProvisional}}, Audits: onboardingHTTPAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: time.Now().UTC()}, SessionTTL: 30 * 24 * time.Hour, AccountRecoveryAdmissions: r, AccountRecoveryCompleter: r}
	handler, _ := New(a, nil, Options{})
	call := func(path, credential, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("Authorization", credential)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	const begin = "/v1/onboarding/duplicate-email-recovery"
	const complete = begin + "/complete"
	for _, credential := range []string{"", "Bearer invalid", "Bearer active", "Basic enrollment"} {
		for _, route := range []struct{ path, body string }{{begin, ""}, {complete, `{"challengeId":"challenge","identityToken":"valid-target-proof"}`}} {
			response := call(route.path, credential, route.body)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("recovery admitted %q: %d", credential, response.Code)
			}
		}
	}
	if r.admitted != 0 || r.completed != 0 {
		t.Fatal("unauthorized recovery reached persistence")
	}
	response := call(complete, "Bearer enrollment", `{"challengeId":"challenge","identityToken":"wrong-target-proof"}`)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "account_recovery_proof_invalid") || r.completed != 0 {
		t.Fatalf("target proof rejected as application credential: %d %s", response.Code, response.Body.String())
	}
	response = call(begin, "Bearer enrollment", "")
	if response.Code != http.StatusOK || r.admitted != 1 || !strings.Contains(response.Body.String(), `"provider":"google"`) {
		t.Fatalf("valid enrollment lost access: %d %s", response.Code, response.Body.String())
	}
	response = call(complete, "Bearer enrollment", `{"challengeId":"challenge","identityToken":"valid-target-proof"}`)
	if response.Code != http.StatusOK || r.completed != 1 || !strings.Contains(response.Body.String(), `"nextAction":"home"`) {
		t.Fatalf("completed recovery did not enter home: %d %s", response.Code, response.Body.String())
	}
	a.AccountRecoveryCompleter = nil
	handler, _ = New(a, nil, Options{})
	response = call(complete, "Bearer enrollment", `{"challengeId":"challenge","identityToken":"valid-target-proof"}`)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing completion dependency accepted: %d", response.Code)
	}
}
