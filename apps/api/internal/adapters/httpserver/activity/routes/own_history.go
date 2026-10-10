package routes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/dto"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/mapper"
	"net/http"
)

type ownHistoryInput struct {
	Authorization string `header:"Authorization"`
	Window        string `query:"window" enum:"all,retained" default:"all"`
	Cursor        string `query:"cursor" maxLength:"4096"`
	Limit         int    `query:"limit" default:"100" minimum:"1" maximum:"100"`
}
type ownHistoryRecord struct {
	Activity dto.Activity `json:"activity"`
	Version  int64        `json:"version" minimum:"1"`
	PathName string       `json:"pathName"`
}
type ownHistoryOutput struct {
	Body struct {
		Data []ownHistoryRecord    `json:"data" nullable:"false"`
		Meta shared.PaginationMeta `json:"meta"`
	}
}

func registerOwnHistory(api huma.API, service Service) {
	huma.Register(api, huma.Operation{OperationID: "list-own-activity-history", Method: http.MethodGet, Path: "/v1/me/activity-history", Summary: "List the current account's activity on accessible Paths", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, in *ownHistoryInput) (*ownHistoryOutput, error) {
		items, next, err := service.ListOwnHistory(ctx, in.Authorization, in.Window, in.Cursor, in.Limit)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		out := &ownHistoryOutput{}
		out.Body.Data = make([]ownHistoryRecord, 0, len(items))
		out.Body.Meta.NextCursor = next
		for _, item := range items {
			out.Body.Data = append(out.Body.Data, ownHistoryRecord{Activity: mapper.Activity(item.Activity), Version: item.Version, PathName: item.PathName})
		}
		return out, nil
	})
}
