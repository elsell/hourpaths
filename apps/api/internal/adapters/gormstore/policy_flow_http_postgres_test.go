package gormstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/auditlimit"
	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	activityroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/routes"
	pathroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/routes"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/sessionauth"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresPolicyReviewKeepsSessionAndStopsTimerBeforeResuming(t *testing.T) {
	f := newNudgeNotificationChannelFixture(t, "policyflow")
	for _, owner := range []string{f.actor.ID, f.other.ID} {
		if err := f.migration.DB.Table("user_preference_models").Create(map[string]any{"user_id": owner, "first_day_of_week": 1, "current_time_zone": "Etc/UTC", "created_at": f.now, "updated_at": f.now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.migration.DB.Table("user_time_zone_history_models").Create(map[string]any{"user_id": owner, "effective_at": f.now, "time_zone": "Etc/UTC"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	clock := &notificationHTTPClock{now: time.Now().UTC().Truncate(time.Microsecond)}
	if _, err := f.migration.Current(ctx); errors.Is(err, ports.ErrNotFound) {
		if _, err = f.migration.Publish(ctx, policySet(1, "policy-flow")); err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	auth := sessionauth.New(f.runtime, clock)
	issue := func(owner string) string {
		digest := sha256.Sum256([]byte(newTestID()))
		token, err := auth.CreateSession(ctx, owner, []string{"api:user"}, digest[:], clock.now.Add(time.Hour), clock.now.Add(time.Hour), audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.SessionCreated, TargetType: "user", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: clock.now})
		if err != nil {
			t.Fatal(err)
		}
		return "Bearer " + token
	}
	actor, peer := issue(f.actor.ID), issue(f.other.ID)
	pathID, timerID := "policy-path-"+newTestID(), "policy-timer-"+newTestID()
	if err := f.migration.DB.Table("path_models").Create(map[string]any{"id": pathID, "owner_user_id": f.actor.ID, "name": "Guitar", "visibility": "private", "created_at": f.now, "updated_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("path_membership_models").Create(map[string]any{"path_id": pathID, "user_id": f.actor.ID, "role": "participant", "joined_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("running_timer_models").Create(map[string]any{"id": timerID, "path_id": pathID, "participant_id": f.actor.ID, "started_at": clock.now.Add(-time.Minute), "occurrence_time_zone": "Etc/UTC"}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.migration.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{}{}) })
	limiter := auditlimit.New(200, time.Minute, 20)
	signing := bytes.Repeat([]byte{42}, 32)
	app := application.App{AccountExport: f.runtime, Auth: auth, Users: f.runtime, PolicyAuthority: f.runtime, PolicyAcceptances: f.runtime, Audits: f.runtime, AuditRateLimiter: limiter, Clock: clock, CursorSigningKey: signing}
	activity := activityapp.New(activityapp.Dependencies{Auth: auth, Profiles: f.runtime, Repository: activitystore.New(f.runtime.DB), Authorizer: subscriptionAuthorizer{allowed: true}, Audits: f.runtime, AuditRateLimiter: limiter, Clock: clock, NewID: newTestID, CursorSigningKey: signing})
	paths := pathapp.New(pathapp.Dependencies{Auth: auth, Profiles: f.runtime, Repository: pathstore.New(f.runtime.DB), Authorizer: subscriptionAuthorizer{allowed: true}, Audits: f.runtime, AuditRateLimiter: limiter, Clock: clock, NewID: newTestID, CursorSigningKey: signing})
	handler, _ := httpserver.New(app, nil, httpserver.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { activityroutes.Register(api, activity); pathroutes.Register(api, paths) }}})
	request := func(method, path, token, body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", token)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		return out
	}
	requireStatus := func(out *httptest.ResponseRecorder, status int) {
		t.Helper()
		if out.Code != status {
			t.Fatalf("status=%d expected=%d body=%s", out.Code, status, out.Body)
		}
	}
	for _, entry := range []struct{ token, owner string }{{actor, f.actor.ID}, {peer, f.other.ID}} {
		result := request("GET", "/v1/me/export/profile?userId=another-account", entry.token, "", "")
		requireStatus(result, 200)
		var exported struct{ Data struct{ UserID string } }
		if err := json.Unmarshal(result.Body.Bytes(), &exported); err != nil || exported.Data.UserID != entry.owner {
			t.Fatalf("export owner=%+v err=%v", exported, err)
		}
	}
	requireStatus(request("GET", "/v1/me/export/profile", "Bearer malformed", "", ""), 401)
	exportedPaths := request("GET", "/v1/me/export/paths", actor, "", "")
	requireStatus(exportedPaths, 200)
	var pathsExport struct {
		Data []struct{ Path struct{ ID, Name string } }
	}
	if err := json.Unmarshal(exportedPaths.Body.Bytes(), &pathsExport); err != nil || len(pathsExport.Data) != 1 || pathsExport.Data[0].Path.ID != pathID || pathsExport.Data[0].Path.Name != "Guitar" {
		t.Fatalf("path export=%s err=%v", exportedPaths.Body, err)
	}
	timerPath := "/v1/paths/" + pathID + "/timer"
	requireStatus(request("POST", timerPath, actor, "", "policy-start-before"), 428)
	for _, item := range []struct {
		token string
		count int
	}{{actor, 1}, {peer, 0}} {
		response := request("GET", "/v1/me/running-timers", item.token, "", "")
		requireStatus(response, 200)
		var body struct{ Data []struct{ ID string } }
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Data) != item.count {
			t.Fatalf("timer owner isolation: %+v", body)
		}
	}
	requireStatus(request("DELETE", timerPath+"/"+timerID, actor, "", "policy-stop-during-review"), 200)
	var count int64
	if err := f.runtime.DB.Table("recorded_activity_models").Where("participant_id = ? AND path_id = ?", f.actor.ID, pathID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("durable stop count=%d err=%v", count, err)
	}
	if err := f.migration.DB.Table("path_membership_models").Create(map[string]any{"path_id": pathID, "user_id": f.other.ID, "role": "participant", "joined_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("recorded_activity_models").Create(map[string]any{"id": "peer-entry-" + newTestID(), "path_id": pathID, "participant_id": f.other.ID, "started_at": clock.now.Add(-time.Hour), "ended_at": clock.now.Add(-time.Minute), "occurrence_time_zone": "Etc/UTC", "note": "private peer note", "created_at": clock.now, "updated_at": clock.now}).Error; err != nil {
		t.Fatal(err)
	}
	exportedActivity := request("GET", "/v1/me/export/paths/"+pathID+"/activities?participantId="+f.other.ID, actor, "", "")
	requireStatus(exportedActivity, 200)
	var activityExport struct {
		Data []struct {
			Activity struct{ ParticipantID string }
		}
	}
	if err := json.Unmarshal(exportedActivity.Body.Bytes(), &activityExport); err != nil || len(activityExport.Data) != 1 || activityExport.Data[0].Activity.ParticipantID != f.actor.ID || strings.Contains(exportedActivity.Body.String(), "private peer note") {
		t.Fatalf("activity export leaked or omitted own data: %s err=%v", exportedActivity.Body, err)
	}
	response := request("GET", "/v1/me/policies", actor, "", "")
	requireStatus(response, 200)
	var review struct {
		Data struct {
			Required    bool
			ReviewToken string
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &review); err != nil || !review.Data.Required {
		t.Fatalf("review=%+v err=%v", review, err)
	}
	body, _ := json.Marshal(map[string]any{"reviewToken": review.Data.ReviewToken, "termsAccepted": true, "privacyAcknowledged": true, "communityGuidelinesAccepted": true})
	requireStatus(request("POST", "/v1/me/policies", peer, string(body), "policy-foreign-review"), 400)
	requireStatus(request("POST", "/v1/me/policies", actor, string(body), "policy-accept-current"), 200)
	// The same opaque session resumes; the other account remains gated.
	requireStatus(request("POST", timerPath, actor, "", "policy-start-after"), 200)
	requireStatus(request("POST", timerPath, peer, "", "policy-peer-still-gated"), 428)
}
