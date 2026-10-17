package gormstore

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"github.com/danielgtaylor/huma/v2/humatest"
	attentionroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/adminattention"
	supportroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/support"
	platform "github.com/elsell/hour-paths/apps/api/internal/app"
	attentionapp "github.com/elsell/hour-paths/apps/api/internal/app/adminattention"
	supportapp "github.com/elsell/hour-paths/apps/api/internal/app/support"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/support"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"strings"
	"testing"
	"time"
)

func TestPrivateSupportAndAdminModerationBoundaries(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtime, e := Open("postgres", *postgresTestDSN)
	if e != nil {
		t.Fatal(e)
	}
	admin, e := Open("postgres", *migrationPostgresTestDSN)
	if e != nil {
		t.Fatal(e)
	}
	closeSocialProfileTestStore(t, runtime)
	closeSocialProfileTestStore(t, admin)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	operator, foreign := uuid.NewString(), uuid.NewString()
	for _, id := range []string{operator, foreign} {
		if e = admin.DB.Exec("INSERT INTO user_models(id,status,display_name,created_at,updated_at) VALUES(?,'active','Support test',?,?)", id, at, at).Error; e != nil {
			t.Fatal(e)
		}
	}
	if e = admin.DB.Exec("INSERT INTO identity_models(issuer,subject,user_id) VALUES('https://support.example',?,?)", operator, operator).Error; e != nil {
		t.Fatal(e)
	}
	if e = runtime.DB.Exec("SELECT grant_platform_admin(?,'https://support.example',?)", operator, operator).Error; e == nil {
		t.Fatal("runtime can grant administration")
	}
	if e = admin.DB.Exec("SELECT grant_platform_admin(?,'https://wrong.example',?)", operator, operator).Error; e == nil {
		t.Fatal("wrong issuer granted administration")
	}
	if e = admin.DB.Exec("SELECT grant_platform_admin(?,'https://support.example',?)", operator, operator).Error; e != nil {
		t.Fatal(e)
	}
	r := SupportRepository{DB: runtime.DB}
	hash, _ := domain.KeyDigest(strings.Repeat("a", 64))
	id := supportTestTicketID(t)
	c := domain.Conversation{ID: id, KeyHash: hash, Subject: "Account access", CreatedAt: at, ExpiresAt: at.Add(domain.Lifetime), Messages: []domain.Message{{ID: id, Body: "Private request <script>", CreatedAt: at}}}
	for n := 0; n < 2; n++ {
		if _, e = r.Create(ctx, c); e != nil {
			t.Fatal(e)
		}
	}
	invalid := c
	invalid.ID = "12345"
	invalid.Messages = []domain.Message{{ID: uuid.NewString(), Body: "Invalid identifier", CreatedAt: at}}
	if _, e = r.Create(ctx, invalid); e == nil {
		t.Fatal("database accepted a sequential ticket identifier")
	}
	ns, e := r.Notifications(ctx, operator, at)
	if e != nil || len(ns) != 1 {
		t.Fatalf("support notices=%d err=%v", len(ns), e)
	}
	if _, e = r.Get(ctx, id, "bad", "", at); !errors.Is(e, ports.ErrNotFound) {
		t.Fatalf("foreign recovery=%v", e)
	}
	if _, e = r.Get(ctx, id, "", foreign, at); !errors.Is(e, ports.ErrNotFound) {
		t.Fatalf("foreign admin=%v", e)
	}
	m := domain.Message{ID: uuid.NewString(), Body: "We can help", Operator: true, CreatedAt: at.Add(time.Second)}
	if _, e = r.Reply(ctx, id, "", operator, m, at); e != nil {
		t.Fatal(e)
	}
	m.Operator = false
	m.ID = uuid.NewString()
	m.Body = "Thanks"
	if _, e = r.Reply(ctx, id, hash, "", m, at); e != nil {
		t.Fatal(e)
	}
	if _, e = r.Reply(ctx, id, hash, "", m, at); e != nil {
		t.Fatal(e)
	}
	got, e := r.Get(ctx, id, hash, "", at)
	if e != nil || len(got.Messages) != 3 {
		t.Fatalf("reply roundtrip=%+v err=%v", got, e)
	}
	if _, e = r.Get(ctx, id, hash, "", c.ExpiresAt); !errors.Is(e, ports.ErrNotFound) {
		t.Fatal("expired recovery admitted")
	}
	if _, e = r.Create(ctx, c); e != nil {
		t.Fatal("create retry after follow-up", e)
	}
	// Exercise actual HTTP binding and projections, including the reply route.
	auth := &supportBoundary{actor: operator, at: at, allowed: true}
	service := &supportapp.Service{Repository: r, Auth: auth, Authorizer: auth, Clock: auth, Limiter: auth}
	_, api := humatest.New(t)
	supportroutes.Register(api, service)
	attentionroutes.Register(api, &attentionapp.Service{Administrator: service, Repository: AdminAttentionRepository{DB: runtime.DB}, Clock: auth})
	for _, route := range []string{"/v1/admin/support", "/v1/admin/support-notifications", "/v1/admin/moderation-notifications", "/v1/admin/support/" + id} {
		for _, token := range []string{"", "invalid-session", "expired-session", "onboarding-session", "foreign-session"} {
			response := api.Get(route, "Authorization: "+token)
			if response.Code != 401 && response.Code != 404 {
				t.Fatalf("unauthorized %s status=%d", route, response.Code)
			}
		}
	}
	response := api.Get("/v1/support/requests/"+id, "X-Support-Key: "+strings.Repeat("a", 64))
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), hash) {
		t.Fatal("private response or cache headers", response.Code)
	}
	response = api.Post("/v1/support/requests/"+id+"/messages", "X-Support-Key: "+strings.Repeat("a", 64), map[string]string{"id": uuid.NewString(), "message": "Follow-up through HTTP"})
	if response.Code != 200 {
		t.Fatal("reply binding", response.Code, response.Body.String())
	}
	auth.allowed = false
	if response = api.Get("/v1/admin/support", "Authorization: admin-session"); response.Code != 404 {
		t.Fatal("SpiceDB denial ignored", response.Code)
	}
	auth.dependencyFailure = true
	if response = api.Get("/v1/admin/support", "Authorization: admin-session"); response.Code < 500 {
		t.Fatal("dependency failure admitted", response.Code)
	}
	auth.dependencyFailure = false
	auth.allowed = true
	// New reports and appeals create recipient-scoped notices, never evidence.
	caseID, enforcement := uuid.NewString(), uuid.NewString()
	if e = runtime.DB.Exec("INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'profile',?,'something_else','Private report','{}',?)", caseID, foreign, foreign, foreign, at).Error; e != nil {
		t.Fatal(e)
	}
	if e = admin.DB.Exec("INSERT INTO moderation_enforcement_models(id,case_id,subject_user_id,action,policy_reason,issued_at) VALUES(?,?,?,'warning','Policy reason',?)", enforcement, caseID, foreign, at).Error; e != nil {
		t.Fatal(e)
	}
	if e = runtime.DB.Exec("INSERT INTO moderation_appeal_models(enforcement_id,id,explanation,submitted_at) VALUES(?,?,?,?)", enforcement, uuid.NewString(), "Private appeal", at).Error; e != nil {
		t.Fatal(e)
	}
	a := AdminAttentionRepository{DB: runtime.DB}
	notices, e := a.Notifications(ctx, operator, at)
	if e != nil || len(notices) != 2 {
		t.Fatalf("moderation notices=%+v err=%v", notices, e)
	}
	for _, n := range notices {
		if _, e = a.Read(ctx, foreign, n.ID, at); !errors.Is(e, ports.ErrNotFound) {
			t.Fatal("foreign notification admitted", e)
		}
		record, e := a.Read(ctx, operator, n.ID, at)
		if e != nil || record.Explanation == "" {
			t.Fatalf("record=%+v err=%v", record, e)
		}
	}
	if e = runtime.DB.Exec("SELECT id FROM moderation_case_models LIMIT 1").Error; e == nil {
		t.Fatal("runtime gained broad evidence read")
	}
	if e = runtime.DB.Exec("UPDATE support_audit_models SET actor='tampered'").Error; e == nil {
		t.Fatal("audit mutable")
	}
	// Failure to record audit must roll back message and recipient notifications.
	tx := runtime.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if e = admin.DB.Exec("REVOKE INSERT ON support_audit_models FROM app").Error; e != nil {
		t.Fatal(e)
	}
	blocked := c
	blocked.ID = supportTestTicketID(t)
	blocked.Messages = []domain.Message{{ID: blocked.ID, Body: "Must roll back", CreatedAt: at}}
	_, e = (SupportRepository{DB: tx}).Create(ctx, blocked)
	_ = tx.Rollback().Error
	if restore := admin.DB.Exec("GRANT INSERT ON support_audit_models TO app").Error; restore != nil {
		t.Fatal(restore)
	}
	if e == nil {
		t.Fatal("unaudited create succeeded")
	}
	var count int64
	admin.DB.Table("support_request_models").Where("id=?", blocked.ID).Count(&count)
	if count != 0 {
		t.Fatal("failed audit persisted request")
	}
	if e = admin.DB.Exec("DELETE FROM platform_admin_models WHERE user_id=?", operator).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = r.Notifications(ctx, operator, at); !errors.Is(e, ports.ErrNotFound) {
		t.Fatal("revoked support notification access", e)
	}
	if _, e = a.Notifications(ctx, operator, at); !errors.Is(e, ports.ErrNotFound) {
		t.Fatal("revoked moderation notification access", e)
	}
	if _, e = a.Read(ctx, operator, notices[0].ID, at); !errors.Is(e, ports.ErrNotFound) {
		t.Fatal("revoked moderation record access", e)
	}
}

type supportBoundary struct {
	actor                      string
	at                         time.Time
	allowed, dependencyFailure bool
}

func (f *supportBoundary) Now() time.Time               { return f.at }
func (f *supportBoundary) Allow(string, time.Time) bool { return true }
func (f *supportBoundary) Authenticate(_ context.Context, token string) (ports.Principal, error) {
	switch token {
	case "admin-session":
		return ports.Principal{UserID: f.actor, Scopes: []string{"api:user"}}, nil
	case "onboarding-session":
		return ports.Principal{UserID: f.actor, Scopes: []string{"api:onboarding"}}, nil
	case "foreign-session":
		return ports.Principal{UserID: "ordinary-account", Scopes: []string{"api:user"}}, nil
	default:
		return ports.Principal{}, platform.ErrUnauthenticated
	}
}
func (f *supportBoundary) Check(context.Context, string, string, string, string) (bool, error) {
	if f.dependencyFailure {
		return false, ports.ErrUnavailable
	}
	return f.allowed, nil
}
func (f *supportBoundary) WriteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *supportBoundary) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}

func supportTestTicketID(t *testing.T) string {
	t.Helper()
	var entropy [10]byte
	if _, e := rand.Read(entropy[:]); e != nil {
		t.Fatal(e)
	}
	return "01ARZ3NDEK" + base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding).EncodeToString(entropy[:])
}
