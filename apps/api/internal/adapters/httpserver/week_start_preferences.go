package httpserver

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"net/http"
)

type WeekStartPreferenceDTO struct {
	FirstDayOfWeek int  `json:"firstDayOfWeek" minimum:"1" maximum:"7"`
	Changed        bool `json:"changed"`
}
type WeekStartPreferenceOutput struct {
	Body struct {
		Data WeekStartPreferenceDTO `json:"data"`
	}
}
type WeekStartPreferenceInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           struct {
		ReviewedFirstDayOfWeek int `json:"reviewedFirstDayOfWeek" required:"true" minimum:"1" maximum:"7"`
		ProposedFirstDayOfWeek int `json:"proposedFirstDayOfWeek" required:"true" minimum:"1" maximum:"7"`
	}
}

func registerWeekStartPreferenceRoutes(api huma.API, application app.App) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{OperationID: "get-configured-week-start", Method: http.MethodGet, Path: "/v1/me/week-start", Summary: "Get the configured first day of the week", Security: security}, func(ctx context.Context, input *MeInput) (*WeekStartPreferenceOutput, error) {
		value, err := application.ConfiguredWeekStart(ctx, input.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &WeekStartPreferenceOutput{}
		out.Body.Data = WeekStartPreferenceDTO{FirstDayOfWeek: value.FirstDayOfWeek}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-configured-week-start", Method: http.MethodPut, Path: "/v1/me/week-start", Summary: "Update the first day of the week without changing existing goals", Security: security}, func(ctx context.Context, input *WeekStartPreferenceInput) (*WeekStartPreferenceOutput, error) {
		result, err := application.UpdateConfiguredWeekStart(ctx, input.Authorization, input.IdempotencyKey, app.WeekStartPreferenceUpdate{ReviewedFirstDayOfWeek: input.Body.ReviewedFirstDayOfWeek, ProposedFirstDayOfWeek: input.Body.ProposedFirstDayOfWeek})
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &WeekStartPreferenceOutput{}
		out.Body.Data = WeekStartPreferenceDTO{FirstDayOfWeek: result.Preference.FirstDayOfWeek, Changed: result.Changed}
		return out, nil
	})
}
