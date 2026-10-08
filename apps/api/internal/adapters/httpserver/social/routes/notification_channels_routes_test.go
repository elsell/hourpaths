package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotificationChannelCatalogRouteExposesTenIndependentSettings(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/me/notification-channels", nil)
	request.Header.Set("Authorization", "Bearer viewer")
	response := httptest.NewRecorder()
	handler(&controlledService{}).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data []struct {
			Channel  string `json:"channel"`
			Enabled  bool   `json:"enabled"`
			Revision int64  `json:"revision"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{"following": true, "path_access": true, "tracking_activity": true, "achievements": true, "comments": true, "reactions": true, "comment_hearts": true, "nudges": true, "goal_reminders": true, "timer_health": true}
	for _, row := range body.Data {
		if !expected[row.Channel] || !row.Enabled || row.Revision != 0 {
			t.Fatalf("unexpected preference %+v", row)
		}
		delete(expected, row.Channel)
	}
	if len(expected) != 0 {
		t.Fatalf("missing channels: %v", expected)
	}
}
