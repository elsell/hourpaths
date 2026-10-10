package stats

import (
	"context"
	"errors"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	platform "github.com/elsell/hour-paths/apps/api/internal/app"
	application "github.com/elsell/hour-paths/apps/api/internal/app/stats"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/stats"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type boundary struct {
	policyFailure, auditFailure, limited bool
	reads                                int
}

func (f *boundary) Authenticate(_ context.Context, token string) (ports.Principal, error) {
	if token == "Bearer wrong-scope" {
		return ports.Principal{UserID: "viewer", Scopes: []string{"other"}}, nil
	}
	if token != "Bearer application-session" {
		return ports.Principal{}, platform.ErrUnauthenticated
	}
	return ports.Principal{UserID: "viewer", Scopes: []string{"api:user"}}, nil
}
func (f *boundary) ReadPath(_ context.Context, viewer, path, participant string) (application.Snapshot, error) {
	f.reads++
	if viewer != "viewer" || path != "shared" || (participant != "viewer" && participant != "participant") {
		return application.Snapshot{}, ports.ErrNotFound
	}
	return application.Snapshot{TimeZone: "UTC", FirstDayOfWeek: 1, Records: []domain.Record{{PathID: path, StartedAt: f.Now().Add(-time.Hour), EndedAt: f.Now(), TimeZone: "UTC"}}}, nil
}
func (f *boundary) Check(_ context.Context, kind, id, permission, actor string) (bool, error) {
	if f.policyFailure {
		return false, errors.New("policy unavailable")
	}
	return kind == "path" && id == "shared" && permission == "view" && actor == "viewer", nil
}
func (*boundary) WriteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (*boundary) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *boundary) AppendAuditEvent(context.Context, audit.Event) error {
	if f.auditFailure {
		return errors.New("audit unavailable")
	}
	return nil
}
func (*boundary) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}
func (f *boundary) Allow(string, time.Time) bool { return !f.limited }
func (*boundary) Now() time.Time                 { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) }

func TestPathStatisticsHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, token, path      string
		status                 int
		policy, audit, limited bool
	}{
		{"personal", "application-session", "shared", 200, false, false, false},
		{"selected participant", "application-session", "shared?participantId=participant", 200, false, false, false},
		{"anonymous", "", "shared", 401, false, false, false},
		{"malformed", "malformed", "shared", 401, false, false, false},
		{"expired", "expired", "shared", 401, false, false, false},
		{"provider token", "eyJhbGciOiJIUzI1NiJ9.e30.invalid", "shared", 401, false, false, false},
		{"wrong scope", "wrong-scope", "shared", 401, false, false, false},
		{"foreign Path", "application-session", "foreign", 404, false, false, false},
		{"foreign participant", "application-session", "shared?participantId=foreign", 404, false, false, false},
		{"policy dependency", "application-session", "shared", 500, true, false, false},
		{"audit dependency", "application-session", "shared", 500, false, true, false},
		{"rate limit", "application-session", "shared", 429, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &boundary{policyFailure: tc.policy, auditFailure: tc.audit, limited: tc.limited}
			service := &application.Service{Auth: f, PathRepository: f, Authorizer: f, Audits: f, AuditRateLimiter: f, Clock: f}
			mux := http.NewServeMux()
			api := humago.New(mux, huma.DefaultConfig("Stats test", "1"))
			Register(api, service)
			parts := strings.SplitN(tc.path, "?", 2)
			url := "/v1/paths/" + parts[0] + "/statistics"
			if len(parts) == 2 {
				url += "?" + parts[1]
			}
			request := httptest.NewRequest(http.MethodGet, url, nil)
			if tc.token != "" {
				request.Header.Set("Authorization", "Bearer "+tc.token)
			}
			result := httptest.NewRecorder()
			mux.ServeHTTP(result, request)
			if result.Code != tc.status {
				t.Fatalf("status %d: %s", result.Code, result.Body.String())
			}
			if tc.status != 200 && strings.Contains(result.Body.String(), "calendar") {
				t.Fatal("failure exposed statistics")
			}
			if tc.status == 200 && !strings.Contains(result.Body.String(), `"totalSeconds":3600`) {
				t.Fatal("authorized data missing")
			}
			if (tc.status == 401 || tc.limited || tc.policy || tc.name == "foreign Path") && f.reads != 0 {
				t.Fatal("read preceded admission")
			}
		})
	}
}
