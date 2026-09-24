package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
)

func TestNudgeNotificationChannelRoutesExposeOnlyPublicRevisionedPreference(t *testing.T) {
	service := &controlledService{nudgeNotificationChannel: application.NudgeNotificationChannelPreference{Enabled: true}}
	request := httptest.NewRequest(http.MethodGet, "/v1/me/notification-channels/nudges", nil)
	request.Header.Set("Authorization", "Bearer viewer")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	assertNudgeNotificationChannelResponse(t, response, http.StatusOK, true, 0)

	service.nudgeNotificationChannel = application.NudgeNotificationChannelPreference{Enabled: false, Revision: 1}
	request = httptest.NewRequest(http.MethodPut, "/v1/me/notification-channels/nudges", bytes.NewBufferString(`{"enabled":false,"expectedRevision":0}`))
	request.Header.Set("Authorization", "Bearer viewer")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "nudge-channel-key")
	response = httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	assertNudgeNotificationChannelResponse(t, response, http.StatusOK, false, 1)
	if service.authorization != "Bearer viewer" || service.nudgeNotificationKey != "nudge-channel-key" || service.nudgeNotificationExpectedRevision != 0 || service.nudgeNotificationEnabled {
		t.Fatalf("service received %+v", service)
	}
}

func TestNudgeNotificationChannelUpdateRequiresRevisionAndIdempotency(t *testing.T) {
	for _, test := range []struct {
		name, body, key string
	}{
		{name: "missing key", body: `{"enabled":true,"expectedRevision":0}`},
		{name: "missing revision", body: `{"enabled":true}`, key: "nudge-channel-key"},
		{name: "negative revision", body: `{"enabled":true,"expectedRevision":-1}`, key: "nudge-channel-key"},
		{name: "unknown channel shape", body: `{"channel":"comments","enabled":true,"expectedRevision":0}`, key: "nudge-channel-key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/v1/me/notification-channels/nudges", bytes.NewBufferString(test.body))
			request.Header.Set("Authorization", "Bearer viewer")
			request.Header.Set("Content-Type", "application/json")
			if test.key != "" {
				request.Header.Set("Idempotency-Key", test.key)
			}
			response := httptest.NewRecorder()
			handler(&controlledService{}).ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestNudgeNotificationChannelOpenAPIIsAuthenticatedAndExact(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, &controlledService{}) }}})
	path := api.OpenAPI().Paths["/v1/me/notification-channels/nudges"]
	if path == nil || path.Get == nil || path.Put == nil || len(path.Get.Security) == 0 || len(path.Put.Security) == 0 {
		t.Fatalf("nudge notification channel contract=%+v", path)
	}
	encoded, err := json.Marshal(api.OpenAPI())
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"enum":["nudges"]`, `"expectedRevision"`, `"revision"`, `"Idempotency-Key"`} {
		if !bytes.Contains(encoded, []byte(fragment)) {
			t.Errorf("OpenAPI omits %s", fragment)
		}
	}
	for _, schemaName := range []string{"NudgeNotificationChannelPreference", "NudgeNotificationChannelPreferenceInput"} {
		schema := api.OpenAPI().Components.Schemas.Map()[schemaName]
		if schema == nil {
			t.Fatalf("missing schema %s", schemaName)
		}
		for _, internal := range []string{"userId", "createdAt", "updatedAt"} {
			if schema.Properties[internal] != nil {
				t.Fatalf("%s exposes %s", schemaName, internal)
			}
		}
	}
}

func assertNudgeNotificationChannelResponse(t *testing.T, response *httptest.ResponseRecorder, status int, enabled bool, revision int64) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 3 || body.Data["channel"] != "nudges" || body.Data["enabled"] != enabled || body.Data["revision"] != float64(revision) {
		t.Fatalf("data=%v body=%s", body.Data, response.Body.String())
	}
}
