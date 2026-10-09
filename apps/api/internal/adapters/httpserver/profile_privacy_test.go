package httpserver

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOwnProfilePrivacyRejectsUnauthenticatedRequests(t *testing.T) {
	handler, _ := New(app.App{Auth: timeZoneRouteAuth{err: app.ErrUnauthenticated}}, nil, Options{})
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequest(method, "/v1/me/profile/privacy", strings.NewReader(`{"visibility":"private","expectedRevision":1,"confirmed":true}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "profile-privacy-0001")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
}

type privacyRouteStore struct {
	profiles map[string]app.OwnProfilePrivacy
}

func (s privacyRouteStore) GetOwnProfilePrivacy(_ context.Context, owner string) (app.OwnProfilePrivacy, error) {
	value, ok := s.profiles[owner]
	if !ok {
		return app.OwnProfilePrivacy{}, ports.ErrNotFound
	}
	return value, nil
}
func (s privacyRouteStore) UpdateOwnProfilePrivacy(_ context.Context, _ app.ProfilePrivacyCommand) (app.ProfilePrivacyResult, error) {
	return app.ProfilePrivacyResult{}, ports.ErrUnavailable
}

func TestOwnProfilePrivacyHTTPAccountIsolationAndAdmission(t *testing.T) {
	state := &deletionRouteState{commands: map[string]app.AccountDeletionCommand{}}
	repository := privacyRouteStore{profiles: map[string]app.OwnProfilePrivacy{"owner": {UserID: "owner", Visibility: identity.ProfileVisibilityPublic, Revision: 1}, "other": {UserID: "other", Visibility: identity.ProfileVisibilityPrivate, Revision: 3}}}
	handler, _ := New(app.App{Auth: profileRouteAuth{state}, Users: deletionRouteUsers{}, ProfilePrivacy: repository, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}}, nil, Options{})
	call := func(method, credential, body string, want int) string {
		t.Helper()
		request := httptest.NewRequest(method, "/v1/me/profile/privacy", strings.NewReader(body))
		request.Header.Set("Authorization", credential)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "privacy-review-key-0001")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s status=%d want=%d body=%s", method, response.Code, want, response.Body.String())
		}
		return response.Body.String()
	}
	valid := `{"visibility":"private","expectedRevision":1,"confirmed":true}`
	for _, credential := range []string{"", "malformed", "Bearer expired", "Bearer provider-jwt", "Bearer provisional"} {
		call(http.MethodGet, credential, "", 401)
		call(http.MethodPut, credential, valid, 401)
	}
	if body := call(http.MethodGet, "Bearer other", "", 200); !strings.Contains(body, `"userId":"other"`) || !strings.Contains(body, `"visibility":"private"`) {
		t.Fatal(body)
	}
	call(http.MethodPut, "Bearer owner", strings.Replace(valid, `"visibility"`, `"userId":"other","visibility"`, 1), 422)
	call(http.MethodPut, "Bearer owner", strings.Replace(valid, `"confirmed":true`, `"confirmed":false`, 1), 400)
	call(http.MethodPut, "Bearer owner", strings.Replace(valid, `"private"`, `"followers"`, 1), 422)
	call(http.MethodGet, "Bearer dependency-failed", "", 503)
	// Missing propagation dependencies fail closed even for a legitimate owner.
	call(http.MethodPut, "Bearer owner", valid, 503)
	if repository.profiles["owner"].Visibility != identity.ProfileVisibilityPublic || repository.profiles["other"].Revision != 3 {
		t.Fatal("rejected change mutated privacy")
	}
}
