package routes

import (
	"context"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type profilePathsControlled struct{ controlledService }

func (s *profilePathsControlled) GetProfilePaths(_ context.Context, authorization, username string) (application.ProfilePaths, error) {
	s.authorization, s.username = authorization, username
	return application.ProfilePaths{Count: 2, Active: application.ActiveFollowingCandidate{ParticipantID: "person", Username: "alice", DisplayName: "Alice", Timers: []application.ActiveFollowingTimer{}}}, s.err
}
func TestProfilePathsRoutePreservesAuthorizationAndOpaqueFailures(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{{nil, 200}, {ports.ErrInvalidCredential, 401}, {ports.ErrNotFound, 404}, {ports.ErrUnavailable, 503}} {
		service := &profilePathsControlled{controlledService: controlledService{err: test.err}}
		request := httptest.NewRequest(http.MethodGet, "/v1/profiles/alice/paths", nil)
		request.Header.Set("Authorization", "Bearer session")
		response := httptest.NewRecorder()
		handler(service).ServeHTTP(response, request)
		if response.Code != test.status || service.authorization != "Bearer session" || service.username != "alice" {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if test.err != nil && strings.Contains(response.Body.String(), "pathCount") {
			t.Fatal("failure exposes profile data")
		}
		if test.err == nil && !strings.Contains(response.Body.String(), `"pathCount":2`) {
			t.Fatal(response.Body.String())
		}
	}
}
