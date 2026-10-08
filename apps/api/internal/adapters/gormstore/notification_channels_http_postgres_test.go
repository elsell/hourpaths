package gormstore

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type notificationHTTPClock struct{ now time.Time }

func (c *notificationHTTPClock) Now() time.Time { return c.now }

func TestPostgresNotificationChannelsHTTPAuthenticationAndAccountIsolation(t *testing.T) {
	f := newNudgeNotificationChannelFixture(t, "channelhttp")
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
	service := socialapp.New(socialapp.Dependencies{Auth: auth, Clock: clock, Audits: f.runtime, AuditRateLimiter: auditlimit.New(100, time.Minute, 20), NotificationChannels: f.repository, NudgeNotificationChannels: f.repository})
	handler, _ := httpserver.New(platformapp.App{}, nil, httpserver.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { socialroutes.Register(api, service) }}})
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
	path := "/v1/me/notification-channels"
	for _, credential := range []string{"", "Bearer malformed", "Basic " + strings.TrimPrefix(actor, "Bearer "), "Bearer eyJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJ3cm9uZyIsImF1ZCI6Indyb25nIn0.signature"} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			target := path
			if method == http.MethodPut {
				target += "/comments"
			}
			result := request(method, target, credential, `{"enabled":false,"expectedRevision":0}`, "channel-http-invalid")
			if result.Code != http.StatusUnauthorized {
				t.Fatalf("invalid credential %s: %d %s", method, result.Code, result.Body)
			}
		}
	}
	result := request(http.MethodPut, path+"/comments", actor, `{"enabled":false,"expectedRevision":0}`, "channel-http-update")
	if result.Code != http.StatusOK {
		t.Fatalf("update %d %s", result.Code, result.Body)
	}
	for _, owner := range []struct {
		token   string
		enabled bool
	}{{actor, false}, {other, true}} {
		result = request(http.MethodGet, path, owner.token, "", "")
		if result.Code != http.StatusOK {
			t.Fatalf("read %d %s", result.Code, result.Body)
		}
		var body struct {
			Data []struct {
				Channel string
				Enabled bool
			}
		}
		if err := json.Unmarshal(result.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Data) != 10 {
			t.Fatalf("catalog size %d", len(body.Data))
		}
		for _, row := range body.Data {
			if row.Channel == "comments" && row.Enabled != owner.enabled {
				t.Fatalf("cross-account preference: %+v", row)
			}
		}
	}
	result = request(http.MethodPut, path+"/comments", other, `{"enabled":false,"expectedRevision":0,"userId":"`+f.actor.ID+`"}`, "channel-http-owner-input")
	if result.Code != http.StatusUnprocessableEntity {
		t.Fatalf("client owner accepted: %d %s", result.Code, result.Body)
	}
	result = request(http.MethodPut, path+"/comments", actor, `{"enabled":true,"expectedRevision":0}`, "channel-http-stale")
	if result.Code != http.StatusConflict {
		t.Fatalf("stale update: %d %s", result.Code, result.Body)
	}
	service.Auth = sessionauth.NewWithAccountAccess(f.runtime, clock, func(context.Context, string) error { return ports.ErrUnavailable })
	result = request(http.MethodGet, path, actor, "", "")
	if result.Code != http.StatusServiceUnavailable {
		t.Fatalf("dependency failure: %d %s", result.Code, result.Body)
	}
	service.Auth = auth
	clock.now = f.now.Add(time.Hour)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		target := path
		if method == http.MethodPut {
			target += "/comments"
		}
		result = request(method, target, actor, `{"enabled":true,"expectedRevision":1}`, "channel-http-expired")
		if result.Code != http.StatusUnauthorized {
			t.Fatalf("expired %s: %d %s", method, result.Code, result.Body)
		}
	}
}
