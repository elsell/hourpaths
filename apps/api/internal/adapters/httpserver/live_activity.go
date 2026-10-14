package httpserver

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/liveactivity"
	"net/http"
	"time"
)

type LiveActivityRegistrationInput struct {
	Authorization string `header:"Authorization"`
	ActivityID    string `path:"activityId" minLength:"1" maxLength:"128"`
	Body          struct {
		AccountLease string `json:"accountLease" minLength:"1" maxLength:"128"`
		// Validate token content in the application layer so schema errors never
		// reflect a credential value into the response.
		Token      string      `json:"token" writeOnly:"true"`
		ExpiresAt  time.Time   `json:"expiresAt"`
		Boundaries []time.Time `json:"boundaries" maxItems:"64"`
	}
}
type LiveActivityRevocationInput struct {
	Authorization string `header:"Authorization"`
	ActivityID    string `path:"activityId" minLength:"1" maxLength:"128"`
	Body          struct {
		AccountLease string `json:"accountLease" minLength:"1" maxLength:"128"`
	}
}

func registerLiveActivityRoutes(api huma.API, application app.App) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{OperationID: "upsert-live-activity", Method: http.MethodPut, Path: "/v1/live-activities/{activityId}", Summary: "Register this Live Activity for private refresh signals", Security: security, MaxBodyBytes: 8192}, func(ctx context.Context, in *LiveActivityRegistrationInput) (*NoContentOutput, error) {
		err := application.UpsertLiveActivity(ctx, in.Authorization, liveactivity.Registration{ID: in.ActivityID, AccountLease: in.Body.AccountLease, Token: in.Body.Token, ExpiresAt: in.Body.ExpiresAt, Boundaries: in.Body.Boundaries})
		if err != nil {
			return nil, mapError(err, true)
		}
		return &NoContentOutput{Status: http.StatusNoContent}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "revoke-live-activity", Method: http.MethodDelete, Path: "/v1/live-activities/{activityId}", Summary: "Revoke refresh signals for this Live Activity lease", Security: security, MaxBodyBytes: 512}, func(ctx context.Context, in *LiveActivityRevocationInput) (*NoContentOutput, error) {
		if err := application.RevokeLiveActivity(ctx, in.Authorization, in.ActivityID, in.Body.AccountLease); err != nil {
			return nil, mapError(err, true)
		}
		return &NoContentOutput{Status: http.StatusNoContent}, nil
	})
}
