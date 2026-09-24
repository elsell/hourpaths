package app

import (
	"context"
	"errors"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (a App) DeclineDuplicateEmailRecovery(ctx context.Context, authorization string) error {
	principal, err := a.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return err
	}
	if principal.UserID == "" {
		return ErrUnauthenticated
	}
	if len(principal.Scopes) != 1 || principal.Scopes[0] != "api:onboarding" {
		if a.AuditRateLimiter == nil || !a.AuditRateLimiter.Allow(principal.UserID, a.Clock.Now().UTC()) {
			return ErrRateLimited
		}
		if a.Audits == nil {
			return errors.New("audit dependency is invalid")
		}
		if err := a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "user", principal.UserID, audit.Denied)); err != nil {
			return err
		}
		return ErrUnauthenticated
	}
	u, err := a.Users.GetProvisionalUser(ctx, principal.UserID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return ErrUnauthenticated
		}
		return err
	}
	if u.ID != principal.UserID || u.Status != identity.StatusProvisional {
		return ErrUnauthenticated
	}
	if a.AuditRateLimiter == nil || !a.AuditRateLimiter.Allow(u.ID, a.Clock.Now().UTC()) {
		return ErrRateLimited
	}
	if a.DuplicateAccountRecoveryDeclines == nil {
		return errors.New("duplicate account recovery decline dependency is invalid")
	}
	event := a.auditEvent(ctx, u.ID, u.ID, audit.DuplicateEmailRecoveryDeclined, "user", u.ID, audit.Succeeded)
	if err := a.DuplicateAccountRecoveryDeclines.DeclineDuplicateEmailRecovery(ctx, u.ID, event); err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return ErrUnauthenticated
		}
		return err
	}
	return nil
}
