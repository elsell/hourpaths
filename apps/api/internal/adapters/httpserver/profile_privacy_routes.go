package httpserver

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"net/http"
)

type ownProfilePrivacyDTO struct {
	UserID     string                     `json:"userId"`
	Visibility identity.ProfileVisibility `json:"visibility" enum:"public,private"`
	Revision   int64                      `json:"revision"`
}

type ownProfilePrivacyOutput struct {
	Body struct {
		Data ownProfilePrivacyDTO `json:"data"`
	}
}

type profilePrivacyUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" minLength:"16" maxLength:"128"`
	Body           struct {
		Visibility       identity.ProfileVisibility `json:"visibility" enum:"public,private"`
		ExpectedRevision int64                      `json:"expectedRevision" minimum:"1"`
		Confirmed        bool                       `json:"confirmed"`
	}
}

func profilePrivacyOutput(profile app.OwnProfilePrivacy) *ownProfilePrivacyOutput {
	out := &ownProfilePrivacyOutput{}
	out.Body.Data.UserID = profile.UserID
	out.Body.Data.Visibility = profile.Visibility
	out.Body.Data.Revision = profile.Revision
	return out
}

func registerProfilePrivacyRoutes(api huma.API, application app.App) {
	huma.Register(api, huma.Operation{OperationID: "get-own-profile-privacy", Method: http.MethodGet, Path: "/v1/me/profile/privacy", Summary: "Read the signed-in account's profile privacy", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *MeInput) (*ownProfilePrivacyOutput, error) {
		profile, err := application.OwnProfilePrivacy(ctx, input.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		return profilePrivacyOutput(profile), nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-own-profile-privacy", Method: http.MethodPut, Path: "/v1/me/profile/privacy", Summary: "Confirm the signed-in account's profile privacy change", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *profilePrivacyUpdateInput) (*ownProfilePrivacyOutput, error) {
		profile, err := application.UpdateOwnProfilePrivacy(ctx, input.Authorization, input.IdempotencyKey, app.ProfilePrivacyUpdate{Visibility: input.Body.Visibility, ExpectedRevision: input.Body.ExpectedRevision, Confirmed: input.Body.Confirmed})
		if err != nil {
			return nil, mapError(err, false)
		}
		return profilePrivacyOutput(profile), nil
	})
}
