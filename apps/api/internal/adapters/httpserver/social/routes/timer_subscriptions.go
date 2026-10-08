package routes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
)

type timerSubscriptionInput struct {
	Authorization string `header:"Authorization"`
	Scope         string `path:"scope" enum:"person,path"`
	SubjectID     string `path:"subjectId" minLength:"1" maxLength:"128"`
}
type timerSubscriptionUpdateInput struct {
	Authorization  string `header:"Authorization"`
	Scope          string `path:"scope" enum:"person,path"`
	SubjectID      string `path:"subjectId" minLength:"1" maxLength:"128"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           struct {
		Enabled          bool  `json:"enabled" required:"true"`
		ExpectedRevision int64 `json:"expectedRevision" required:"true" minimum:"0" maximum:"9223372036854775806"`
	}
}
type timerSubscriptionPreference struct {
	Enabled  bool  `json:"enabled" required:"true"`
	Revision int64 `json:"revision" minimum:"0"`
}
type timerSubscriptionOutput struct {
	Body struct {
		Data timerSubscriptionPreference `json:"data"`
	}
}

func timerSubscriptionResponse(preference application.TimerSubscription) *timerSubscriptionOutput {
	out := &timerSubscriptionOutput{}
	out.Body.Data = timerSubscriptionPreference{Enabled: preference.Enabled, Revision: preference.Revision}
	return out
}
func registerTimerSubscriptionRoutes(api huma.API, service Service, security []map[string][]string) {
	huma.Register(api, huma.Operation{OperationID: "get-timer-subscription", Method: http.MethodGet, Path: "/v1/me/timer-subscriptions/{scope}/{subjectId}", Summary: "Read the viewer's timer-start subscription", Security: security}, func(ctx context.Context, input *timerSubscriptionInput) (*timerSubscriptionOutput, error) {
		preference, err := service.GetTimerSubscription(ctx, input.Authorization, input.Scope, input.SubjectID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return timerSubscriptionResponse(preference), nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-timer-subscription", Method: http.MethodPut, Path: "/v1/me/timer-subscriptions/{scope}/{subjectId}", Summary: "Update the viewer's timer-start subscription", Security: security}, func(ctx context.Context, input *timerSubscriptionUpdateInput) (*timerSubscriptionOutput, error) {
		preference, err := service.UpdateTimerSubscription(ctx, input.Authorization, input.Scope, input.SubjectID, input.IdempotencyKey, input.Body.ExpectedRevision, input.Body.Enabled)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return timerSubscriptionResponse(preference), nil
	})
}
