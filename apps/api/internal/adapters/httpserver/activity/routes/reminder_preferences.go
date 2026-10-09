package routes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
)

type GoalReminderPreferenceService interface {
	GetGoalReminderPreference(context.Context, string, string) (application.GoalReminderPreference, error)
	UpdateGoalReminderPreference(context.Context, string, string, string, int64, bool) (application.GoalReminderPreference, error)
}
type goalReminderPreferenceInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId" minLength:"1" maxLength:"128"`
}
type goalReminderPreferenceUpdateInput struct {
	Authorization  string `header:"Authorization"`
	PathID         string `path:"pathId" minLength:"1" maxLength:"128"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           struct {
		Enabled          bool  `json:"enabled" required:"true"`
		ExpectedRevision int64 `json:"expectedRevision" required:"true" minimum:"0" maximum:"9223372036854775806"`
	}
}
type GoalReminderPreferenceData struct {
	Enabled  bool  `json:"enabled"`
	Revision int64 `json:"revision" minimum:"0"`
}
type goalReminderPreferenceOutput struct {
	Body struct {
		Data GoalReminderPreferenceData `json:"data"`
	}
}

func goalReminderPreferenceResponse(p application.GoalReminderPreference) *goalReminderPreferenceOutput {
	result := &goalReminderPreferenceOutput{}
	result.Body.Data = GoalReminderPreferenceData{Enabled: p.Enabled, Revision: p.Revision}
	return result
}
func RegisterGoalReminderPreferences(api huma.API, service GoalReminderPreferenceService) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{OperationID: "get-goal-reminder-preference", Method: http.MethodGet, Path: "/v1/me/path-reminders/{pathId}", Summary: "Read the participant's goal reminder preference", Security: security}, func(ctx context.Context, input *goalReminderPreferenceInput) (*goalReminderPreferenceOutput, error) {
		result, err := service.GetGoalReminderPreference(ctx, input.Authorization, input.PathID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return goalReminderPreferenceResponse(result), nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-goal-reminder-preference", Method: http.MethodPut, Path: "/v1/me/path-reminders/{pathId}", Summary: "Update the participant's goal reminder preference", Security: security}, func(ctx context.Context, input *goalReminderPreferenceUpdateInput) (*goalReminderPreferenceOutput, error) {
		result, err := service.UpdateGoalReminderPreference(ctx, input.Authorization, input.PathID, input.IdempotencyKey, input.Body.ExpectedRevision, input.Body.Enabled)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return goalReminderPreferenceResponse(result), nil
	})
}
