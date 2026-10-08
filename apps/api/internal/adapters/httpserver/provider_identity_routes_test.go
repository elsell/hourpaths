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

type providerRouteRepository struct{ mutations int }

func (*providerRouteRepository) ListProviderIdentities(context.Context, string) ([]identity.Provider, error) {
	return []identity.Provider{identity.ProviderGoogle}, nil
}
func (r *providerRouteRepository) AdmitIdentityLink(context.Context, app.IdentityLinkAdmission) error {
	r.mutations++
	return nil
}
func (r *providerRouteRepository) CompleteIdentityLink(context.Context, app.IdentityLinkCompletion) error {
	r.mutations++
	return nil
}
func (r *providerRouteRepository) UnlinkProviderIdentity(context.Context, app.IdentityUnlink) error {
	r.mutations++
	return ports.ErrConflict
}

func TestRejectedLinkProofDoesNotInvalidateTheAuthenticatedSession(t *testing.T) {
	state := &deletionRouteState{commands: map[string]app.AccountDeletionCommand{}}
	repo := &providerRouteRepository{}
	application := app.App{Auth: state, Users: deletionRouteUsers{}, ProviderIdentities: repo, IdentityVerifier: rejectedIdentityVerifier{}, Clock: docsClock{now: time.Now()}, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}}
	handler, _ := New(application, nil, Options{})
	call := func(method, path, credential, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Authorization", credential)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	response := call(http.MethodPost, "/v1/me/identities/link/complete", "Bearer owner", `{"challengeId":"challenge","identityToken":"rejected-target-proof"}`)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "identity_link_proof_invalid") {
		t.Fatalf("target proof rejected as session failure: %d %s", response.Code, response.Body.String())
	}
	response = call(http.MethodGet, "/v1/me/identities", "Bearer owner", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"provider":"google"`) {
		t.Fatalf("existing session stopped working: %d %s", response.Code, response.Body.String())
	}
	response = call(http.MethodPost, "/v1/me/identities/link/complete", "Bearer invalid", `{"challengeId":"challenge","identityToken":"rejected-target-proof"}`)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid app session accepted: %d", response.Code)
	}
	if repo.mutations != 0 {
		t.Fatal("unproven linking mutated identity repository")
	}
}

func TestProviderRoutesRequireCurrentAccountAndFailClosed(t *testing.T) {
	state := &deletionRouteState{commands: map[string]app.AccountDeletionCommand{}}
	repo := &providerRouteRepository{}
	application := app.App{Auth: state, Users: deletionRouteUsers{}, ProviderIdentities: repo, IdentityVerifier: rejectedIdentityVerifier{}, Clock: docsClock{now: time.Now()}, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}}
	handler, _ := New(application, nil, Options{})
	for _, route := range []struct{ method, path, body string }{
		{http.MethodGet, "/v1/me/identities", ""},
		{http.MethodPost, "/v1/me/identities/link", `{"provider":"apple"}`},
		{http.MethodPost, "/v1/me/identities/link/complete", `{"challengeId":"challenge","identityToken":"proof"}`},
		{http.MethodDelete, "/v1/me/identities/google", `{"reviewedUserId":"owner"}`},
	} {
		for _, credential := range []string{"", "Bearer invalid", "Basic owner", "Bearer"} {
			request := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
			request.Header.Set("Authorization", credential)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s accepted rejected session: %d", route.method, route.path, response.Code)
			}
		}
	}
	request := httptest.NewRequest(http.MethodDelete, "/v1/me/identities/google", strings.NewReader(`{"reviewedUserId":"another-account"}`))
	request.Header.Set("Authorization", "Bearer owner")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code < 400 || repo.mutations != 0 {
		t.Fatalf("cross-account review reached persistence: %d, mutations %d", response.Code, repo.mutations)
	}
	application.ProviderIdentities = nil
	handler, _ = New(application, nil, Options{})
	request = httptest.NewRequest(http.MethodGet, "/v1/me/identities", nil)
	request.Header.Set("Authorization", "Bearer owner")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing provider dependency did not fail closed: %d", response.Code)
	}
}
