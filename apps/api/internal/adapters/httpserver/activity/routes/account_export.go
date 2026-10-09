package routes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/mapper"
	"net/http"
)

type ownActivityExportInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	Cursor        string `query:"cursor" maxLength:"4096"`
	Limit         int    `query:"limit" default:"100" minimum:"1" maximum:"100"`
}

func registerOwnActivityExport(api huma.API, service Service) {
	huma.Register(api, huma.Operation{OperationID: "export-own-activities", Method: http.MethodGet, Path: "/v1/me/export/paths/{pathId}/activities", Summary: "Export only the current account's recorded activity on an accessible Path", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *ownActivityExportInput) (*activityListOutput, error) {
		items, next, err := service.ExportOwnActivities(ctx, input.Authorization, input.PathID, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &activityListOutput{}
		output.Body.Data = mapper.List(items)
		output.Body.Meta.NextCursor = next
		return output, nil
	})
}
