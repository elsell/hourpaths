package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestNudgePreferenceRoutesProjectRevisionedViewerPreference(t *testing.T) {
	updatedAt := time.Date(2026, 8, 5, 12, 30, 0, 0, time.UTC)
	service := &controlledService{nudgePreference: application.NudgeAudiencePreference{
		PathID: "path-1", UserID: "viewer", Audience: domain.NudgeAudienceFollowers, Revision: 4, UpdatedAt: updatedAt,
	}}

	request := httptest.NewRequest(http.MethodGet, "/v1/paths/path-1/nudge-preference", nil)
	request.Header.Set("Authorization", "Bearer viewer")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	assertNudgePreferenceResponse(t, response, http.StatusOK, "followers", 4)

	request = httptest.NewRequest(http.MethodPut, "/v1/paths/path-1/nudge-preference", bytes.NewBufferString(`{"audience":"followers","expectedRevision":3}`))
	request.Header.Set("Authorization", "Bearer viewer")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "nudge-preference-key")
	response = httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	assertNudgePreferenceResponse(t, response, http.StatusOK, "followers", 4)
	if service.authorization != "Bearer viewer" || service.nudgePathID != "path-1" || service.nudgeKey != "nudge-preference-key" || service.nudgeExpectedRevision != 3 || service.nudgeAudience != domain.NudgeAudienceFollowers {
		t.Fatalf("service received %+v", service)
	}
}

func TestNudgePreferenceRouteProjectsTheUnstoredDefaultWithoutPersistenceMetadata(t *testing.T) {
	service := &controlledService{nudgePreference: application.NudgeAudiencePreference{
		PathID: "path-1", UserID: "viewer", Audience: domain.DefaultNudgeAudience,
	}}
	request := httptest.NewRequest(http.MethodGet, "/v1/paths/path-1/nudge-preference", nil)
	request.Header.Set("Authorization", "Bearer viewer")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	assertNudgePreferenceResponse(t, response, http.StatusOK, "path_members", 0)
}

func TestNudgeEligibilityAndSendRoutesUseSafeStructuredContent(t *testing.T) {
	sentAt := time.Date(2026, 8, 5, 12, 45, 0, 0, time.UTC)
	service := &controlledService{
		nudgeEligibility: application.NudgeEligibility{PathID: "path-1", RecipientUserID: "recipient", Eligible: false, Reason: application.NudgeGoalCompleteReason},
		nudge:            domain.Nudge{ID: "nudge-1", SenderID: "viewer", RecipientID: "recipient", PathID: "path-1", Content: domain.NudgeContent{Kind: domain.NudgeContentPreset, Preset: domain.NudgeLetsGo}, SentAt: sentAt},
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/paths/path-1/members/recipient/nudge-eligibility", nil)
	request.Header.Set("Authorization", "Bearer viewer")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("eligibility status=%d body=%s", response.Code, response.Body.String())
	}
	var eligibility struct {
		Data struct {
			PathID, RecipientUserID string
			Eligible                bool
			Reason                  string
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &eligibility); err != nil {
		t.Fatal(err)
	}
	if eligibility.Data.PathID != "path-1" || eligibility.Data.RecipientUserID != "recipient" || eligibility.Data.Eligible || eligibility.Data.Reason != "goal_complete" {
		t.Fatalf("eligibility=%+v body=%s", eligibility.Data, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/paths/path-1/members/recipient/nudges", bytes.NewBufferString(`{"content":{"kind":"preset","preset":"lets_go"}}`))
	request.Header.Set("Authorization", "Bearer viewer")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "send-nudge-key-01")
	response = httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("send status=%d body=%s", response.Code, response.Body.String())
	}
	var sent struct {
		Data struct {
			ID, SenderUserID, RecipientUserID, PathID string
			Content                                   struct{ Kind, Preset string }
			SentAt                                    time.Time
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Data.ID != "nudge-1" || sent.Data.SenderUserID != "viewer" || sent.Data.RecipientUserID != "recipient" || sent.Data.PathID != "path-1" || sent.Data.Content.Kind != "preset" || sent.Data.Content.Preset != "lets_go" || !sent.Data.SentAt.Equal(sentAt) {
		t.Fatalf("nudge=%+v body=%s", sent.Data, response.Body.String())
	}
	if service.nudgeKey != "send-nudge-key-01" || service.nudgeContent != (domain.NudgeContent{Kind: domain.NudgeContentPreset, Preset: domain.NudgeLetsGo}) {
		t.Fatalf("service received %+v", service)
	}
}

func TestNudgeMutationsRequireIdempotencyAndRejectCustomContent(t *testing.T) {
	for _, test := range []struct {
		name, method, path, body string
	}{
		{name: "preference key", method: http.MethodPut, path: "/v1/paths/path-1/nudge-preference", body: `{"audience":"everyone","expectedRevision":0}`},
		{name: "send key", method: http.MethodPost, path: "/v1/paths/path-1/members/recipient/nudges", body: `{"content":{"kind":"preset","preset":"lets_go"}}`},
		{name: "custom content", method: http.MethodPost, path: "/v1/paths/path-1/members/recipient/nudges", body: `{"content":{"kind":"custom","preset":"lets_go","text":"hurry up"}}`},
		{name: "missing content kind", method: http.MethodPost, path: "/v1/paths/path-1/members/recipient/nudges", body: `{"content":{"preset":"lets_go"}}`},
		{name: "missing content", method: http.MethodPost, path: "/v1/paths/path-1/members/recipient/nudges", body: `{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			request.Header.Set("Authorization", "Bearer viewer")
			request.Header.Set("Content-Type", "application/json")
			if test.name == "custom content" || test.name == "missing content kind" || test.name == "missing content" {
				request.Header.Set("Idempotency-Key", "send-nudge-key-01")
			}
			response := httptest.NewRecorder()
			handler(&controlledService{}).ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestNudgeRoutesUseSharedOpaqueErrorMapping(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "hidden", err: ports.ErrNotFound, want: http.StatusNotFound},
		{name: "rate limited", err: platformapp.ErrRateLimited, want: http.StatusTooManyRequests},
		{name: "invalid", err: ports.ErrInvalidArgument, want: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &controlledService{err: test.err}
			request := httptest.NewRequest(http.MethodPost, "/v1/paths/path-1/members/recipient/nudges", bytes.NewBufferString(`{"content":{"kind":"preset","preset":"lets_go"}}`))
			request.Header.Set("Authorization", "Bearer viewer")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "send-nudge-key-01")
			response := httptest.NewRecorder()
			handler(service).ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func TestNudgeOpenAPIContractHasExactEnumsAndCreatedStatus(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, &controlledService{}) }}})
	paths := api.OpenAPI().Paths
	preference := paths["/v1/paths/{pathId}/nudge-preference"]
	eligibility := paths["/v1/paths/{pathId}/members/{userId}/nudge-eligibility"]
	send := paths["/v1/paths/{pathId}/members/{userId}/nudges"]
	if preference == nil || preference.Get == nil || preference.Put == nil || eligibility == nil || eligibility.Get == nil || send == nil || send.Post == nil {
		t.Fatalf("nudge paths missing: preference=%+v eligibility=%+v send=%+v", preference, eligibility, send)
	}
	if send.Post.DefaultStatus != http.StatusCreated || len(preference.Get.Security) == 0 || len(preference.Put.Security) == 0 || len(eligibility.Get.Security) == 0 || len(send.Post.Security) == 0 {
		t.Fatalf("nudge operations must be authenticated and POST must be 201")
	}
	encoded, err := json.Marshal(api.OpenAPI())
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`"enum":["nobody","path_members","followers","everyone"]`,
		`"enum":["goal_complete","rate_limited"]`,
		`"enum":["preset"]`,
		`"enum":["you_have_got_this","lets_go","little_progress_counts","keep_it_going","time_to_work"]`,
		`"expectedRevision"`, `"revision"`, `"Idempotency-Key"`,
	} {
		if !bytes.Contains(encoded, []byte(fragment)) {
			t.Errorf("OpenAPI omits %s", fragment)
		}
	}
	for _, schemaName := range []string{"NudgeInput", "NudgeContent"} {
		schema := api.OpenAPI().Components.Schemas.Map()[schemaName]
		if schema == nil {
			t.Fatalf("missing %s schema", schemaName)
		}
		if schema.Properties["text"] != nil || schema.Properties["message"] != nil {
			t.Fatalf("%s exposes custom text: %+v", schemaName, schema.Properties)
		}
		if schemaName == "NudgeInput" && !containsString(schema.Required, "content") {
			t.Fatalf("NudgeInput does not require content: %+v", schema.Required)
		}
		if schemaName == "NudgeContent" && (!containsString(schema.Required, "kind") || !containsString(schema.Required, "preset")) {
			t.Fatalf("NudgeContent does not require kind and preset: %+v", schema.Required)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertNudgePreferenceResponse(t *testing.T, response *httptest.ResponseRecorder, status int, audience string, revision int64) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			PathID, UserID, Audience string
			Revision                 int64
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.PathID != "path-1" || body.Data.UserID != "viewer" || body.Data.Audience != audience || body.Data.Revision != revision || bytes.Contains(response.Body.Bytes(), []byte(`"updatedAt"`)) {
		t.Fatalf("data=%+v body=%s", body.Data, response.Body.String())
	}
}
