package moderationroutes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	rootapp "github.com/elsell/hour-paths/apps/api/internal/app"
	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type enforcementStore struct {
	notice      app.Notice
	events      []audit.Event
	unavailable bool
}

func (f *enforcementStore) Now() time.Time               { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) }
func (f *enforcementStore) Allow(string, time.Time) bool { return true }
func (f *enforcementStore) Authenticate(_ context.Context, token string) (ports.Principal, error) {
	switch token {
	case "Bearer subject":
		return ports.Principal{UserID: "subject", Scopes: []string{"api:user"}}, nil
	case "Bearer other":
		return ports.Principal{UserID: "other", Scopes: []string{"api:user"}}, nil
	case "Bearer onboarding":
		return ports.Principal{UserID: "subject", Scopes: []string{"onboarding"}}, nil
	default:
		return ports.Principal{}, rootapp.ErrUnauthenticated
	}
}
func (f *enforcementStore) AppendAuditEvent(_ context.Context, event audit.Event) error {
	f.events = append(f.events, event)
	return nil
}
func (f *enforcementStore) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}
func (f *enforcementStore) ReadNotice(ctx context.Context, owner, id string, event audit.Event) (app.Notice, error) {
	if f.unavailable {
		return app.Notice{}, ports.ErrUnavailable
	}
	if owner != f.notice.Decision.SubjectUserID || id != f.notice.Decision.ID {
		return app.Notice{}, ports.ErrNotFound
	}
	f.AppendAuditEvent(ctx, event)
	return f.notice, nil
}
func (f *enforcementStore) ListNotices(ctx context.Context, owner string, _ ports.PageRequest, event audit.Event) (app.NoticePage, error) {
	if f.unavailable {
		return app.NoticePage{}, ports.ErrUnavailable
	}
	f.AppendAuditEvent(ctx, event)
	if owner != f.notice.Decision.SubjectUserID {
		return app.NoticePage{}, nil
	}
	return app.NoticePage{Items: []app.Notice{f.notice}}, nil
}
func (f *enforcementStore) SubmitAppeal(ctx context.Context, c app.AppealCommand) (domain.Appeal, error) {
	if c.OwnerID != f.notice.Decision.SubjectUserID {
		return domain.Appeal{}, ports.ErrNotFound
	}
	if f.notice.Appeal != nil {
		return domain.Appeal{}, ports.ErrConflict
	}
	f.notice.Appeal = &c.Appeal
	f.AppendAuditEvent(ctx, c.Audit)
	return c.Appeal, nil
}
func enforcementHandler(f *enforcementStore) http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("test", "1"))
	RegisterEnforcements(api, app.NewEnforcements(app.EnforcementDependencies{Auth: f, Repository: f, Audits: f, RateLimiter: f, Clock: f, CursorSigningKey: []byte(strings.Repeat("k", 32))}))
	return mux
}
func TestEnforcementHTTPAccountBoundaryAndPrivateAppeal(t *testing.T) {
	f := &enforcementStore{}
	f.notice = app.Notice{Decision: domain.Enforcement{ID: "notice", SubjectUserID: "subject", Action: domain.Warning, PolicyReason: "Harassment policy", IssuedAt: f.Now().Add(-time.Hour)}}
	h := enforcementHandler(f)
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", token)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "0123456789abcdef")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, token := range []string{"", "malformed", "Bearer expired", "Bearer wrong-issuer", "Bearer wrong-audience", "Bearer onboarding"} {
		for _, method := range []string{"GET", "POST"} {
			path := "/v1/enforcement-notices/notice"
			body := ""
			if method == "POST" {
				path += "/appeal"
				body = `{"explanation":"Context"}`
			}
			if w := request(method, path, token, body); w.Code != 401 {
				t.Fatalf("%s %s: %d %s", method, token, w.Code, w.Body)
			}
		}
	}
	if len(f.events) != 0 || f.notice.Appeal != nil {
		t.Fatal("unauthenticated request reached storage")
	}
	for _, path := range []string{"/v1/enforcement-notices/notice", "/v1/enforcement-notices/missing"} {
		if w := request("GET", path, "Bearer other", ""); w.Code != 404 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if w := request("POST", "/v1/enforcement-notices/notice/appeal", "Bearer other", `{"explanation":"Context"}`); w.Code != 404 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request("GET", "/v1/enforcement-notices", "Bearer other", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal(w.Code, w.Body)
	}
	if w := request("POST", "/v1/enforcement-notices/notice/appeal", "Bearer subject", `{"explanation":"My context"}`); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	f.notice.Appeal.Reviewer = "secret-reviewer"
	f.notice.Appeal.Outcome = domain.AppealUpheld
	f.notice.Appeal.DecisionReason = "Policy applies"
	f.notice.Appeal.DecidedAt = f.Now()
	for _, path := range []string{"/v1/enforcement-notices", "/v1/enforcement-notices/notice"} {
		w := request("GET", path, "Bearer subject", "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Policy applies") || strings.Contains(w.Body.String(), "secret-reviewer") || strings.Contains(w.Body.String(), "subjectUserId") {
			t.Fatal(w.Code, w.Body)
		}
	}
	f.unavailable = true
	if w := request("GET", "/v1/enforcement-notices/notice", "Bearer subject", ""); w.Code < 500 {
		t.Fatal("dependency failure did not fail closed", w.Code, w.Body)
	}
}
