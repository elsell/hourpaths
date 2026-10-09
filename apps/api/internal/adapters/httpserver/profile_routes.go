package httpserver

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

type ownProfileDTO struct {
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Revision    int64  `json:"revision"`
}

type ownProfileOutput struct {
	Body struct {
		Data ownProfileDTO `json:"data"`
	}
}

type ownProfileUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" minLength:"16" maxLength:"128"`
	Body           struct {
		Username         string `json:"username" minLength:"3" maxLength:"64" pattern:"^[A-Za-z0-9_.]+$"`
		DisplayName      string `json:"displayName" minLength:"1" maxLength:"100"`
		Description      string `json:"description" maxLength:"500"`
		ExpectedRevision int64  `json:"expectedRevision" minimum:"1"`
	}
}

func profileOutput(profile app.OwnProfile) *ownProfileOutput {
	out := &ownProfileOutput{}
	out.Body.Data = ownProfileDTO{UserID: profile.UserID, Username: profile.Text.Username, DisplayName: profile.Text.DisplayName, Description: profile.Text.Description, Revision: profile.Revision}
	return out
}

func registerProfileRoutes(api huma.API, application app.App) {
	huma.Register(api, huma.Operation{OperationID: "get-own-profile", Method: http.MethodGet, Path: "/v1/me/profile", Summary: "Read the signed-in account's editable profile", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *MeInput) (*ownProfileOutput, error) {
		profile, err := application.OwnProfile(ctx, input.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		return profileOutput(profile), nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-own-profile", Method: http.MethodPut, Path: "/v1/me/profile", Summary: "Save the signed-in account's profile text", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *ownProfileUpdateInput) (*ownProfileOutput, error) {
		profile, err := application.UpdateOwnProfile(ctx, input.Authorization, input.IdempotencyKey, app.ProfileUpdate{ExpectedRevision: input.Body.ExpectedRevision, Text: identity.ProfileText{Username: input.Body.Username, DisplayName: input.Body.DisplayName, Description: input.Body.Description}})
		if err != nil {
			return nil, mapError(err, false)
		}
		return profileOutput(profile), nil
	})
}
