package gormstore

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/auditlimit"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	socialroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/social/routes"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/sessionauth"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresTimerSubscriptionsHTTPAuthenticationAndAccountIsolation(t *testing.T) {
	for _, scope := range []string{"person", "path"} {
		t.Run(scope, func(t *testing.T) { testTimerSubscriptionHTTP(t, scope) })
	}
}

type subscriptionAuthorizer struct {
	allowed bool
	err     error
}

func (a subscriptionAuthorizer) Check(context.Context, string, string, string, string) (bool, error) {
	return a.allowed, a.err
}
func (a subscriptionAuthorizer) WriteRelationship(context.Context, string, string, string, string, string) error {
	return ports.ErrUnavailable
}
func (a subscriptionAuthorizer) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return ports.ErrUnavailable
}
func testTimerSubscriptionHTTP(t *testing.T, scope string) {
	f := newNudgeNotificationChannelFixture(t, "timersubhttp")
	clock := &notificationHTTPClock{now: f.now}
	auth := sessionauth.New(f.runtime, clock)
	issue := func(owner string) string {
		digest := sha256.Sum256([]byte(newTestID()))
		token, err := auth.CreateSession(context.Background(), owner, []string{"api:user"}, digest[:], f.now.Add(time.Hour), f.now.Add(time.Hour), audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.SessionCreated, TargetType: "user", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: f.now})
		if err != nil {
			t.Fatal(err)
		}
		return "Bearer " + token
	}
	actor, other := issue(f.actor.ID), issue(f.other.ID)
	target := socialRelationshipTestUser(t, f.migration.DB, "target", identity.ProfileVisibilityPublic, f.now)
	for _, follower := range []string{f.actor.ID, f.other.ID} {
		if err := f.migration.DB.Create(&socialFollowModel{FollowerUserID: follower, FollowingUserID: target.ID, CreatedAt: f.now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	subject := target.ID
	if scope == "path" {
		subject = "timer-subscription-path-" + newTestID()
		if err := f.migration.DB.Table("path_models").Create(map[string]any{"id": subject, "owner_user_id": f.actor.ID, "name": "Shared", "visibility": "private", "created_at": f.now, "updated_at": f.now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.migration.DB.Table("path_membership_models").Create(map[string]any{"path_id": subject, "user_id": f.other.ID, "role": "participant", "joined_at": f.now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	enabled := scope == "person"
	updateBody := fmt.Sprintf(`{"enabled":%t,"expectedRevision":0}`, enabled)
	service := socialapp.New(socialapp.Dependencies{Auth: auth, Authorizer: subscriptionAuthorizer{allowed: true}, Clock: clock, Audits: f.runtime, AuditRateLimiter: auditlimit.New(100, time.Minute, 20), TimerSubscriptions: NewTimerSubscriptionRepository(f.runtime.DB)})
	handler, _ := newPolicyAcceptedHTTPTestServer(platformapp.App{}, nil, httpserver.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { socialroutes.Register(api, service) }}})
	request := func(method, path, authorization, body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", authorization)
		if method == http.MethodPut {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", key)
		}
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		return out
	}
	path := "/v1/me/timer-subscriptions/" + scope + "/" + subject
	for _, credential := range []string{"", "Bearer malformed", "Basic " + strings.TrimPrefix(actor, "Bearer "), "Bearer eyJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJ3cm9uZyIsImF1ZCI6Indyb25nIn0.signature"} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			target := path
			result := request(method, target, credential, updateBody, "channel-http-invalid")
			if result.Code != http.StatusUnauthorized {
				t.Fatalf("invalid credential %s: %d %s", method, result.Code, result.Body)
			}
		}
	}
	result := request(http.MethodPut, path, actor, updateBody, "channel-http-update")
	if result.Code != http.StatusOK {
		t.Fatalf("update %d %s", result.Code, result.Body)
	}
	for _, owner := range []struct {
		token   string
		enabled bool
	}{{actor, enabled}, {other, !enabled}} {
		result = request(http.MethodGet, path, owner.token, "", "")
		if result.Code != http.StatusOK {
			t.Fatalf("read %d %s", result.Code, result.Body)
		}
		var body struct {
			Data struct {
				Enabled  bool
				Revision int64
			}
		}
		if err := json.Unmarshal(result.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Data.Enabled != owner.enabled {
			t.Fatalf("cross-account preference: %+v", body.Data)
		}

	}
	result = request(http.MethodPut, path, other, `{"enabled":false,"expectedRevision":0,"userId":"`+f.actor.ID+`"}`, "channel-http-owner-input")
	if result.Code != http.StatusUnprocessableEntity {
		t.Fatalf("client owner accepted: %d %s", result.Code, result.Body)
	}
	result = request(http.MethodPut, path, actor, updateBody, "channel-http-stale")
	if result.Code != http.StatusConflict {
		t.Fatalf("stale update: %d %s", result.Code, result.Body)
	}
	service.Auth = sessionauth.NewWithAccountAccess(f.runtime, clock, func(context.Context, string) error { return ports.ErrUnavailable })
	result = request(http.MethodGet, path, actor, "", "")
	if result.Code != http.StatusServiceUnavailable {
		t.Fatalf("dependency failure: %d %s", result.Code, result.Body)
	}
	service.Auth = auth
	if scope == "path" {
		for _, permission := range []struct {
			authorizer subscriptionAuthorizer
			status     int
		}{{subscriptionAuthorizer{}, http.StatusNotFound}, {subscriptionAuthorizer{err: ports.ErrUnavailable}, http.StatusServiceUnavailable}} {
			service.Authorizer = permission.authorizer
			for _, method := range []string{http.MethodGet, http.MethodPut} {
				response := request(method, path, actor, updateBody, "subscription-no-permission")
				if response.Code != permission.status {
					t.Fatalf("authorization %s: %d %s", method, response.Code, response.Body)
				}
			}
		}
		service.Authorizer = subscriptionAuthorizer{allowed: true}
	}
	clock.now = f.now.Add(time.Hour)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		target := path
		result = request(method, target, actor, `{"enabled":true,"expectedRevision":1}`, "channel-http-expired")
		if result.Code != http.StatusUnauthorized {
			t.Fatalf("expired %s: %d %s", method, result.Code, result.Body)
		}
	}
}
