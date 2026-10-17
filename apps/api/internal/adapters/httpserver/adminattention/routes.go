package adminattentionroutes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	app "github.com/elsell/hour-paths/apps/api/internal/app/adminattention"
	"net/http"
	"time"
)

type AdministratorInput struct {
	Authorization string `header:"Authorization"`
}
type readInput struct {
	AdministratorInput
	ID string `path:"id" format:"uuid"`
}
type AdminAttentionNotice struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind" enum:"report,appeal"`
	CreatedAt time.Time `json:"createdAt"`
	Read      bool      `json:"read"`
}
type AttentionListData struct {
	Items []AdminAttentionNotice `json:"items" nullable:"false"`
}
type AttentionListBody struct {
	Data AttentionListData `json:"data"`
}
type listOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         AttentionListBody
}
type AdminAttentionRecord struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind" enum:"report,appeal"`
	Reason      string    `json:"reason"`
	Explanation string    `json:"explanation"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"createdAt"`
}
type AttentionRecordBody struct {
	Data AdminAttentionRecord `json:"data"`
}
type readOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         AttentionRecordBody
}

func Register(api huma.API, s *app.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-admin-moderation-notifications", Method: http.MethodGet, Path: "/v1/admin/moderation-notifications", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, i *AdministratorInput) (*listOutput, error) {
		items, e := s.Notifications(ctx, i.Authorization)
		if e != nil {
			return nil, shared.MapError(e, true)
		}
		o := &listOutput{CacheControl: "no-store"}
		o.Body.Data.Items = []AdminAttentionNotice{}
		for _, n := range items {
			o.Body.Data.Items = append(o.Body.Data.Items, AdminAttentionNotice{n.ID, n.Kind, n.CreatedAt, n.Read})
		}
		return o, nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-admin-moderation-notification", Method: http.MethodGet, Path: "/v1/admin/moderation-notifications/{id}", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, i *readInput) (*readOutput, error) {
		r, e := s.Read(ctx, i.Authorization, i.ID)
		if e != nil {
			return nil, shared.MapError(e, true)
		}
		return &readOutput{CacheControl: "no-store", Body: AttentionRecordBody{Data: AdminAttentionRecord{r.ID, r.Kind, r.Reason, r.Explanation, r.State, r.CreatedAt}}}, nil
	})
}
