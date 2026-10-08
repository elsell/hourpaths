package routes

import (
	"context"
	"encoding/json"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/notification"
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

func (service *controlledService) ListNotificationChannels(_ context.Context, authorization string) ([]application.NotificationChannelPreference, error) {
	service.authorization = authorization
	var result []application.NotificationChannelPreference
	for _, channel := range notification.Channels() {
		result = append(result, application.NotificationChannelPreference{Channel: channel, NudgeNotificationChannelPreference: application.NudgeNotificationChannelPreference{Enabled: true}})
	}
	return result, service.err
}
func (service *controlledService) UpdateNotificationChannel(_ context.Context, authorization, channel, key string, revision int64, enabled bool) (application.NotificationChannelPreference, error) {
	service.authorization = authorization
	return application.NotificationChannelPreference{Channel: notification.Channel(channel), NudgeNotificationChannelPreference: application.NudgeNotificationChannelPreference{Enabled: enabled, Revision: revision + 1}}, service.err
}

func (service *controlledService) GetTimerSubscription(_ context.Context, authorization, scope, subject string) (application.TimerSubscription, error) {
	service.authorization = authorization
	return application.TimerSubscription{Enabled: scope == "path"}, service.err
}
func (service *controlledService) UpdateTimerSubscription(_ context.Context, authorization, scope, subject, key string, revision int64, enabled bool) (application.TimerSubscription, error) {
	service.authorization = authorization
	return application.TimerSubscription{Enabled: enabled, Revision: revision + 1}, service.err
}
