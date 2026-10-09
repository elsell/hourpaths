package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type policyRouteStore struct {
	owners   []string
	commands []app.PolicyRenewalCommand
	failure  error
	accepted bool
}

func (r *policyRouteStore) Current(context.Context) (ports.PolicySet, error) {
	return ports.PolicySet{Revision: 2, TermsVersion: "terms2", PrivacyPolicyVersion: "privacy1", CommunityGuidelinesVersion: "guidelines1", TermsURL: "https://app.example/terms", PrivacyPolicyURL: "https://app.example/privacy", CommunityGuidelinesURL: "https://app.example/guidelines", SupportURL: "https://app.example/support", UpdatedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}, nil
}
func (r *policyRouteStore) HasAcceptedPolicies(_ context.Context, owner string, _ identity.CurrentPolicyVersions) (bool, error) {
	r.owners = append(r.owners, owner)
	return r.accepted, r.failure
}
func (r *policyRouteStore) RenewPolicyAcceptance(_ context.Context, c app.PolicyRenewalCommand) (app.PolicyRenewalResult, error) {
	r.commands = append(r.commands, c)
	return app.PolicyRenewalResult{Policies: c.Policies, AcceptedAt: c.AcceptedAt}, r.failure
}
func TestPolicyRenewalHTTPAuthenticatesAndBindsReviewToAccount(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		for _, tc := range []struct {
			name, credential, scope string
			failure                 error
			status                  int
		}{
			{"missing", "", "api:user", app.ErrUnauthenticated, 401}, {"malformed", "Bearer malformed", "api:user", ports.ErrInvalidCredential, 401}, {"expired", "Bearer expired", "api:user", ports.ErrInvalidCredential, 401}, {"revoked", "Bearer revoked", "api:user", ports.ErrInvalidCredential, 401}, {"provider", "Bearer provider", "api:user", ports.ErrInvalidCredential, 401}, {"wrongissuer", "Bearer issuer", "api:user", ports.ErrInvalidCredential, 401}, {"wrongaudience", "Bearer audience", "api:user", ports.ErrInvalidCredential, 401}, {"provisional", "Bearer provisional", "api:onboarding", nil, 401}, {"dependency", "Bearer unavailable", "api:user", ports.ErrUnavailable, 503}, {"legitimate", "Bearer current", "api:user", nil, 200},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				received := "unset"
				store := &policyRouteStore{}
				key := bytes.Repeat([]byte{17}, 32)
				application := app.App{Auth: weekStartRouteAuth{ports.Principal{UserID: "owner", Scopes: []string{tc.scope}}, tc.failure, &received}, Users: timeZoneRouteUsers{}, PolicyAuthority: store, PolicyAcceptances: store, CursorSigningKey: key, Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}}
				token, err := shared.EncodePolicyRenewalReviewToken(key, "owner", 2, identity.CurrentPolicyVersions{TermsOfService: "terms2", PrivacyPolicy: "privacy1", CommunityGuidelines: "guidelines1"})
				if err != nil {
					t.Fatal(err)
				}
				body, _ := json.Marshal(map[string]any{"reviewToken": token, "termsAccepted": true, "privacyAcknowledged": true, "communityGuidelinesAccepted": true})
				handler, _ := New(application, nil, Options{})
				request := httptest.NewRequest(method, "/v1/me/policies?userId=someone-else", strings.NewReader(string(body)))
				request.Header.Set("Authorization", tc.credential)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", "policy-renew-route-01")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != tc.status || received != tc.credential {
					t.Fatalf("response=%d %s auth=%q", response.Code, response.Body.String(), received)
				}
				if tc.status != 200 {
					if len(store.owners)+len(store.commands) != 0 {
						t.Fatal("invalid credentials reached persistence")
					}
					return
				}
				if method == "GET" {
					if len(store.owners) != 1 || store.owners[0] != "owner" {
						t.Fatal("unscoped read")
					}
				} else if len(store.commands) != 1 || store.commands[0].ActorUserID != "owner" {
					t.Fatal("unscoped acceptance")
				}
			})
		}
	}
}

func TestPolicyAdmissionProtectsIndependentDomainsAndPreservesReviewAndStop(t *testing.T) {
	received := ""
	store := &policyRouteStore{}
	application := app.App{Auth: weekStartRouteAuth{ports.Principal{UserID: "owner", Scopes: []string{"api:user"}}, nil, &received}, Users: timeZoneRouteUsers{}, PolicyAuthority: store, PolicyAcceptances: store, CursorSigningKey: bytes.Repeat([]byte{17}, 32), Audits: timeZoneRouteAudits{}, AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}}
	handled := 0
	handler, _ := New(application, nil, Options{DomainRegistrations: []func(huma.API){func(api huma.API) {
		for _, entry := range []struct{ id, path string }{{"independent-domain-write", "/v1/policy-test/write"}, {"stop-path-timer", "/v1/policy-test/stop"}} {
			huma.Register(api, huma.Operation{OperationID: entry.id, Method: "POST", Path: entry.path, Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *MeInput) (*NoContentOutput, error) {
				handled++
				return &NoContentOutput{Status: 204}, nil
			})
		}
	}}})
	request := func(method, path string) int {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("Authorization", "Bearer session")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	if status := request("POST", "/v1/policy-test/write"); status != 428 || handled != 0 {
		t.Fatalf("restricted=%d handled=%d", status, handled)
	}
	if status := request("GET", "/v1/me/policies"); status != 200 {
		t.Fatalf("review=%d", status)
	}
	if status := request("POST", "/v1/policy-test/stop"); status != 204 || handled != 1 {
		t.Fatalf("stop=%d handled=%d", status, handled)
	}
	store.accepted = true
	if status := request("POST", "/v1/policy-test/write"); status != 204 || handled != 2 {
		t.Fatalf("accepted=%d handled=%d", status, handled)
	}
	store.failure = ports.ErrUnavailable
	if status := request("POST", "/v1/policy-test/write"); status != 503 || handled != 2 {
		t.Fatalf("failure=%d handled=%d", status, handled)
	}
}
