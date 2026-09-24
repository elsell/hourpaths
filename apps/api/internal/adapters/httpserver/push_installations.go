package httpserver

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
)

type PushInstallationInput struct {
	Authorization  string `header:"Authorization"`
	InstallationID string `path:"installationId" minLength:"1" maxLength:"128"`
	Body           struct {
		Provider string `json:"provider" enum:"expo"`
		Platform string `json:"platform" enum:"ios,android"`
		Locale   string `json:"locale" enum:"en,es"`
		Token    string `json:"token" minLength:"1" maxLength:"2048"`
	}
}

type DeletePushInstallationInput struct {
	Authorization  string `header:"Authorization"`
	InstallationID string `path:"installationId" minLength:"1" maxLength:"128"`
}

func registerPushInstallationRoutes(api huma.API, application app.App) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{
		OperationID: "upsert-push-installation",
		Method:      http.MethodPut,
		Path:        "/v1/push-installations/{installationId}",
		Summary:     "Register or refresh this application installation for push",
		Security:    security,
	}, func(ctx context.Context, input *PushInstallationInput) (*NoContentOutput, error) {
		err := application.UpsertPushInstallation(
			ctx, input.Authorization, input.InstallationID,
			input.Body.Provider, input.Body.Platform, input.Body.Locale, input.Body.Token,
		)
		if err != nil {
			return nil, mapError(err, true)
		}
		return &NoContentOutput{Status: http.StatusNoContent}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "delete-push-installation",
		Method:      http.MethodDelete,
		Path:        "/v1/push-installations/{installationId}",
		Summary:     "Stop push delivery to this application installation",
		Security:    security,
	}, func(ctx context.Context, input *DeletePushInstallationInput) (*NoContentOutput, error) {
		if err := application.DeletePushInstallation(
			ctx, input.Authorization, input.InstallationID,
		); err != nil {
			return nil, mapError(err, true)
		}
		return &NoContentOutput{Status: http.StatusNoContent}, nil
	})
}
