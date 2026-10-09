package routes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"net/http"
	"time"
)

type ownRunningTimerInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor" maxLength:"4096"`
	Limit         int    `query:"limit" default:"50" minimum:"1" maximum:"100"`
}
type ownRunningTimerDTO struct {
	ID        string    `json:"id"`
	PathID    string    `json:"pathId"`
	PathName  string    `json:"pathName"`
	StartedAt time.Time `json:"startedAt"`
}
type ownRunningTimersOutput struct {
	Body struct {
		Data []ownRunningTimerDTO  `json:"data" nullable:"false"`
		Meta shared.PaginationMeta `json:"meta"`
	}
}

func registerOwnRunningTimers(api huma.API, service Service) {
	huma.Register(api, huma.Operation{OperationID: "list-own-running-timers", Method: http.MethodGet, Path: "/v1/me/running-timers", Summary: "List only the current account's running timers for stop controls", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, in *ownRunningTimerInput) (*ownRunningTimersOutput, error) {
		items, next, err := service.ListOwnRunningTimers(ctx, in.Authorization, in.Cursor, in.Limit)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		out := &ownRunningTimersOutput{}
		out.Body.Data = make([]ownRunningTimerDTO, 0, len(items))
		out.Body.Meta.NextCursor = next
		for _, item := range items {
			out.Body.Data = append(out.Body.Data, ownRunningTimerDTO{ID: item.Timer.ID, PathID: item.Timer.PathID, PathName: item.PathName, StartedAt: item.Timer.StartedAt})
		}
		return out, nil
	})
}
