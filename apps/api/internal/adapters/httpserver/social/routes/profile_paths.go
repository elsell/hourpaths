package routes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/social/dto"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"net/http"
)

type profilePathsService interface {
	GetProfilePaths(context.Context, string, string) (application.ProfilePaths, error)
}
type profilePathsData struct {
	PathCount int                     `json:"pathCount" minimum:"0"`
	Active    dto.ActiveFollowingItem `json:"active"`
}
type profilePathsOutput struct {
	Body struct {
		Data profilePathsData `json:"data"`
	}
}

func registerProfilePaths(api huma.API, service Service, security []map[string][]string) {
	reader, ok := service.(profilePathsService)
	if !ok {
		return
	}
	huma.Register(api, huma.Operation{OperationID: "get-profile-paths", Method: http.MethodGet, Path: "/v1/profiles/{username}/paths", Summary: "Get visible profile Path count and active Paths", Security: security}, func(ctx context.Context, input *profileInput) (*profilePathsOutput, error) {
		result, err := reader.GetProfilePaths(ctx, input.Authorization, input.Username)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &profilePathsOutput{}
		out.Body.Data.PathCount = result.Count
		out.Body.Data.Active = mapActiveFollowingItem(result.Active)
		return out, nil
	})
}
