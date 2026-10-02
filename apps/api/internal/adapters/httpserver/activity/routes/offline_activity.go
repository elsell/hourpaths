package routes

import (
	"context"
	"errors"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/dto"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/mapper"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"net/http"
)

type offlineActivityInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.OfflineActivityInput
}
type offlineActivityOutput struct {
	Body struct {
		Data dto.OfflineActivityResult `json:"data"`
	}
}

func registerOfflineActivity(api huma.API, service Service) {
	huma.Register(api, huma.Operation{OperationID: "synchronize-offline-path-activity", Method: http.MethodPost, Path: "/v1/paths/{pathId}/offline-activity", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *offlineActivityInput) (*offlineActivityOutput, error) {
		result, err := service.SynchronizeActivity(ctx, input.Authorization, input.PathID, input.IdempotencyKey, application.OfflineActivityInput{Kind: input.Body.Kind, ActivityID: input.Body.ActivityID, StartedAt: input.Body.StartedAt, DurationSeconds: input.Body.DurationSeconds, OccurrenceTimeZone: input.Body.OccurrenceTimeZone, Note: input.Body.Note, AuthoredAt: input.Body.AuthoredAt, Counter: input.Body.Counter})
		if err != nil {
			mapped := shared.MapError(err, true)
			var problem *shared.APIError
			if errors.As(mapped, &problem) {
				problem.Operation = "synchronize-offline-path-activity"
			}
			return nil, mapped
		}
		out := &offlineActivityOutput{}
		out.Body.Data.Outcome = result.Outcome
		if result.Activity != nil {
			entry := mapper.Activity(*result.Activity)
			out.Body.Data.Activity = &entry
		}
		if result.Order != nil {
			out.Body.Data.Order = &dto.ActivityEditOrder{AuthoredAt: result.Order.AuthoredAt, Counter: result.Order.Counter, OperationID: result.Order.OperationID}
		}
		return out, nil
	})
}
