package moderationroutes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"net/http"
	"time"
)

type EnforcementService interface {
	List(context.Context, string, string, int) ([]app.Notice, string, error)
	Get(context.Context, string, string) (app.Notice, error)
	Appeal(context.Context, string, string, string, string) (domain.Appeal, error)
}
type EnforcementAppealData struct {
	ID             string     `json:"id"`
	Explanation    string     `json:"explanation"`
	SubmittedAt    time.Time  `json:"submittedAt"`
	Outcome        string     `json:"outcome,omitempty"`
	DecisionReason string     `json:"decisionReason,omitempty"`
	DecidedAt      *time.Time `json:"decidedAt,omitempty"`
}
type EnforcementCommentData struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}
type EnforcementNoticeData struct {
	AffectedComment *EnforcementCommentData `json:"affectedComment,omitempty"`
	ID              string                  `json:"id"`
	Action          string                  `json:"action"`
	PolicyReason    string                  `json:"policyReason"`
	IssuedAt        time.Time               `json:"issuedAt"`
	Until           *time.Time              `json:"until,omitempty"`
	AppealDeadline  time.Time               `json:"appealDeadline"`
	Appeal          *EnforcementAppealData  `json:"appeal,omitempty"`
}
type noticeInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id" minLength:"1" maxLength:"200"`
}
type noticeOutput struct {
	Body struct {
		Data EnforcementNoticeData `json:"data"`
	}
}
type noticeListInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor" maxLength:"4096"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type EnforcementNoticeListData struct {
	Items []EnforcementNoticeData `json:"items"`
}
type EnforcementNoticeListMeta struct {
	NextCursor string `json:"nextCursor,omitempty"`
}
type EnforcementNoticeListBody struct {
	Data EnforcementNoticeListData `json:"data"`
	Meta EnforcementNoticeListMeta `json:"meta"`
}
type noticeListOutput struct{ Body EnforcementNoticeListBody }
type appealInput struct {
	Authorization  string `header:"Authorization"`
	ID             string `path:"id" minLength:"1" maxLength:"200"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           struct {
		Explanation string `json:"explanation"`
	}
}
type appealOutput struct {
	Body struct {
		Data EnforcementAppealData `json:"data"`
	}
}

func appealData(a domain.Appeal) EnforcementAppealData {
	d := EnforcementAppealData{ID: a.ID, Explanation: a.Explanation, SubmittedAt: a.SubmittedAt, Outcome: string(a.Outcome), DecisionReason: a.DecisionReason}
	if !a.DecidedAt.IsZero() {
		at := a.DecidedAt
		d.DecidedAt = &at
	}
	return d
}
func noticeData(n app.Notice) EnforcementNoticeData {
	e := n.Decision
	d := EnforcementNoticeData{ID: e.ID, Action: string(e.Action), PolicyReason: e.PolicyReason, IssuedAt: e.IssuedAt, AppealDeadline: e.IssuedAt.Add(30 * 24 * time.Hour)}
	if e.AffectedCommentID != "" && !e.AffectedCommentCreatedAt.IsZero() {
		d.AffectedComment = &EnforcementCommentData{ID: e.AffectedCommentID, CreatedAt: e.AffectedCommentCreatedAt}
	}
	if !e.Until.IsZero() {
		until := e.Until
		d.Until = &until
	}
	if n.Appeal != nil {
		a := appealData(*n.Appeal)
		d.Appeal = &a
	}
	return d
}
func RegisterEnforcements(api huma.API, service EnforcementService) {
	huma.Register(api, huma.Operation{OperationID: "list-enforcement-notices", Method: http.MethodGet, Path: "/v1/enforcement-notices", Summary: "List your private enforcement notices", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, in *noticeListInput) (*noticeListOutput, error) {
		items, next, err := service.List(ctx, in.Authorization, in.Cursor, in.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &noticeListOutput{}
		out.Body.Data.Items = make([]EnforcementNoticeData, 0, len(items))
		for _, n := range items {
			out.Body.Data.Items = append(out.Body.Data.Items, noticeData(n))
		}
		out.Body.Meta.NextCursor = next
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-enforcement-notice", Method: http.MethodGet, Path: "/v1/enforcement-notices/{id}", Summary: "Read your private enforcement notice", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, in *noticeInput) (*noticeOutput, error) {
		n, err := service.Get(ctx, in.Authorization, in.ID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &noticeOutput{}
		out.Body.Data = noticeData(n)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "submit-enforcement-appeal", Method: http.MethodPost, Path: "/v1/enforcement-notices/{id}/appeal", Summary: "Appeal your enforcement decision", Security: []map[string][]string{{"oidc": {}}}, MaxBodyBytes: 16384, DefaultStatus: http.StatusCreated}, func(ctx context.Context, in *appealInput) (*appealOutput, error) {
		a, err := service.Appeal(ctx, in.Authorization, in.ID, in.IdempotencyKey, in.Body.Explanation)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &appealOutput{}
		out.Body.Data = appealData(a)
		return out, nil
	})
}
