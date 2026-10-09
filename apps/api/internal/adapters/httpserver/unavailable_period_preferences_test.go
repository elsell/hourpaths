package httpserver

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/preferences"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type unavailablePeriodRouteAuth struct {
	principal ports.Principal
	err       error
	received  *string
}

func (a unavailablePeriodRouteAuth) Authenticate(_ context.Context, value string) (ports.Principal, error) {
	*a.received = value
	return a.principal, a.err
}

type unavailablePeriodRouteStore struct {
	gets     *[]string
	commands *[]app.UnavailablePeriodCommand
}

func (s unavailablePeriodRouteStore) GetUnavailablePeriod(_ context.Context, id string) (app.UnavailablePeriodPreference, error) {
	*s.gets = append(*s.gets, id)
	return app.UnavailablePeriodPreference{Period: preferences.UnavailablePeriod{Enabled: true, StartMinute: 1320, EndMinute: 480}, Revision: 1, TimeZone: "Etc/UTC"}, nil
}
func (s unavailablePeriodRouteStore) UpdateUnavailablePeriod(_ context.Context, c app.UnavailablePeriodCommand) (app.UnavailablePeriodResult, error) {
	*s.commands = append(*s.commands, c)
	return app.UnavailablePeriodResult{Preference: app.UnavailablePeriodPreference{Period: c.Period, Revision: c.ExpectedRevision + 1, TimeZone: c.ReviewedTimeZone}}, nil
}
func TestUnavailablePeriodRoutesAuthenticateAndBindPreferencesToSessionOwner(t *testing.T) {
	for _, method := range []string{"GET", "PUT"} {
		for _, tc := range []struct {
			name, credential, scope string
			failure                 error
			status                  int
		}{
			{"missing", "", "api:user", app.ErrUnauthenticated, 401},
			{"malformed", "Bearer malformed", "api:user", ports.ErrInvalidCredential, 401},
			{"expired", "Bearer expired", "api:user", ports.ErrInvalidCredential, 401},
			{"revoked", "Bearer revoked", "api:user", ports.ErrInvalidCredential, 401},
			{"provider", "Bearer provider", "api:user", ports.ErrInvalidCredential, 401},
			{"wrongissuer", "Bearer wrong-issuer", "api:user", ports.ErrInvalidCredential, 401},
			{"wrongaudience", "Bearer wrong-audience", "api:user", ports.ErrInvalidCredential, 401},
			{"provisional", "Bearer provisional", "api:onboarding", nil, 401},
			{"dependency", "Bearer unavailable", "api:user", ports.ErrUnavailable, 503},
			{"legitimate", "Bearer current", "api:user", nil, 200},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				received := "unset"
				var gets []string
				var commands []app.UnavailablePeriodCommand
				application := app.App{Auth: unavailablePeriodRouteAuth{ports.Principal{UserID: "owner", Scopes: []string{tc.scope}}, tc.failure, &received}, Users: timeZoneRouteUsers{}, UnavailablePeriods: unavailablePeriodRouteStore{&gets, &commands}, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}}
				handler, _ := newHTTPTestServer(application, nil, Options{})
				request := httptest.NewRequest(method, "/v1/me/unavailable-period?userId=someone-else", strings.NewReader(`{"expectedRevision":0,"reviewedTimeZone":"Etc/UTC","enabled":true,"startMinute":1320,"endMinute":480}`))
				request.Header.Set("Authorization", tc.credential)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", "quiet-hours-route-0001")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != tc.status || received != tc.credential {
					t.Fatalf("boundary: %d %q %s", response.Code, received, response.Body.String())
				}
				if tc.status != 200 {
					if len(gets)+len(commands) != 0 {
						t.Fatal("rejected credential reached preference store")
					}
					return
				}
				if method == "GET" {
					if len(gets) != 1 || gets[0] != "owner" {
						t.Fatalf("read owner: %v", gets)
					}
				} else {
					if len(commands) != 1 || commands[0].ActorUserID != "owner" || commands[0].Audit.OwnerUserID != "owner" {
						t.Fatalf("update ownership: %+v", commands)
					}
				}
			})
		}
	}
}
