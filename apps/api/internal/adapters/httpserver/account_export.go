package httpserver

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"net/http"
	"time"
)

type accountExportProfileDTO struct {
	UserID         string    `json:"userId"`
	Email          string    `json:"email"`
	Username       string    `json:"username"`
	DisplayName    string    `json:"displayName"`
	Description    string    `json:"description"`
	Visibility     string    `json:"visibility"`
	TimeZone       string    `json:"timeZone"`
	FirstDayOfWeek int       `json:"firstDayOfWeek"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type accountExportProfileOutput struct {
	Body struct {
		Data accountExportProfileDTO `json:"data"`
	}
}

func registerAccountExportRoutes(api huma.API, application app.App) {
	huma.Register(api, huma.Operation{OperationID: "export-own-profile", Method: http.MethodGet, Path: "/v1/me/export/profile", Summary: "Export the authenticated account's profile and calendar preferences", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *MeInput) (*accountExportProfileOutput, error) {
		profile, err := application.ExportOwnAccountProfile(ctx, input.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		output := &accountExportProfileOutput{}
		output.Body.Data = accountExportProfileDTO{UserID: profile.UserID, Email: profile.Email, Username: profile.Username, DisplayName: profile.DisplayName, Description: profile.Description, Visibility: string(profile.Visibility), TimeZone: profile.TimeZone, FirstDayOfWeek: profile.FirstDayOfWeek, CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt}
		return output, nil
	})
}
