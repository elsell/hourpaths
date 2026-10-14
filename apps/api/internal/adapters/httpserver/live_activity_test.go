package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/liveactivity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type liveActivityRouteAuth struct{}

func (liveActivityRouteAuth) Authenticate(_ context.Context, header string) (ports.Principal, error) {
	if header == "Bearer unavailable" {
		return ports.Principal{}, ports.ErrUnavailable
	}
	if header == "Bearer onboarding" {
		return ports.Principal{UserID: "alice", Scopes: []string{"onboarding"}}, nil
	}
	if header != "Bearer alice" && header != "Bearer bob" {
		return ports.Principal{}, app.ErrUnauthenticated
	}
	return ports.Principal{UserID: strings.TrimPrefix(header, "Bearer "), Scopes: []string{"api:user"}}, nil
}

type liveActivityRouteStore struct {
	rows   map[string]liveactivity.Registration
	events []audit.Event
}

func (r *liveActivityRouteStore) UpsertLiveActivity(_ context.Context, value liveactivity.Registration, event audit.Event) error {
	if old, ok := r.rows[value.ID]; ok && old.OwnerUserID != value.OwnerUserID {
		return ports.ErrNotFound
	}
	r.rows[value.ID] = value
	r.events = append(r.events, event)
	return nil
}
func (r *liveActivityRouteStore) RevokeLiveActivity(_ context.Context, owner, id, lease string, event audit.Event) error {
	value, ok := r.rows[id]
	if !ok || value.OwnerUserID != owner || value.AccountLease != lease {
		return ports.ErrNotFound
	}
	delete(r.rows, id)
	r.events = append(r.events, event)
	return nil
}
func (r *liveActivityRouteStore) AppendAuditEvent(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}
func (r *liveActivityRouteStore) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}

func TestLiveActivityRoutesAuthenticateScopeAndDoNotDiscloseTokens(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	store := &liveActivityRouteStore{rows: map[string]liveactivity.Registration{}}
	handler, api := newHTTPTestServer(app.App{Auth: liveActivityRouteAuth{}, LiveActivities: store, Audits: store, Clock: pushRouteClock{now}, AuditRateLimiter: pushRouteLimiter{}}, nil, Options{DisableDocs: true})
	token := strings.Repeat("ab", 32)
	body, _ := json.Marshal(map[string]any{"accountLease": "lease", "token": token, "expiresAt": now.Add(time.Hour), "boundaries": []time.Time{now.Add(30 * time.Minute)}})
	request := func(method, auth string, payload []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/v1/live-activities/activity", bytes.NewReader(payload))
		req.Header.Set("Authorization", auth)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if strings.Contains(response.Body.String(), token) {
			t.Fatal("response disclosed device token")
		}
		return response
	}
	for _, auth := range []string{"", "malformed", "Bearer expired", "Bearer wrong-issuer", "Bearer wrong-audience", "Bearer onboarding"} {
		if response := request(http.MethodPut, auth, body); response.Code != http.StatusUnauthorized {
			t.Fatalf("auth=%q status=%d", auth, response.Code)
		}
	}
	if len(store.rows) != 0 {
		t.Fatal("unauthenticated registration reached persistence")
	}
	if response := request(http.MethodPut, "Bearer unavailable", body); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("dependency failure status=%d", response.Code)
	}
	if response := request(http.MethodPut, "Bearer alice", body); response.Code != http.StatusNoContent {
		t.Fatalf("register status=%d %s", response.Code, response.Body.String())
	}
	if store.rows["activity"].OwnerUserID != "alice" {
		t.Fatal("owner not bound to authenticated account")
	}
	if response := request(http.MethodPut, "Bearer bob", body); response.Code != http.StatusNotFound {
		t.Fatalf("cross-owner replace status=%d", response.Code)
	}
	revoke := []byte(`{"accountLease":"lease"}`)
	if response := request(http.MethodDelete, "Bearer bob", revoke); response.Code != http.StatusNotFound {
		t.Fatalf("cross-owner revoke status=%d", response.Code)
	}
	if len(store.rows) != 1 {
		t.Fatal("cross-owner request removed registration")
	}
	if response := request(http.MethodDelete, "Bearer alice", revoke); response.Code != http.StatusNoContent {
		t.Fatalf("revoke status=%d", response.Code)
	}
	if len(store.rows) != 0 {
		t.Fatal("registration remained after revoke")
	}
	route := api.OpenAPI().Paths["/v1/live-activities/{activityId}"]
	if route == nil || route.Put == nil || route.Delete == nil {
		t.Fatal("missing generated contract")
	}
	if !accountRestrictionExempt("revoke-live-activity") || !policyReviewExempt("revoke-live-activity") || accountRestrictionExempt("upsert-live-activity") {
		t.Fatal("cleanup/admission policy mismatch")
	}
}
