package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateHomePreferencesUsesSelfScopedTypedContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/v1/me/home-preferences", strings.NewReader(`{"expectedRevision":0,"orderMethod":"manual","pinnedPathIds":["path-2"],"manualPathIds":["path-2","path-1"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer valid-application-session")
	request.Header.Set("Idempotency-Key", "home-order-key-0001")
	response := httptest.NewRecorder()
	pathHandler(controlledService{}).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			OrderMethod                  string `json:"orderMethod"`
			Revision                     int64  `json:"revision"`
			PinnedPathIDs, ManualPathIDs []string
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.OrderMethod != "manual" || body.Data.Revision != 1 || len(body.Data.PinnedPathIDs) != 1 || len(body.Data.ManualPathIDs) != 2 {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func TestPathListCarriesFullHomePreferenceSnapshotWhenEmpty(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/paths?limit=25", nil)
	request.Header.Set("Authorization", "Bearer valid-application-session")
	response := httptest.NewRecorder()
	pathHandler(controlledService{}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"homePreferences":{"orderMethod":"recent","revision":0,"pinnedPathIds":[],"manualPathIds":[]}`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMemberPathProjectionCarriesAuthoritativeRecentHomeActivity(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/paths/path-1", nil)
	request.Header.Set("Authorization", "Bearer valid-application-session")
	response := httptest.NewRecorder()
	pathHandler(controlledService{}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"home":{"classification":"shared","pinned":false,"recentActivityAt":"2026-08-04T11:00:00Z"}`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestUpdateHomePreferencesRequiresIdempotencyAndRejectsInvalidOrder(t *testing.T) {
	for name, fixture := range map[string][2]string{
		"missing key":   {"", `{"expectedRevision":0,"orderMethod":"recent","pinnedPathIds":[],"manualPathIds":[]}`},
		"unknown order": {"home-order-key-0001", `{"expectedRevision":0,"orderMethod":"popular","pinnedPathIds":[],"manualPathIds":[]}`},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/v1/me/home-preferences", strings.NewReader(fixture[1]))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer valid-application-session")
			if fixture[0] != "" {
				request.Header.Set("Idempotency-Key", fixture[0])
			}
			response := httptest.NewRecorder()
			pathHandler(controlledService{}).ServeHTTP(response, request)
			if response.Code < 400 {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
