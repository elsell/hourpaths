package routes

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/dto"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/mapper"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
)

type offlineTimerInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.OfflineTimerInput
}
type offlineTimerOutput struct {
	Body struct {
		Data dto.OfflineTimerResult `json:"data"`
	}
}

func registerOfflineTimer(api huma.API, service Service) {
	huma.Register(api, huma.Operation{OperationID: "synchronize-offline-path-timer", Method: http.MethodPost, Path: "/v1/paths/{pathId}/offline-timer", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *offlineTimerInput) (*offlineTimerOutput, error) {
		result, err := service.SynchronizeTimer(ctx, input.Authorization, input.PathID, input.IdempotencyKey, application.OfflineTimerInput{TimerID: input.Body.TimerID, Kind: input.Body.Kind, StartedAt: input.Body.StartedAt, EndedAt: input.Body.EndedAt, CorrectedStartedAt: input.Body.CorrectedStartedAt, OccurrenceTimeZone: input.Body.OccurrenceTimeZone})
		if err != nil {
			mapped := shared.MapError(err, true)
			var problem *shared.APIError
			if errors.As(mapped, &problem) {
				problem.Operation = "synchronize-offline-path-timer"
			}
			return nil, mapped
		}
		out := &offlineTimerOutput{}
		out.Body.Data = dto.OfflineTimerResult{Terminal: result.Terminal, MustStop: result.MustStop, Outcome: result.Outcome, SavedSeconds: result.SavedSeconds, DiscardedSeconds: result.DiscardedSeconds}
		if result.Timer != nil {
			timer := mapper.Timer(*result.Timer)
			out.Body.Data.Timer = &timer
		}
		if result.Activity != nil {
			entry := mapper.Activity(*result.Activity)
			out.Body.Data.Activity = &entry
		}
		return out, nil
	})
}
