package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type timeZoneRouteAuth struct{ err error }

func (a timeZoneRouteAuth) Authenticate(context.Context, string) (ports.Principal, error) {
	return ports.Principal{UserID: "owner", Scopes: []string{"api:user"}}, a.err
}

type timeZoneRouteUsers struct{}

func (timeZoneRouteUsers) ResolveOrCreate(context.Context, ports.Claims, audit.Event, audit.Event, audit.Event, bool) (identity.User, error) {
	return identity.User{}, nil
}
func (timeZoneRouteUsers) GetUser(context.Context, string) (identity.User, error) {
	return identity.User{ID: "owner", Status: identity.StatusActive}, nil
}
func (timeZoneRouteUsers) GetProvisionalUser(context.Context, string) (identity.User, error) {
	return identity.User{}, ports.ErrNotFound
}
func (timeZoneRouteUsers) DisableUser(context.Context, string, audit.Event) error { return nil }

type timeZoneRouteAudits struct{}

func (timeZoneRouteAudits) AppendAuditEvent(context.Context, audit.Event) error { return nil }
func (timeZoneRouteAudits) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}

type timeZoneRoutePreferences struct {
	preference app.TimeZonePreference
	result     app.TimeZonePreferenceResult
	err        error
	command    *app.TimeZonePreferenceCommand
}

func (r timeZoneRoutePreferences) GetTimeZonePreference(context.Context, string) (app.TimeZonePreference, error) {
	return r.preference, r.err
}
func (r timeZoneRoutePreferences) UpdateTimeZonePreference(_ context.Context, command app.TimeZonePreferenceCommand) (app.TimeZonePreferenceResult, error) {
	if r.command != nil {
		*r.command = command
	}
	return r.result, r.err
}

func timeZoneRouteHandler(repository app.TimeZonePreferenceRepository) http.Handler {
	now := time.Date(2026, 8, 3, 14, 30, 0, 0, time.UTC)
	application := app.App{Auth: timeZoneRouteAuth{}, Users: timeZoneRouteUsers{}, TimeZonePreferences: repository, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: now}}
	handler, _ := New(application, nil, Options{})
	return handler
}

func TestTimeZonePreferenceRoutesExposeCurrentAndConfirmedUpdateContract(t *testing.T) {
	effectiveAt := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	get := httptest.NewRecorder()
	timeZoneRouteHandler(timeZoneRoutePreferences{preference: app.TimeZonePreference{TimeZone: "America/New_York", EffectiveAt: effectiveAt}}).
		ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/me/time-zone", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"timeZone":"America/New_York"`) || !strings.Contains(get.Body.String(), `"changed":false`) {
		t.Fatalf("GET response = %d %s", get.Code, get.Body.String())
	}

	var command app.TimeZonePreferenceCommand
	update := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/v1/me/time-zone", strings.NewReader(`{"reviewedTimeZone":"America/New_York","proposedTimeZone":"Europe/Paris","confirmed":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer session")
	request.Header.Set("Idempotency-Key", "time-zone-key-0001")
	timeZoneRouteHandler(timeZoneRoutePreferences{result: app.TimeZonePreferenceResult{Preference: app.TimeZonePreference{TimeZone: "Europe/Paris", EffectiveAt: time.Date(2026, 8, 3, 14, 30, 0, 0, time.UTC)}, Changed: true}, command: &command}).ServeHTTP(update, request)
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), `"timeZone":"Europe/Paris"`) || !strings.Contains(update.Body.String(), `"changed":true`) {
		t.Fatalf("PUT response = %d %s", update.Code, update.Body.String())
	}
	if command.ReviewedTimeZone != "America/New_York" || command.ProposedTimeZone != "Europe/Paris" {
		t.Fatalf("update command = %+v", command)
	}
}

func TestTimeZonePreferenceUpdateRequiresConfirmationAndMapsConflicts(t *testing.T) {
	for name, body := range map[string]string{
		"omitted": `{"reviewedTimeZone":"America/New_York","proposedTimeZone":"Europe/Paris"}`,
		"false":   `{"reviewedTimeZone":"America/New_York","proposedTimeZone":"Europe/Paris","confirmed":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/v1/me/time-zone", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "time-zone-key-0001")
			response := httptest.NewRecorder()
			timeZoneRouteHandler(timeZoneRoutePreferences{}).ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity && response.Code != http.StatusBadRequest {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
	for _, failure := range []error{ports.ErrConflict, ports.ErrIdempotencyConflict} {
		request := httptest.NewRequest(http.MethodPut, "/v1/me/time-zone", strings.NewReader(`{"reviewedTimeZone":"America/New_York","proposedTimeZone":"Europe/Paris","confirmed":true}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "time-zone-key-0001")
		response := httptest.NewRecorder()
		timeZoneRouteHandler(timeZoneRoutePreferences{err: failure}).ServeHTTP(response, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("%v response = %d %s", failure, response.Code, response.Body.String())
		}
	}
}
