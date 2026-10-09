package routes

import (
	"context"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http/httptest"
	"testing"
)

type connectionRouteAuth struct {
	received *string
	err      error
	scope    string
}

func (a connectionRouteAuth) Authenticate(_ context.Context, authorization string) (ports.Principal, error) {
	*a.received = authorization
	return ports.Principal{UserID: "viewer", Scopes: []string{a.scope}}, a.err
}
func TestConnectionRoutesAlwaysAuthenticateBeforeReadingOrRemoving(t *testing.T) {
	for _, route := range []struct{ method, path string }{{"GET", "/v1/profiles/alice/followers"}, {"GET", "/v1/profiles/alice/following"}, {"DELETE", "/v1/me/followers/follower-id"}} {
		for _, tc := range []struct {
			name, authorization, scope string
			err                        error
			status                     int
		}{
			{"missing", "", "api:user", platformapp.ErrUnauthenticated, 401},
			{"malformed", "Bearer invalid", "api:user", ports.ErrInvalidCredential, 401},
			{"expired", "Bearer expired", "api:user", ports.ErrInvalidCredential, 401},
			{"provider token", "Bearer provider", "api:user", ports.ErrInvalidCredential, 401},
			{"wrong issuer", "Bearer wrong-issuer", "api:user", ports.ErrInvalidCredential, 401},
			{"wrong audience", "Bearer wrong-audience", "api:user", ports.ErrInvalidCredential, 401},
			{"provisional", "Bearer provisional", "api:onboarding", nil, 401},
			{"dependency", "Bearer session", "api:user", ports.ErrUnavailable, 503},
		} {
			t.Run(route.path+"/"+tc.name, func(t *testing.T) {
				received := "unset"
				service := application.New(application.Dependencies{Auth: connectionRouteAuth{received: &received, err: tc.err, scope: tc.scope}})
				request := httptest.NewRequest(route.method, route.path, nil)
				request.Header.Set("Authorization", tc.authorization)
				request.Header.Set("Idempotency-Key", "remove-follower-key-01")
				response := httptest.NewRecorder()
				handler(service).ServeHTTP(response, request)
				if response.Code != tc.status || received != tc.authorization {
					t.Fatalf("boundary: %d %q %s", response.Code, received, response.Body.String())
				}
			})
		}
	}
}
