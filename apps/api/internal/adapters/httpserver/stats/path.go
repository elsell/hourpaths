package stats

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/stats"
	"net/http"
)

type pathStatsInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId" minLength:"1" maxLength:"128"`
	ParticipantID string `query:"participantId" maxLength:"128"`
}
type pathStatsOutput struct {
	Body struct {
		Data domain.PathSummary `json:"data"`
	}
}

func registerPath(api huma.API, service Service) {
	huma.Register(api, huma.Operation{OperationID: "get-path-statistics", Method: http.MethodGet, Path: "/v1/paths/{pathId}/statistics", Summary: "Read one visible participant's completed Path statistics", Tags: []string{"Stats"}, Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *pathStatsInput) (*pathStatsOutput, error) {
		result, err := service.GetPath(ctx, input.Authorization, input.PathID, input.ParticipantID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &pathStatsOutput{}
		output.Body.Data = result
		return output, nil
	})
}
