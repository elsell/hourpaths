package httpserver

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"net/http"
	"time"
)

type AccountRestrictionData struct {
	NoticeID     string     `json:"noticeId"`
	Action       string     `json:"action" enum:"suspension,ban"`
	PolicyReason string     `json:"policyReason"`
	IssuedAt     time.Time  `json:"issuedAt"`
	Until        *time.Time `json:"until,omitempty"`
}
type AccountRestrictionReviewData struct {
	Restriction *AccountRestrictionData `json:"restriction"`
}
type accountRestrictionOutput struct {
	Body struct {
		Data AccountRestrictionReviewData `json:"data"`
	}
}

func registerAccountRestrictionRoute(api huma.API, application app.App) {
	huma.Register(api, huma.Operation{OperationID: "get-account-restriction", Method: http.MethodGet, Path: "/v1/me/restriction", Summary: "Review the current account restriction", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, in *MeInput) (*accountRestrictionOutput, error) {
		review, err := application.CurrentAccountRestriction(ctx, in.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &accountRestrictionOutput{}
		if d := review.Decision; d != nil {
			data := &AccountRestrictionData{NoticeID: d.ID, Action: string(d.Action), PolicyReason: d.PolicyReason, IssuedAt: d.IssuedAt}
			if !d.Until.IsZero() {
				until := d.Until
				data.Until = &until
			}
			out.Body.Data.Restriction = data
		}
		return out, nil
	})
}
