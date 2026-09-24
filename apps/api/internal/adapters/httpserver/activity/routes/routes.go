package routes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/dto"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/mapper"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
)

type Service interface {
	StartTimer(context.Context, string, string, string) (application.StartTimerResult, error)
	CurrentTimer(context.Context, string, string) (application.CurrentTimerResult, error)
	StopTimer(context.Context, string, string, string, string) (application.StopTimerResult, error)
	CreateManualActivity(context.Context, string, string, string, application.ManualActivityInput) (application.CreateManualActivityResult, error)
	UpdateActivity(context.Context, string, string, string, string, application.ManualActivityInput) (application.UpdateActivityResult, error)
	DeleteActivity(context.Context, string, string, string, string) (application.DeleteActivityResult, error)
	GetActivity(context.Context, string, string, string) (domain.RecordedActivity, int64, error)
	ListActivities(context.Context, string, string, string, string, int) ([]application.ActivityListRecord, string, error)
	ListActivityRevisions(context.Context, string, string, string, string, int) ([]application.ActivityRevisionRecord, string, error)
	ManualActivityDefaults(context.Context, string, string) (application.ManualActivityDefaults, error)
}

type pathInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
}
type mutationInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
}
type stopInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	TimerID        string `path:"timerId"`
}
type stateOutput struct {
	Body struct {
		Data dto.TimerState `json:"data"`
	}
}
type stopOutput struct {
	Body struct {
		Data dto.StopResult `json:"data"`
	}
}
type activityMutationInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	ActivityID     string `path:"activityId"`
	Body           dto.ManualActivityInputBody
}
type createActivityInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.ManualActivityInputBody
}
type activityInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	ActivityID    string `path:"activityId"`
}
type activityDeletionInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	ActivityID     string `path:"activityId"`
}
type activityListInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	ParticipantID string `query:"participantId"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type activityRevisionListInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	ActivityID    string `path:"activityId"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type activityMutationOutput struct {
	Body struct {
		Data dto.ActivityMutationResult `json:"data"`
	}
}
type activityDetailOutput struct {
	Body struct {
		Data dto.ActivityDetail `json:"data"`
	}
}
type activityDeletionOutput struct {
	Body struct {
		Data dto.ActivityDeletionResult `json:"data"`
	}
}
type activityRevisionsOutput struct {
	Body struct {
		Data []dto.ActivityRevision `json:"data" nullable:"false"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}
type activityListOutput struct {
	Body struct {
		Data []dto.ActivityDetail `json:"data" nullable:"false"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}
type activityDefaultsOutput struct {
	Body struct {
		Data dto.ManualActivityDefaults `json:"data"`
	}
}

func Register(api huma.API, service Service) {
	path := "/v1/paths/{pathId}/timer"
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{OperationID: "start-path-timer", Method: http.MethodPost, Path: path, Security: security}, func(ctx context.Context, input *mutationInput) (*stateOutput, error) {
		result, err := service.StartTimer(ctx, input.Authorization, input.PathID, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &stateOutput{}
		out.Body.Data = mapper.Started(result)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-path-timer", Method: http.MethodGet, Path: path, Security: security}, func(ctx context.Context, input *pathInput) (*stateOutput, error) {
		result, err := service.CurrentTimer(ctx, input.Authorization, input.PathID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &stateOutput{}
		out.Body.Data = mapper.Current(result)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "stop-path-timer", Method: http.MethodDelete, Path: path + "/{timerId}", Security: security}, func(ctx context.Context, input *stopInput) (*stopOutput, error) {
		result, err := service.StopTimer(ctx, input.Authorization, input.PathID, input.TimerID, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &stopOutput{}
		out.Body.Data = mapper.Stopped(result)
		return out, nil
	})
	activitiesPath := "/v1/paths/{pathId}/activities"
	huma.Register(api, huma.Operation{OperationID: "get-manual-activity-defaults", Method: http.MethodGet, Path: activitiesPath + "/manual-defaults", Security: security}, func(ctx context.Context, input *pathInput) (*activityDefaultsOutput, error) {
		result, err := service.ManualActivityDefaults(ctx, input.Authorization, input.PathID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &activityDefaultsOutput{}
		out.Body.Data = dto.ManualActivityDefaults{LocalDate: result.LocalDate, LocalStartTime: result.LocalStartTime, TimeZone: result.TimeZone, CurrentInstant: result.CurrentInstant}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "create-manual-activity", Method: http.MethodPost, Path: activitiesPath, Security: security, DefaultStatus: http.StatusCreated}, func(ctx context.Context, input *createActivityInput) (*activityMutationOutput, error) {
		result, err := service.CreateManualActivity(ctx, input.Authorization, input.PathID, input.IdempotencyKey, mapper.ManualInput(input.Body))
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &activityMutationOutput{}
		out.Body.Data = mapper.Created(result)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-path-activities", Method: http.MethodGet, Path: activitiesPath, Security: security}, func(ctx context.Context, input *activityListInput) (*activityListOutput, error) {
		result, cursor, err := service.ListActivities(ctx, input.Authorization, input.PathID, input.ParticipantID, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &activityListOutput{}
		out.Body.Data = mapper.List(result)
		out.Body.Meta.NextCursor = cursor
		return out, nil
	})
	activityPath := activitiesPath + "/{activityId}"
	huma.Register(api, huma.Operation{OperationID: "update-activity", Method: http.MethodPut, Path: activityPath, Security: security}, func(ctx context.Context, input *activityMutationInput) (*activityMutationOutput, error) {
		result, err := service.UpdateActivity(ctx, input.Authorization, input.PathID, input.ActivityID, input.IdempotencyKey, mapper.ManualInput(input.Body))
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &activityMutationOutput{}
		out.Body.Data = mapper.Updated(result)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "delete-activity", Method: http.MethodDelete, Path: activityPath, Security: security}, func(ctx context.Context, input *activityDeletionInput) (*activityDeletionOutput, error) {
		result, err := service.DeleteActivity(ctx, input.Authorization, input.PathID, input.ActivityID, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &activityDeletionOutput{}
		out.Body.Data = mapper.Deleted(result)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-activity", Method: http.MethodGet, Path: activityPath, Security: security}, func(ctx context.Context, input *activityInput) (*activityDetailOutput, error) {
		entry, version, err := service.GetActivity(ctx, input.Authorization, input.PathID, input.ActivityID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &activityDetailOutput{}
		out.Body.Data = mapper.Detail(entry, version)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-activity-revisions", Method: http.MethodGet, Path: activityPath + "/revisions", Security: security}, func(ctx context.Context, input *activityRevisionListInput) (*activityRevisionsOutput, error) {
		result, cursor, err := service.ListActivityRevisions(ctx, input.Authorization, input.PathID, input.ActivityID, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &activityRevisionsOutput{}
		out.Body.Data = mapper.Revisions(result)
		out.Body.Meta.NextCursor = cursor
		return out, nil
	})
}
