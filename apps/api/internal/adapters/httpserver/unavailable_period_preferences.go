package httpserver

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/preferences"
	"net/http"
)

type UnavailablePeriodDTO struct {
	Enabled     bool   `json:"enabled"`
	StartMinute int    `json:"startMinute" minimum:"0" maximum:"1439"`
	EndMinute   int    `json:"endMinute" minimum:"0" maximum:"1439"`
	Revision    int64  `json:"revision" minimum:"0"`
	TimeZone    string `json:"timeZone"`
}
type UnavailablePeriodOutput struct {
	Body struct {
		Data UnavailablePeriodDTO `json:"data"`
	}
}
type UnavailablePeriodInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           struct {
		Enabled          bool   `json:"enabled" required:"true"`
		StartMinute      int    `json:"startMinute" required:"true" minimum:"0" maximum:"1439"`
		EndMinute        int    `json:"endMinute" required:"true" minimum:"0" maximum:"1439"`
		ExpectedRevision int64  `json:"expectedRevision" required:"true" minimum:"0"`
		ReviewedTimeZone string `json:"reviewedTimeZone" required:"true"`
	}
}

func unavailablePeriodOutput(value app.UnavailablePeriodPreference) *UnavailablePeriodOutput {
	out := &UnavailablePeriodOutput{}
	out.Body.Data = UnavailablePeriodDTO{Enabled: value.Period.Enabled, StartMinute: value.Period.StartMinute, EndMinute: value.Period.EndMinute, Revision: value.Revision, TimeZone: value.TimeZone}
	return out
}
func registerUnavailablePeriodRoutes(api huma.API, application app.App) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{OperationID: "get-unavailable-period", Method: http.MethodGet, Path: "/v1/me/unavailable-period", Summary: "Read the account daily unavailable period", Security: security}, func(ctx context.Context, input *MeInput) (*UnavailablePeriodOutput, error) {
		value, err := application.ConfiguredUnavailablePeriod(ctx, input.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		return unavailablePeriodOutput(value), nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-unavailable-period", Method: http.MethodPut, Path: "/v1/me/unavailable-period", Summary: "Update the account daily unavailable period", Security: security}, func(ctx context.Context, input *UnavailablePeriodInput) (*UnavailablePeriodOutput, error) {
		body := input.Body
		value, err := application.UpdateConfiguredUnavailablePeriod(ctx, input.Authorization, input.IdempotencyKey, app.UnavailablePeriodUpdate{ExpectedRevision: body.ExpectedRevision, ReviewedTimeZone: body.ReviewedTimeZone, Period: preferences.UnavailablePeriod{Enabled: body.Enabled, StartMinute: body.StartMinute, EndMinute: body.EndMinute}})
		if err != nil {
			return nil, mapError(err, false)
		}
		return unavailablePeriodOutput(value.Preference), nil
	})
}
