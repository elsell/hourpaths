package routes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"net/http"
)

type notificationChannelPreference struct {
	Channel  string `json:"channel" enum:"following,path_access,tracking_activity,achievements,comments,reactions,comment_hearts,nudges,goal_reminders,timer_health"`
	Enabled  bool   `json:"enabled" required:"true"`
	Revision int64  `json:"revision" minimum:"0"`
}
type notificationChannelsOutput struct {
	Body struct {
		Data []notificationChannelPreference `json:"data"`
	}
}
type notificationChannelOutput struct {
	Body struct {
		Data notificationChannelPreference `json:"data"`
	}
}
type notificationChannelUpdateInput struct {
	Authorization  string `header:"Authorization"`
	Channel        string `path:"channel" enum:"following,path_access,tracking_activity,achievements,comments,reactions,comment_hearts,nudges,goal_reminders,timer_health"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           struct {
		Enabled          bool  `json:"enabled" required:"true"`
		ExpectedRevision int64 `json:"expectedRevision" required:"true" minimum:"0"`
	}
}

func mapNotificationChannel(p application.NotificationChannelPreference) notificationChannelPreference {
	return notificationChannelPreference{Channel: string(p.Channel), Enabled: p.Enabled, Revision: p.Revision}
}
func registerNotificationChannelRoutes(api huma.API, service Service, security []map[string][]string) {
	huma.Register(api, huma.Operation{OperationID: "list-notification-channels", Method: http.MethodGet, Path: "/v1/me/notification-channels", Summary: "List the viewer's notification channels", Security: security}, func(ctx context.Context, input *nudgeNotificationChannelInput) (*notificationChannelsOutput, error) {
		preferences, err := service.ListNotificationChannels(ctx, input.Authorization)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &notificationChannelsOutput{}
		out.Body.Data = make([]notificationChannelPreference, 0, len(preferences))
		for _, p := range preferences {
			out.Body.Data = append(out.Body.Data, mapNotificationChannel(p))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-notification-channel", Method: http.MethodPut, Path: "/v1/me/notification-channels/{channel}", Summary: "Update one of the viewer's notification channels", Security: security}, func(ctx context.Context, input *notificationChannelUpdateInput) (*notificationChannelOutput, error) {
		preference, err := service.UpdateNotificationChannel(ctx, input.Authorization, input.Channel, input.IdempotencyKey, input.Body.ExpectedRevision, input.Body.Enabled)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &notificationChannelOutput{}
		out.Body.Data = mapNotificationChannel(preference)
		return out, nil
	})
}
