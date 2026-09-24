package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
)

func TestInteractionSettingsRoutesProjectIndependentFlags(t *testing.T) {
	service := &controlledService{interactionSettings: application.InteractionSettings{CommentsEnabled: true, ReactionsEnabled: true}}
	request := httptest.NewRequest(http.MethodGet, "/v1/me/interaction-settings", nil)
	request.Header.Set("Authorization", "Bearer viewer")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	assertInteractionSettingsResponse(t, response, http.StatusOK, true, true)

	request = httptest.NewRequest(http.MethodPut, "/v1/me/interaction-settings", bytes.NewBufferString(`{"commentsEnabled":false,"reactionsEnabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer viewer")
	request.Header.Set("Idempotency-Key", "interaction-settings-key")
	response = httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	assertInteractionSettingsResponse(t, response, http.StatusOK, false, true)
	if service.authorization != "Bearer viewer" || service.interactionKey != "interaction-settings-key" || service.interactionSettings != (application.InteractionSettings{CommentsEnabled: false, ReactionsEnabled: true}) {
		t.Fatalf("service received %+v", service)
	}
}

func TestInteractionSettingsUpdateRequiresIdempotencyKey(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/v1/me/interaction-settings", bytes.NewBufferString(`{"commentsEnabled":true,"reactionsEnabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer viewer")
	response := httptest.NewRecorder()
	handler(&controlledService{}).ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func assertInteractionSettingsResponse(t *testing.T, response *httptest.ResponseRecorder, status int, comments, reactions bool) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data dtoInteractionSettings `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.CommentsEnabled != comments || body.Data.ReactionsEnabled != reactions {
		t.Fatalf("data=%+v", body.Data)
	}
}

type dtoInteractionSettings struct {
	CommentsEnabled  bool `json:"commentsEnabled"`
	ReactionsEnabled bool `json:"reactionsEnabled"`
}
