package moderationroutes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
)

type Service interface {
	Submit(context.Context, string, domain.Target, domain.Reason, string, string) (app.Receipt, error)
}
type reportInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           struct {
		TargetKind  string `json:"targetKind" enum:"profile,path,feed_event,comment,nudge"`
		TargetID    string `json:"targetId" minLength:"1" maxLength:"200"`
		Reason      string `json:"reason" enum:"spam_or_scam,harassment_or_bullying,hate_or_abusive_content,sexual_or_inappropriate_content,impersonation,privacy_or_personal_information,dangerous_or_self_harm_content,something_else"`
		Explanation string `json:"explanation,omitempty"`
	}
}
type ReportBlockIdentityData struct {
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}
type ReportReceiptData struct {
	ID          string                   `json:"id"`
	BlockTarget *ReportBlockIdentityData `json:"blockTarget,omitempty"`
}
type reportOutput struct {
	Body struct {
		Data ReportReceiptData `json:"data"`
	}
}

func Register(api huma.API, service Service) {
	huma.Register(api, huma.Operation{OperationID: "submit-report", Method: http.MethodPost, Path: "/v1/reports", Summary: "Submit a private report for currently accessible content", Security: []map[string][]string{{"oidc": {}}}, DefaultStatus: http.StatusCreated, MaxBodyBytes: 16384}, func(ctx context.Context, input *reportInput) (*reportOutput, error) {
		receipt, err := service.Submit(ctx, input.Authorization, domain.Target{Kind: domain.TargetKind(input.Body.TargetKind), ID: input.Body.TargetID}, domain.Reason(input.Body.Reason), input.Body.Explanation, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &reportOutput{}
		out.Body.Data.ID = receipt.ID
		if receipt.BlockTarget != nil {
			out.Body.Data.BlockTarget = &ReportBlockIdentityData{UserID: receipt.BlockTarget.UserID, Username: receipt.BlockTarget.Username, DisplayName: receipt.BlockTarget.DisplayName}
		}
		return out, nil
	})
}
