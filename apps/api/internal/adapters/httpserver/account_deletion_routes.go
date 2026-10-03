package httpserver

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
)

type accountDeletionInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		Confirmed      bool   `json:"confirmed"`
		ReviewedUserID string `json:"reviewedUserId" minLength:"1" maxLength:"128"`
		ReceiptSecret  string `json:"receiptSecret" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$"`
	}
}

type accountDeletionReceiptInput struct {
	Body struct {
		UserID        string `json:"userId" minLength:"1" maxLength:"128"`
		ReceiptSecret string `json:"receiptSecret" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$"`
	}
}

func registerAccountDeletionRoutes(api huma.API, application app.App) {
	huma.Register(api, huma.Operation{OperationID: "delete-account", Method: http.MethodPost, Path: "/v1/me/deletion", Summary: "Confirm permanent deletion of the reviewed account", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *accountDeletionInput) (*NoContentOutput, error) {
		if err := application.DeleteAccount(ctx, input.Authorization, app.AccountDeletionRequest{Confirmed: input.Body.Confirmed, ReviewedUserID: input.Body.ReviewedUserID, ReceiptSecret: input.Body.ReceiptSecret}); err != nil {
			return nil, mapError(err, false)
		}
		return &NoContentOutput{Status: http.StatusNoContent}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "confirm-account-deletion-receipt", Method: http.MethodPost, Path: "/v1/account-deletion/receipt", Summary: "Confirm completed deletion with its single-purpose receipt secret"}, func(ctx context.Context, input *accountDeletionReceiptInput) (*NoContentOutput, error) {
		if err := application.ConfirmAccountDeletion(ctx, input.Body.UserID, input.Body.ReceiptSecret); err != nil {
			return nil, mapError(err, false)
		}
		return &NoContentOutput{Status: http.StatusNoContent}, nil
	})
}
