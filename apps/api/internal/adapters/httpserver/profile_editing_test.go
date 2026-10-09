package httpserver

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
)

func TestOwnProfileRoutesRejectUnauthenticatedRequests(t *testing.T) {
	handler, _ := newHTTPTestServer(app.App{Auth: timeZoneRouteAuth{err: app.ErrUnauthenticated}}, nil, Options{})
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequest(method, "/v1/me/profile", strings.NewReader(`{"username":"new.name","displayName":"New Name","description":"About me","expectedRevision":1}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "profile-save-0001")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
}

type profileRouteAuth struct{ *deletionRouteState }

func (a profileRouteAuth) Authenticate(ctx context.Context, credential string) (ports.Principal, error) {
	if credential == "Bearer provisional" {
		return ports.Principal{UserID: "owner", Scopes: []string{"api:onboarding"}}, nil
	}
	if credential == "Bearer dependency-failed" {
		return ports.Principal{}, ports.ErrUnavailable
	}
	return a.deletionRouteState.Authenticate(ctx, credential)
}

type profileRouteStore struct {
	rows   map[string]app.OwnProfile
	failed bool
}

func (s *profileRouteStore) GetOwnProfile(_ context.Context, owner string) (app.OwnProfile, error) {
	if s.failed {
		return app.OwnProfile{}, ports.ErrUnavailable
	}
	row, ok := s.rows[owner]
	if !ok {
		return app.OwnProfile{}, ports.ErrNotFound
	}
	return row, nil
}
func (s *profileRouteStore) UpdateOwnProfile(_ context.Context, command app.ProfileUpdateCommand) (app.OwnProfile, error) {
	if s.failed {
		return app.OwnProfile{}, ports.ErrUnavailable
	}
	row, ok := s.rows[command.ActorUserID]
	if !ok {
		return app.OwnProfile{}, ports.ErrNotFound
	}
	if row.Revision != command.Update.ExpectedRevision {
		return app.OwnProfile{}, ports.ErrConflict
	}
	for id, other := range s.rows {
		if id != command.ActorUserID && strings.EqualFold(other.Text.Username, command.Update.Text.Username) {
			return app.OwnProfile{}, ports.ErrUsernameUnavailable
		}
	}
	row.Text = command.Update.Text
	row.Revision++
	s.rows[command.ActorUserID] = row
	return row, nil
}
func TestOwnProfileHTTPKeepsIdentityOwnerAndMapsFailures(t *testing.T) {
	state := &deletionRouteState{commands: map[string]app.AccountDeletionCommand{}}
	repository := &profileRouteStore{rows: map[string]app.OwnProfile{
		"owner": {UserID: "owner", Revision: 1, Text: identity.ProfileText{Username: "owner", DisplayName: "Owner"}},
		"other": {UserID: "other", Revision: 1, Text: identity.ProfileText{Username: "other", DisplayName: "Other"}},
	}}
	handler, _ := newHTTPTestServer(app.App{Auth: profileRouteAuth{state}, Users: deletionRouteUsers{}, Profiles: repository, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}}, nil, Options{})
	call := func(method, credential, body string, want int) string {
		t.Helper()
		request := httptest.NewRequest(method, "/v1/me/profile", strings.NewReader(body))
		request.Header.Set("Authorization", credential)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "profile-edit-0001")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s status=%d want=%d body=%s", method, response.Code, want, response.Body.String())
		}
		return response.Body.String()
	}
	valid := `{"username":"changed","displayName":"Changed Name","description":"About me","expectedRevision":1}`
	for _, credential := range []string{"", "malformed", "Bearer expired", "Bearer provider-jwt", "Bearer provisional"} {
		call(http.MethodGet, credential, "", 401)
		call(http.MethodPut, credential, valid, 401)
	}
	if got := call(http.MethodGet, "Bearer other", "", 200); !strings.Contains(got, `"userId":"other"`) {
		t.Fatal(got)
	}
	// The owner is derived exclusively from the session: the DTO rejects supplied identity fields.
	call(http.MethodPut, "Bearer owner", strings.Replace(valid, `"username"`, `"userId":"other","username"`, 1), 422)
	call(http.MethodPut, "Bearer owner", strings.Replace(valid, `"changed"`, `"OTHER"`, 1), 409)
	call(http.MethodPut, "Bearer owner", strings.Replace(valid, `"Changed Name"`, `"   "`, 1), 400)
	call(http.MethodPut, "Bearer owner", valid, 200)
	call(http.MethodPut, "Bearer owner", valid, 409)
	if repository.rows["other"].Text.DisplayName != "Other" || repository.rows["other"].Revision != 1 {
		t.Fatal("another account changed")
	}
	call(http.MethodGet, "Bearer dependency-failed", "", 503)
	call(http.MethodPut, "Bearer dependency-failed", valid, 503)
	repository.failed = true
	call(http.MethodGet, "Bearer owner", "", 503)
	call(http.MethodPut, "Bearer owner", valid, 503)
}
