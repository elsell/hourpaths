package path

import (
	"context"
	"errors"
	"testing"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestGetPathRequiresApplicationSessionBeforeDependencies(t *testing.T) {
	for _, authorization := range []string{"", "Bearer malformed", "Bearer oidc.header.payload"} {
		t.Run(authorization, func(t *testing.T) {
			var checks []authorizationCall
			var gets []repositoryGet
			dependencies := configuredDependencies()
			dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
			dependencies.Repository = controlledRepository{gets: &gets}
			_, err := New(dependencies).Get(context.Background(), authorization, "path-id")
			if !errors.Is(err, ports.ErrInvalidCredential) {
				t.Fatalf("credential returned %v", err)
			}
			if len(checks) != 0 || len(gets) != 0 {
				t.Fatalf("invalid credential reached protected dependencies: checks=%+v gets=%+v", checks, gets)
			}
		})
	}

	dependencies := configuredDependencies()
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "member", Scopes: []string{"openid"}}}
	if _, err := New(dependencies).Get(context.Background(), "Bearer valid", "path-id"); !errors.Is(err, platformapp.ErrUnauthenticated) {
		t.Fatalf("wrong application-session scope returned %v", err)
	}
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "member", Scopes: []string{"api:user", "api:onboarding"}}}
	if _, err := New(dependencies).Get(context.Background(), "Bearer valid", "path-id"); !errors.Is(err, platformapp.ErrUnauthenticated) {
		t.Fatalf("mixed application-session scopes returned %v", err)
	}
}

func TestGetPathChecksPermissionBeforeScopedRepositoryAndAuditsSuccess(t *testing.T) {
	var checks []authorizationCall
	var gets []repositoryGet
	var events []audit.Event
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	dependencies.Repository = controlledRepository{entity: domain.Entity{ID: "path-id", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Guitar", Visibility: "private"}}, gets: &gets}
	dependencies.Audits = controlledAudits{events: &events}
	result, err := New(dependencies).Get(platformapp.WithCorrelationID(context.Background(), "request-id"), "Bearer valid", "path-id")
	if err != nil || result.ID != "path-id" {
		t.Fatalf("Get() = %+v, %v", result, err)
	}
	if len(checks) != 1 || checks[0] != (authorizationCall{"path", "path-id", "view", "member"}) {
		t.Fatalf("authorization check = %+v", checks)
	}
	if len(gets) != 1 || gets[0] != (repositoryGet{"member", "path-id"}) {
		t.Fatalf("scoped repository read = %+v", gets)
	}
	if len(events) != 1 {
		t.Fatalf("audit events = %+v", events)
	}
	event := events[0]
	if event.Action != audit.ResourceViewed || event.OwnerUserID != "creator" || event.ActorUserID != "member" || event.TargetType != "path" || event.TargetID != "path-id" || event.Outcome != audit.Succeeded || event.CorrelationID != "request-id" || !event.OccurredAt.Equal(dependencies.Clock.Now()) {
		t.Fatalf("read audit = %+v", event)
	}
}

func TestGetPathConcealsDeniedAndAuditFailureFailsClosed(t *testing.T) {
	var events []audit.Event
	var gets []repositoryGet
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{allowed: false}
	dependencies.Repository = controlledRepository{gets: &gets}
	dependencies.Audits = controlledAudits{events: &events}
	_, err := New(dependencies).Get(context.Background(), "Bearer valid", "secret")
	if !errors.Is(err, platformapp.ErrForbidden) {
		t.Fatalf("denial returned %v", err)
	}
	if len(gets) != 0 {
		t.Fatalf("denial reached repository: %+v", gets)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceAccessDenied || events[0].OwnerUserID != "member" || events[0].ActorUserID != "member" || events[0].TargetType != "path" || events[0].TargetID != "secret" || events[0].Outcome != audit.Denied {
		t.Fatalf("denial audit leaked ownership or was omitted: %+v", events)
	}

	auditFailure := errors.New("audit unavailable")
	dependencies.Audits = controlledAudits{err: auditFailure}
	if _, err := New(dependencies).Get(context.Background(), "Bearer valid", "secret"); !errors.Is(err, auditFailure) || errors.Is(err, platformapp.ErrForbidden) {
		t.Fatalf("audit failure was hidden by denial: %v", err)
	}
}

func TestGetPathPreservesDependencyFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*Dependencies, error)
	}{
		{"authentication", func(d *Dependencies, err error) { d.Auth = controlledAuthenticator{err: err} }},
		{"authorization", func(d *Dependencies, err error) { d.Authorizer = controlledAuthorizer{err: err} }},
		{"repository", func(d *Dependencies, err error) { d.Repository = controlledRepository{err: err} }},
		{"audit", func(d *Dependencies, err error) { d.Audits = controlledAudits{err: err} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			failure := errors.New(test.name + " unavailable")
			dependencies := configuredDependencies()
			test.set(&dependencies, failure)
			if _, err := New(dependencies).Get(context.Background(), "Bearer valid", "path-id"); !errors.Is(err, failure) || errors.Is(err, ports.ErrInvalidCredential) || errors.Is(err, platformapp.ErrForbidden) {
				t.Fatalf("dependency failure was reclassified: %v", err)
			}
		})
	}
}

func TestGetPathRateLimitRunsBeforeAuthorization(t *testing.T) {
	var checks []authorizationCall
	var gets []repositoryGet
	dependencies := configuredDependencies()
	dependencies.AuditRateLimiter = controlledLimiter{denied: true}
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	dependencies.Repository = controlledRepository{gets: &gets}
	if _, err := New(dependencies).Get(context.Background(), "Bearer valid", "path-id"); !errors.Is(err, platformapp.ErrRateLimited) {
		t.Fatalf("rate limit returned %v", err)
	}
	if len(checks) != 0 || len(gets) != 0 {
		t.Fatalf("rate-limited read reached dependencies: checks=%+v gets=%+v", checks, gets)
	}
}
