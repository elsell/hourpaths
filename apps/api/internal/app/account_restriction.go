package app

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

// AccountRestrictionRepository returns one currently effective, unreversed
// decision, preferring a ban. A nil decision means this account is unrestricted.
type AccountRestrictionRepository interface {
	ActiveAccountRestriction(context.Context, string, time.Time) (*moderation.Enforcement, error)
}

type AccountRestrictionReview struct {
	UserID   string
	Decision *moderation.Enforcement
}

type AccountAdmission interface {
	AdmitAccountUse(context.Context, string) error
}

func (a App) accountRestriction(ctx context.Context, authorization string) (AccountRestrictionReview, error) {
	if a.AccountRestrictions == nil || a.Auth == nil || a.Users == nil || a.Clock == nil || a.Audits == nil || a.AuditRateLimiter == nil {
		return AccountRestrictionReview{}, ports.ErrUnavailable
	}
	user, err := a.authenticateCurrentUser(ctx, authorization)
	if err != nil {
		return AccountRestrictionReview{}, err
	}
	now := a.Clock.Now().UTC()
	if now.IsZero() {
		return AccountRestrictionReview{}, ports.ErrUnavailable
	}
	decision, err := a.AccountRestrictions.ActiveAccountRestriction(ctx, user.ID, now)
	if err != nil {
		return AccountRestrictionReview{}, ports.ErrUnavailable
	}
	if decision != nil && (decision.SubjectUserID != user.ID || !decision.RestrictsAccount(now)) {
		return AccountRestrictionReview{}, ports.ErrUnavailable
	}
	return AccountRestrictionReview{UserID: user.ID, Decision: decision}, nil
}

// Admission does not revoke credentials: restricted users still need their
// authenticated notice, appeal and account-lifecycle routes.
func (a App) AdmitAccountUse(ctx context.Context, authorization string) error {
	review, err := a.accountRestriction(ctx, authorization)
	if err != nil {
		return err
	}
	if review.Decision == nil {
		return nil
	}
	if !a.AuditRateLimiter.Allow(review.UserID, a.Clock.Now().UTC()) {
		return ErrRateLimited
	}
	if err = a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, review.UserID, review.UserID, audit.ResourceAccessDenied, "account_restriction", review.UserID, audit.Denied)); err != nil {
		return err
	}
	return ports.ErrAccountRestricted
}

func (a App) CurrentAccountRestriction(ctx context.Context, authorization string) (AccountRestrictionReview, error) {
	review, err := a.accountRestriction(ctx, authorization)
	if err != nil {
		return AccountRestrictionReview{}, err
	}
	if !a.AuditRateLimiter.Allow(review.UserID, a.Clock.Now().UTC()) {
		return AccountRestrictionReview{}, ErrRateLimited
	}
	if err = a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, review.UserID, review.UserID, audit.ResourceViewed, "account_restriction", review.UserID, audit.Succeeded)); err != nil {
		return AccountRestrictionReview{}, err
	}
	return review, nil
}
