package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type onboardingHTTPAuth struct {
	principal ports.Principal
	err       error
}

func (a onboardingHTTPAuth) Authenticate(context.Context, string) (ports.Principal, error) {
	return a.principal, a.err
}

type onboardingHTTPUsers struct {
	provisional identity.User
}

func (u onboardingHTTPUsers) ResolveOrCreate(context.Context, ports.Claims, audit.Event, audit.Event, audit.Event, bool) (identity.User, error) {
	return identity.User{}, nil
}
func (u onboardingHTTPUsers) GetUser(context.Context, string) (identity.User, error) {
	return identity.User{}, ports.ErrNotFound
}
func (u onboardingHTTPUsers) GetProvisionalUser(context.Context, string) (identity.User, error) {
	return u.provisional, nil
}
func (onboardingHTTPUsers) DisableUser(context.Context, string, audit.Event) error { return nil }

type onboardingHTTPAudits struct{}

func (onboardingHTTPAudits) AppendAuditEvent(context.Context, audit.Event) error { return nil }
func (onboardingHTTPAudits) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}

type onboardingHTTPRecoveryDeclines struct {
	userID string
}

type onboardingHTTPUsernameSuggestions struct{}

func (onboardingHTTPUsernameSuggestions) SuggestAvailableUsername(context.Context, string) (string, error) {
	return "provider.seed", nil
}

type onboardingHTTPPolicyAuthority struct{ policySet ports.PolicySet }

func (a onboardingHTTPPolicyAuthority) Current(context.Context) (ports.PolicySet, error) {
	return a.policySet, nil
}

func onboardingHTTPPolicySet(versions identity.CurrentPolicyVersions) ports.PolicySet {
	return ports.PolicySet{
		Revision: 7, TermsVersion: versions.TermsOfService, PrivacyPolicyVersion: versions.PrivacyPolicy,
		CommunityGuidelinesVersion: versions.CommunityGuidelines,
		TermsURL:                   "https://app.example/legal/terms", PrivacyPolicyURL: "https://app.example/legal/privacy",
		CommunityGuidelinesURL: "https://app.example/community-guidelines", SupportURL: "https://app.example/support",
		UpdatedAt: time.Date(2026, 7, 21, 15, 0, 0, 0, time.UTC),
	}
}

type onboardingHTTPActivator struct {
	authorization string
	activation    identity.OnboardingActivation
	expiresAt     time.Time
}

func (a *onboardingHTTPActivator) ActivateOnboarding(_ context.Context, authorization string, activation identity.OnboardingActivation, expiresAt time.Time, _, _, _ audit.Event) (string, time.Time, error) {
	a.authorization, a.activation, a.expiresAt = authorization, activation, expiresAt
	return "active-session", expiresAt.Add(-time.Minute), nil
}

func (r *onboardingHTTPRecoveryDeclines) DeclineDuplicateEmailRecovery(_ context.Context, userID string, _ audit.Event) error {
	r.userID = userID
	return nil
}

func TestOnboardingRouteReturnsPrivateProviderSeedsToTheRestrictedSession(t *testing.T) {
	versions := identity.CurrentPolicyVersions{TermsOfService: "terms-v1", PrivacyPolicy: "privacy-v1", CommunityGuidelines: "guidelines-v1"}
	application := app.App{
		Auth: onboardingHTTPAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
		Users: onboardingHTTPUsers{provisional: identity.User{
			ID: "provisional-owner", Email: "private@example.com", DisplayName: "Provider Seed", Status: identity.StatusProvisional,
		}},
		Audits:              onboardingHTTPAudits{},
		AuditRateLimiter:    docsLimiter{},
		Clock:               docsClock{now: time.Date(2026, 7, 21, 16, 0, 0, 0, time.UTC)},
		UsernameSuggestions: onboardingHTTPUsernameSuggestions{},
		PolicyAuthority:     onboardingHTTPPolicyAuthority{policySet: onboardingHTTPPolicySet(versions)},
		CursorSigningKey:    []byte("0123456789abcdef0123456789abcdef"),
	}
	handler, api := New(application, nil, Options{DisableDocs: true})
	request := httptest.NewRequest(http.MethodGet, "/v1/onboarding", nil)
	request.Header.Set("Authorization", "Bearer onboarding-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("onboarding profile status = %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			Email              string `json:"email"`
			DisplayName        string `json:"displayName"`
			UsernameSuggestion string `json:"usernameSuggestion"`
			PolicyReviewToken  string `json:"policyReviewToken"`
			Policies           struct {
				TermsOfService      struct{ Version, URL string } `json:"termsOfService"`
				PrivacyPolicy       struct{ Version, URL string } `json:"privacyPolicy"`
				CommunityGuidelines struct{ Version, URL string } `json:"communityGuidelines"`
				SupportURL          string                        `json:"supportUrl"`
			} `json:"policies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Email != "private@example.com" || body.Data.DisplayName != "Provider Seed" || body.Data.UsernameSuggestion != "provider.seed" || body.Data.PolicyReviewToken == "" {
		t.Fatalf("onboarding response = %+v", body.Data)
	}
	if body.Data.Policies.TermsOfService.Version != "terms-v1" || body.Data.Policies.TermsOfService.URL != "https://app.example/legal/terms" || body.Data.Policies.PrivacyPolicy.Version != "privacy-v1" || body.Data.Policies.CommunityGuidelines.Version != "guidelines-v1" || body.Data.Policies.SupportURL != "https://app.example/support" {
		t.Fatalf("onboarding policies = %+v", body.Data.Policies)
	}
	operation := api.OpenAPI().Paths["/v1/onboarding"].Get
	if operation == nil || operation.OperationID != "get-onboarding-profile" {
		t.Fatalf("onboarding OpenAPI operation = %+v", operation)
	}
}

func TestOnboardingActivationRouteUsesOnlyClientAssertionsAndReturnsTheActiveSession(t *testing.T) {
	now := time.Date(2026, 7, 21, 19, 0, 0, 0, time.UTC)
	versions := identity.CurrentPolicyVersions{TermsOfService: "terms-v1", PrivacyPolicy: "privacy-v1", CommunityGuidelines: "guidelines-v1"}
	key := []byte("0123456789abcdef0123456789abcdef")
	policyReviewToken, err := shared.EncodePolicyReviewToken(key, "provisional-owner", 7, versions)
	if err != nil {
		t.Fatal(err)
	}
	activator := &onboardingHTTPActivator{}
	application := app.App{
		Auth:                onboardingHTTPAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
		OnboardingActivator: activator, PolicyAuthority: onboardingHTTPPolicyAuthority{policySet: onboardingHTTPPolicySet(versions)}, CursorSigningKey: key,
		AuditRateLimiter: docsLimiter{}, Clock: docsClock{now: now}, SessionTTL: time.Hour,
	}
	body := `{"username":"reviewed.user","displayName":"Reviewed User","profileVisibility":"private","timeZone":"America/New_York","firstDayOfWeek":1,"atLeast16":true,"termsAccepted":true,"privacyAcknowledged":true,"communityGuidelinesAccepted":true,"policyReviewToken":"` + policyReviewToken + `"}`
	handler, api := New(application, nil, Options{DisableDocs: true})
	request := httptest.NewRequest(http.MethodPost, "/v1/onboarding/activation", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer onboarding-session")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("activation response status=%d cache=%q body=%s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var output struct {
		Data SessionExchangeData `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Data.Token != "active-session" || output.Data.NextAction != "home" || !output.Data.ExpiresAt.Equal(now.Add(59*time.Minute)) {
		t.Fatalf("activation response = %+v", output.Data)
	}
	if activator.authorization != "Bearer onboarding-session" || activator.activation.UserID != "provisional-owner" || activator.activation.Username != "reviewed.user" || activator.activation.PolicyAcceptance.TermsOfServiceAcceptedVersion != "terms-v1" {
		t.Fatalf("activation boundary = auth %q aggregate %+v", activator.authorization, activator.activation)
	}
	operation := api.OpenAPI().Paths["/v1/onboarding/activation"].Post
	if operation == nil || operation.OperationID != "activate-onboarding" || len(operation.Security) != 1 {
		t.Fatalf("activation OpenAPI operation = %+v", operation)
	}
}

func TestOnboardingRouteRejectsNonOnboardingCredentialsWithoutLeakingProfileSeeds(t *testing.T) {
	for _, test := range []struct {
		name      string
		principal ports.Principal
		err       error
	}{
		{name: "ordinary application session", principal: ports.Principal{UserID: "active-user", Scopes: []string{"api:user"}}},
		{name: "mixed scopes", principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding", "api:user"}}},
		{name: "malformed credential", err: ports.ErrInvalidCredential},
	} {
		t.Run(test.name, func(t *testing.T) {
			application := app.App{
				Auth: onboardingHTTPAuth{principal: test.principal, err: test.err},
				Users: onboardingHTTPUsers{provisional: identity.User{
					ID: "provisional-owner", Email: "private@example.com", DisplayName: "Provider Seed", Status: identity.StatusProvisional,
				}},
				Audits:           onboardingHTTPAudits{},
				AuditRateLimiter: docsLimiter{},
				Clock:            docsClock{now: time.Now().UTC()},
			}
			handler, _ := New(application, nil, Options{DisableDocs: true})
			request := httptest.NewRequest(http.MethodGet, "/v1/onboarding", nil)
			request.Header.Set("Authorization", "Bearer rejected")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("onboarding rejection status = %d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "private@example.com") || strings.Contains(response.Body.String(), "Provider Seed") {
				t.Fatalf("onboarding rejection leaked private profile: %s", response.Body.String())
			}
		})
	}
}

func TestDuplicateEmailRecoveryDeclineRouteContinuesOnboardingWithoutContent(t *testing.T) {
	declines := &onboardingHTTPRecoveryDeclines{}
	application := app.App{
		Auth: onboardingHTTPAuth{principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding"}}},
		Users: onboardingHTTPUsers{provisional: identity.User{
			ID: "provisional-owner", Email: "private@example.com", Status: identity.StatusProvisional,
		}},
		DuplicateAccountRecoveryDeclines: declines,
		AuditRateLimiter:                 docsLimiter{},
		Clock:                            docsClock{now: time.Date(2026, 7, 21, 18, 30, 0, 0, time.UTC)},
	}
	handler, api := New(application, nil, Options{DisableDocs: true})
	request := httptest.NewRequest(http.MethodPost, "/v1/onboarding/duplicate-email-recovery/decline", nil)
	request.Header.Set("Authorization", "Bearer onboarding-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("decline response status=%d body=%s", response.Code, response.Body.String())
	}
	if declines.userID != "provisional-owner" {
		t.Fatalf("decline persisted for %q", declines.userID)
	}
	operation := api.OpenAPI().Paths["/v1/onboarding/duplicate-email-recovery/decline"].Post
	if operation == nil || operation.OperationID != "decline-duplicate-email-recovery" || len(operation.Security) != 1 {
		t.Fatalf("decline OpenAPI operation = %+v", operation)
	}
}

func TestDuplicateEmailRecoveryDeclineRouteRejectsOtherCredentialsWithoutDisclosure(t *testing.T) {
	for _, test := range []struct {
		name       string
		principal  ports.Principal
		err        error
		wantStatus int
	}{
		{name: "active application session", principal: ports.Principal{UserID: "active-user", Scopes: []string{"api:user"}}, wantStatus: http.StatusUnauthorized},
		{name: "mixed scopes", principal: ports.Principal{UserID: "provisional-owner", Scopes: []string{"api:onboarding", "api:user"}}, wantStatus: http.StatusUnauthorized},
		{name: "malformed credential", err: ports.ErrInvalidCredential, wantStatus: http.StatusUnauthorized},
		{name: "authentication dependency failure", err: errors.New("session repository unavailable"), wantStatus: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			declines := &onboardingHTTPRecoveryDeclines{}
			application := app.App{
				Auth:                             onboardingHTTPAuth{principal: test.principal, err: test.err},
				Users:                            onboardingHTTPUsers{provisional: identity.User{ID: "provisional-owner", Email: "private@example.com", Status: identity.StatusProvisional}},
				DuplicateAccountRecoveryDeclines: declines,
				Audits:                           onboardingHTTPAudits{},
				AuditRateLimiter:                 docsLimiter{},
				Clock:                            docsClock{now: time.Now().UTC()},
			}
			handler, _ := New(application, nil, Options{DisableDocs: true})
			request := httptest.NewRequest(http.MethodPost, "/v1/onboarding/duplicate-email-recovery/decline", nil)
			request.Header.Set("Authorization", "Bearer rejected")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("decline rejection status=%d body=%s", response.Code, response.Body.String())
			}
			if declines.userID != "" || strings.Contains(response.Body.String(), "private@example.com") {
				t.Fatalf("decline rejection reached owner data: persisted=%q body=%s", declines.userID, response.Body.String())
			}
		})
	}
}
