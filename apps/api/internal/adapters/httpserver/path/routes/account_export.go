package routes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/dto"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/mapper"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"net/http"
	"time"
)

type accountExportPathDTO struct {
	Path      dto.PathItem `json:"path"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}
type accountExportPathsOutput struct {
	Body struct {
		Data []accountExportPathDTO `json:"data" nullable:"false"`
		Meta shared.PaginationMeta  `json:"meta"`
	}
}

func registerAccountExportPaths(api huma.API, service Service) {
	huma.Register(api, huma.Operation{OperationID: "export-own-paths", Method: http.MethodGet, Path: "/v1/me/export/paths", Summary: "Export accessible active or archived Paths with their goals", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *PathListInput) (*accountExportPathsOutput, error) {
		var items []pathapp.Projection
		var next string
		var err error
		if input.Archived {
			items, next, err = service.ListArchivedProjected(ctx, input.Authorization, input.Cursor, input.Limit)
		} else {
			items, next, err = service.ListProjected(ctx, input.Authorization, input.Cursor, input.Limit)
		}
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		output := &accountExportPathsOutput{}
		output.Body.Data = make([]accountExportPathDTO, 0, len(items))
		output.Body.Meta.NextCursor = next
		for _, item := range items {
			output.Body.Data = append(output.Body.Data, accountExportPathDTO{Path: mapper.Item(item.Path), CreatedAt: item.Path.CreatedAt, UpdatedAt: item.Path.UpdatedAt})
		}
		return output, nil
	})
}
