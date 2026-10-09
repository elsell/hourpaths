package routes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
)

type notificationOwnershipInput struct {
	Authorization  string `header:"Authorization"`
	NotificationID string `path:"notificationId" maxLength:"128"`
}
type notificationOwnershipDTO struct {
	Owned bool `json:"owned"`
}
type notificationOwnershipOutput struct {
	Body struct {
		Data notificationOwnershipDTO `json:"data"`
	}
}

func registerNotificationOwnership(api huma.API, service InvitationService) {
	huma.Register(api, huma.Operation{
		OperationID: "check-notification-ownership", Method: http.MethodGet,
		Path:     "/v1/me/notifications/{notificationId}/ownership",
		Summary:  "Verify recipient ownership for account deletion cleanup without exposing notification content",
		Security: []map[string][]string{{"oidc": {}}},
	}, func(ctx context.Context, input *notificationOwnershipInput) (*notificationOwnershipOutput, error) {
		// Reuse the audited, recipient-scoped resolution policy for every supported
		// notification kind. Only the ownership fact crosses this transport boundary.
		ctx = pathapp.WithNotificationEmojiRepresentation(pathapp.WithNotificationTimerRepresentation(pathapp.WithNotificationAchievementRepresentation(pathapp.WithNotificationLongTimerRepresentation(pathapp.WithNotificationGoalDeadlineRepresentation(ctx, true), true), true), true), true)
		if _, err := service.GetNotification(ctx, input.Authorization, input.NotificationID); err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &notificationOwnershipOutput{}
		output.Body.Data.Owned = true
		return output, nil
	})
}
