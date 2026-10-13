package httpserver

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type restrictionRouteStore struct {
	decision *moderation.Enforcement
	failure  error
	owners   []string
}

func (r *restrictionRouteStore) ActiveAccountRestriction(_ context.Context, owner string, _ time.Time) (*moderation.Enforcement, error) {
	r.owners = append(r.owners, owner)
	return r.decision, r.failure
}

func TestAccountRestrictionHTTPAdmissionAndPrivateReview(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, token, scope     string
		authFailure, errorRead error
		decision               *moderation.Enforcement
		status                 int
	}{
		{name: "missing", authFailure: app.ErrUnauthenticated, status: 401},
		{name: "malformed", token: "Bearer malformed", authFailure: ports.ErrInvalidCredential, status: 401},
		{name: "expired", token: "Bearer expired", authFailure: ports.ErrInvalidCredential, status: 401},
		{name: "wrong issuer", token: "Bearer issuer", authFailure: ports.ErrInvalidCredential, status: 401},
		{name: "wrong audience", token: "Bearer audience", authFailure: ports.ErrInvalidCredential, status: 401},
		{name: "revoked", token: "Bearer revoked", authFailure: ports.ErrInvalidCredential, status: 401},
		{name: "provisional", token: "Bearer onboarding", scope: "api:onboarding", status: 401},
		{name: "dependency", token: "Bearer valid", scope: "api:user", errorRead: ports.ErrUnavailable, status: 503},
		{name: "unrestricted", token: "Bearer valid", scope: "api:user", status: 204},
		{name: "restricted", token: "Bearer valid", scope: "api:user", decision: &moderation.Enforcement{ID: "notice", SubjectUserID: "owner", Action: moderation.Ban, PolicyReason: "reviewed reason", IssuedAt: now.Add(-time.Hour)}, status: 403},
		{name: "foreign decision", token: "Bearer valid", scope: "api:user", decision: &moderation.Enforcement{ID: "peer-secret", SubjectUserID: "peer", Action: moderation.Ban, PolicyReason: "private", IssuedAt: now.Add(-time.Hour)}, status: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			received := ""
			store := &restrictionRouteStore{decision: tc.decision, failure: tc.errorRead}
			application := app.App{Auth: weekStartRouteAuth{ports.Principal{UserID: "owner", Scopes: []string{tc.scope}}, tc.authFailure, &received}, Users: timeZoneRouteUsers{}, AccountRestrictions: store, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: now}}
			reached := 0
			handler, _ := New(application, nil, Options{PolicyAdmission: acceptedPolicyFixture{}, DomainRegistrations: []func(huma.API){func(api huma.API) {
				huma.Register(api, huma.Operation{OperationID: "account-test-mutation", Method: "POST", Path: "/v1/account-test", Security: []map[string][]string{{"oidc": {}}}}, func(context.Context, *MeInput) (*NoContentOutput, error) {
					reached++
					return &NoContentOutput{Status: 204}, nil
				})
			}}})
			request := func(method, path string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, path, nil)
				req.Header.Set("Authorization", tc.token)
				res := httptest.NewRecorder()
				handler.ServeHTTP(res, req)
				return res
			}
			res := request("POST", "/v1/account-test")
			if res.Code != tc.status || (reached == 1) != (tc.status == 204) {
				t.Fatalf("mutation: %d %s reached=%d", res.Code, res.Body.String(), reached)
			}
			if tc.status == 403 && !strings.Contains(res.Body.String(), "account_restricted") {
				t.Fatal(res.Body.String())
			}
			review := request("GET", "/v1/me/restriction?userId=peer")
			expected := tc.status
			if expected == 204 || expected == 403 {
				expected = 200
			}
			if review.Code != expected {
				t.Fatalf("review: %d %s", review.Code, review.Body.String())
			}
			if strings.Contains(review.Body.String(), "peer-secret") || strings.Contains(review.Body.String(), "private") {
				t.Fatal("foreign decision leaked")
			}
			for _, owner := range store.owners {
				if owner != "owner" {
					t.Fatal("unscoped restriction read")
				}
			}
			if tc.authFailure != nil && len(store.owners) > 0 {
				t.Fatal("invalid credential reached repository")
			}
		})
	}
}

type restrictedAdmission struct{ calls int }

func (r *restrictedAdmission) AdmitAccountUse(context.Context, string) error {
	r.calls++
	return ports.ErrAccountRestricted
}
func TestRestrictedAccountPreservesOnlyRecoveryOperations(t *testing.T) {
	gate := &restrictedAdmission{}
	ids := []string{"get-account-restriction", "list-enforcement-notices", "get-enforcement-notice", "submit-enforcement-appeal", "revoke-session", "refresh-session", "get-me", "export-own-activities", "export-own-profile", "export-own-paths", "delete-account", "confirm-account-deletion-receipt", "deactivate-me"}
	blocked := []string{"start-path-timer", "stop-path-timer", "list-own-running-timers", "check-notification-ownership", "create-path", "update-profile", "future-protected-operation"}
	handled := 0
	handler := http.NewServeMux()
	api := humago.New(handler, huma.DefaultConfig("Admission test", "1"))
	installAccountAdmission(api, gate)
	installPolicyAdmission(api, app.App{})
	func() {
		for _, id := range append(append([]string{}, ids...), blocked...) {
			huma.Register(api, huma.Operation{OperationID: id, Method: "POST", Path: "/v1/admission-test/" + id, Security: []map[string][]string{{"oidc": {}}}}, func(context.Context, *MeInput) (*NoContentOutput, error) {
				handled++
				return &NoContentOutput{Status: 204}, nil
			})
		}
	}()
	for _, id := range append(append([]string{}, ids...), blocked...) {
		req := httptest.NewRequest("POST", "/v1/admission-test/"+id, nil)
		req.Header.Set("Authorization", "Bearer restricted")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		want := 403
		for _, exempt := range ids {
			if id == exempt {
				want = 204
			}
		}
		if res.Code != want {
			t.Fatalf("%s: %d %s", id, res.Code, res.Body.String())
		}
	}
	if handled != len(ids) || gate.calls != len(blocked) {
		t.Fatalf("handled=%d denied=%d", handled, gate.calls)
	}
}
