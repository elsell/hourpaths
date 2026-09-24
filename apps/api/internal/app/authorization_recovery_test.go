package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestDeniedAuthorizationDeadLetterRecoveryIsAuditedWithoutOwnershipLeak(t *testing.T) {
	var events []audit.Event
	a := App{Clock: fakeClock{now: time.Now().UTC()}, Auth: fakeAuth{}, Users: fakeUsers{user: identity.User{ID: "attacker"}}, Audits: fakeAudits{events: &events}, AuditRateLimiter: fakeAuditRateLimiter{}, AuthorizationOutbox: fakeOutbox{requeueErr: ports.ErrNotFound}}
	if err := a.RequeueAuthorizationDeadLetter(context.Background(), "Bearer token", "secret-change"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("concealed recovery returned %v", err)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceAccessDenied || events[0].OwnerUserID != "attacker" || events[0].ActorUserID != "attacker" || events[0].TargetType != "authorization_change" || events[0].TargetID != "secret-change" || events[0].Outcome != audit.Denied {
		t.Fatalf("denied recovery audit leaked ownership or was omitted: %+v", events)
	}
	auditFailure := errors.New("audit unavailable")
	a.Audits = fakeAudits{err: auditFailure}
	if err := a.RequeueAuthorizationDeadLetter(context.Background(), "Bearer token", "secret-change"); !errors.Is(err, auditFailure) || errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("denial audit failure did not fail closed: %v", err)
	}
}

func TestAuthorizationDeadLetterRecoveryRejectsCredentialsAndLimitsBeforePersistence(t *testing.T) {
	var recoveredOwner string
	outbox := fakeOutbox{requeueOwner: &recoveredOwner}
	a := App{Clock: fakeClock{now: time.Now().UTC()}, Auth: fakeAuth{err: ports.ErrInvalidCredential}, Users: fakeUsers{}, Audits: fakeAudits{}, AuditRateLimiter: fakeAuditRateLimiter{}, AuthorizationOutbox: outbox}
	if err := a.RequeueAuthorizationDeadLetter(context.Background(), "Bearer malformed", "change"); !errors.Is(err, ports.ErrInvalidCredential) || errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("invalid credential classification was lost: %v", err)
	}
	if recoveredOwner != "" {
		t.Fatalf("invalid session reached persistence as %q", recoveredOwner)
	}
	a.Auth = fakeAuth{}
	a.Users = fakeUsers{user: identity.User{ID: "owner"}}
	a.AuditRateLimiter = fakeAuditRateLimiter{denied: true}
	if err := a.RequeueAuthorizationDeadLetter(context.Background(), "Bearer token", "change"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate-limited recovery returned %v", err)
	}
	if recoveredOwner != "" {
		t.Fatalf("rate-limited recovery reached persistence as %q", recoveredOwner)
	}
}
