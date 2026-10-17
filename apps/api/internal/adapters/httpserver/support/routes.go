package supportroutes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	app "github.com/elsell/hour-paths/apps/api/internal/app/support"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/support"
	"net/http"
	"time"
)

type Message struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	Operator  bool      `json:"operator"`
	CreatedAt time.Time `json:"createdAt"`
}
type Conversation struct {
	ID        string    `json:"id"`
	Subject   string    `json:"subject"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Messages  []Message `json:"messages" nullable:"false"`
}
type output struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Data Conversation `json:"data"`
	}
}
type SupportListData struct {
	Items []Conversation `json:"items" nullable:"false"`
}
type SupportListMeta struct {
	NextCursor string `json:"nextCursor,omitempty"`
}
type SupportListBody struct {
	Data SupportListData `json:"data"`
	Meta SupportListMeta `json:"meta"`
}
type listOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         SupportListBody
}
type Notice struct {
	ID        string    `json:"id"`
	RequestID string    `json:"requestId"`
	CreatedAt time.Time `json:"createdAt"`
	Read      bool      `json:"read"`
}
type SupportNoticesData struct {
	Items []Notice `json:"items" nullable:"false"`
}
type SupportNoticesBody struct {
	Data SupportNoticesData `json:"data"`
}
type noticeOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         SupportNoticesBody
}
type createInput struct {
	Key  string `header:"X-Support-Key" required:"true" minLength:"64" maxLength:"64"`
	Body struct {
		ID      string `json:"id" pattern:"^[0-7][0-9A-HJKMNP-TV-Z]{25}$"`
		Subject string `json:"subject" minLength:"1" maxLength:"160"`
		Message string `json:"message" minLength:"1" maxLength:"5000"`
	}
}
type ReadInput struct {
	ID  string `path:"id" pattern:"^[0-7][0-9A-HJKMNP-TV-Z]{25}$"`
	Key string `header:"X-Support-Key" required:"true" minLength:"64" maxLength:"64"`
}
type replyInput struct {
	ReadInput
	Body struct {
		ID      string `json:"id" format:"uuid"`
		Message string `json:"message" minLength:"1" maxLength:"5000"`
	}
}
type AdminInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id" pattern:"^[0-7][0-9A-HJKMNP-TV-Z]{25}$"`
}
type adminReply struct {
	AdminInput
	Body struct {
		ID      string `json:"id" format:"uuid"`
		Message string `json:"message" minLength:"1" maxLength:"5000"`
	}
}
type inboxInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor" maxLength:"200"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"50"`
}

func project(c domain.Conversation) Conversation {
	v := Conversation{ID: c.ID, Subject: c.Subject, CreatedAt: c.CreatedAt, ExpiresAt: c.ExpiresAt, Messages: []Message{}}
	for _, m := range c.Messages {
		v.Messages = append(v.Messages, Message{m.ID, m.Body, m.Operator, m.CreatedAt})
	}
	return v
}
func result(c domain.Conversation, e error) (*output, error) {
	if e != nil {
		return nil, shared.MapError(e, true)
	}
	o := &output{CacheControl: "no-store"}
	o.Body.Data = project(c)
	return o, nil
}
func Register(api huma.API, s *app.Service) {
	huma.Register(api, huma.Operation{OperationID: "create-support-request", Method: http.MethodPost, Path: "/v1/support/requests", Summary: "Create a private support conversation", MaxBodyBytes: 32768, DefaultStatus: 201}, func(ctx context.Context, i *createInput) (*output, error) {
		return result(s.Create(ctx, i.Body.ID, i.Key, i.Body.Subject, i.Body.Message))
	})
	huma.Register(api, huma.Operation{OperationID: "get-support-request", Method: http.MethodGet, Path: "/v1/support/requests/{id}", Summary: "Read a conversation using its recovery credential"}, func(ctx context.Context, i *ReadInput) (*output, error) { return result(s.Read(ctx, i.ID, i.Key)) })
	huma.Register(api, huma.Operation{OperationID: "reply-support-request", Method: http.MethodPost, Path: "/v1/support/requests/{id}/messages", Summary: "Add a private follow-up", MaxBodyBytes: 32768}, func(ctx context.Context, i *replyInput) (*output, error) {
		return result(s.Reply(ctx, i.ID, i.Key, i.Body.ID, i.Body.Message))
	})
	huma.Register(api, huma.Operation{OperationID: "list-admin-support", Method: http.MethodGet, Path: "/v1/admin/support", Summary: "Read the platform administrator support inbox", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, i *inboxInput) (*listOutput, error) {
		items, e := s.Inbox(ctx, i.Authorization, i.Cursor, i.Limit)
		if e != nil {
			return nil, shared.MapError(e, true)
		}
		o := &listOutput{CacheControl: "no-store"}
		o.Body.Data.Items = []Conversation{}
		for _, c := range items {
			o.Body.Data.Items = append(o.Body.Data.Items, project(c))
		}
		if len(items) == i.Limit {
			o.Body.Meta.NextCursor = items[len(items)-1].ID
		}
		return o, nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-admin-support", Method: http.MethodGet, Path: "/v1/admin/support/{id}", Summary: "Read a support conversation as an administrator", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, i *AdminInput) (*output, error) {
		return result(s.AdminRead(ctx, i.Authorization, i.ID))
	})
	huma.Register(api, huma.Operation{OperationID: "reply-admin-support", Method: http.MethodPost, Path: "/v1/admin/support/{id}/messages", Summary: "Reply as support", Security: []map[string][]string{{"oidc": {}}}, MaxBodyBytes: 32768}, func(ctx context.Context, i *adminReply) (*output, error) {
		return result(s.AdminReply(ctx, i.Authorization, i.ID, i.Body.ID, i.Body.Message))
	})
	huma.Register(api, huma.Operation{OperationID: "list-admin-support-notifications", Method: http.MethodGet, Path: "/v1/admin/support-notifications", Summary: "Read private support notifications", Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, i *inboxInput) (*noticeOutput, error) {
		items, e := s.Notifications(ctx, i.Authorization)
		if e != nil {
			return nil, shared.MapError(e, true)
		}
		o := &noticeOutput{CacheControl: "no-store"}
		o.Body.Data.Items = []Notice{}
		for _, n := range items {
			o.Body.Data.Items = append(o.Body.Data.Items, Notice{n.ID, n.RequestID, n.CreatedAt, n.Read})
		}
		return o, nil
	})
}
