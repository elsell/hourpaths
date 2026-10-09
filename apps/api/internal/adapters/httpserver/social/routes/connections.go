package routes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/social/dto"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"net/http"
)

type connectionsService interface {
	ListConnections(context.Context, string, string, application.ConnectionDirection, string, int) ([]domain.PublicProfile, string, error)
	RemoveFollower(context.Context, string, string, string) (application.RelationshipResult, error)
}
type connectionsInput struct {
	Authorization string `header:"Authorization"`
	Username      string `path:"username" minLength:"3" maxLength:"64" pattern:"^[A-Za-z0-9_.]+$"`
	Cursor        string `query:"cursor" maxLength:"4096"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}

func registerConnections(api huma.API, service Service, security []map[string][]string) {
	connections, ok := service.(connectionsService)
	if !ok {
		return
	}
	for _, direction := range []application.ConnectionDirection{application.Followers, application.Following} {
		huma.Register(api, huma.Operation{OperationID: "list-profile-" + string(direction), Method: http.MethodGet, Path: "/v1/profiles/{username}/" + string(direction), Summary: "List visible profile " + string(direction), Security: security}, func(ctx context.Context, input *connectionsInput) (*searchOutput, error) {
			profiles, cursor, err := connections.ListConnections(ctx, input.Authorization, input.Username, direction, input.Cursor, input.Limit)
			if err != nil {
				return nil, shared.MapError(err, true)
			}
			out := &searchOutput{}
			out.Body.Data = make([]dto.PublicProfile, 0, len(profiles))
			out.Body.Meta.NextCursor = cursor
			for _, profile := range profiles {
				out.Body.Data = append(out.Body.Data, mapProfile(profile))
			}
			return out, nil
		})
	}

	huma.Register(api, huma.Operation{OperationID: "remove-profile-follower", Method: http.MethodDelete, Path: "/v1/me/followers/{userId}", Summary: "Remove an inbound follower", Security: security}, func(ctx context.Context, input *removeFollowerInput) (*relationshipOutput, error) {
		result, err := connections.RemoveFollower(ctx, input.Authorization, input.UserID, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &relationshipOutput{}
		out.Body.Data.Profile = mapProfile(result.Target)
		return out, nil
	})
}

type removeFollowerInput struct {
	Authorization  string `header:"Authorization"`
	UserID         string `path:"userId" minLength:"1" maxLength:"128"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
}
