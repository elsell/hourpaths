package social

import (
	"context"
	"crypto/sha256"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (service *Service) GetTimerSubscription(ctx context.Context, authorization, rawScope, subject string) (TimerSubscription, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return TimerSubscription{}, err
	}
	scope := TimerSubscriptionScope(rawScope)
	if _, err := service.authorizeTimerSubscription(ctx, principal.UserID, scope, subject); err != nil {
		return TimerSubscription{}, err
	}
	preference, err := service.TimerSubscriptions.GetTimerSubscription(ctx, principal.UserID, scope, subject)
	if err != nil {
		return TimerSubscription{}, service.timerSubscriptionFailure(ctx, principal.UserID, subject, err)
	}
	if preference.Revision < 0 {
		return TimerSubscription{}, errInvalidDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "timer_subscription", subject, audit.Succeeded)
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return TimerSubscription{}, err
	}
	return preference, nil
}

func (service *Service) UpdateTimerSubscription(ctx context.Context, authorization, rawScope, subject, key string, revision int64, enabled bool) (TimerSubscription, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return TimerSubscription{}, err
	}
	if !validRelationshipIdempotencyKey(key) || revision < 0 || revision == math.MaxInt64 {
		return TimerSubscription{}, ports.ErrInvalidArgument
	}
	scope := TimerSubscriptionScope(rawScope)
	now, err := service.authorizeTimerSubscription(ctx, principal.UserID, scope, subject)
	if err != nil {
		return TimerSubscription{}, err
	}
	digest := sha256.Sum256([]byte(UpdateTimerSubscriptionOperation + "\x00" + rawScope + "\x00" + subject + "\x00" + strconv.FormatInt(revision, 10) + "\x00" + strconv.FormatBool(enabled)))
	command := TimerSubscriptionCommand{ActorUserID: principal.UserID, SubjectID: subject, Scope: scope, Enabled: enabled, ExpectedRevision: revision, OccurredAt: now,
		Idempotency: ports.Idempotency{PrincipalID: principal.UserID, Operation: UpdateTimerSubscriptionOperation, Key: key, RequestHash: digest[:]},
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "timer_subscription", subject, audit.Succeeded),
	}
	command.Audit.OccurredAt = now
	result, err := service.TimerSubscriptions.UpdateTimerSubscription(ctx, command)
	if err != nil {
		return TimerSubscription{}, service.timerSubscriptionFailure(ctx, principal.UserID, subject, err)
	}
	if result.Preference.Enabled != enabled || result.Preference.Revision != revision+1 {
		return TimerSubscription{}, errInvalidDependencies
	}
	return result.Preference, nil
}

func (service *Service) authorizeTimerSubscription(ctx context.Context, actor string, scope TimerSubscriptionScope, subject string) (time.Time, error) {
	if !scope.Valid() || strings.TrimSpace(subject) == "" || strings.TrimSpace(subject) != subject || strings.ContainsRune(subject, '\x00') {
		return time.Time{}, ports.ErrInvalidArgument
	}
	if service.TimerSubscriptions == nil || service.Clock == nil || service.Audits == nil || service.AuditRateLimiter == nil {
		return time.Time{}, errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return time.Time{}, errInvalidDependencies
	}
	if !service.AuditRateLimiter.Allow(actor, now) {
		return time.Time{}, platformapp.ErrRateLimited
	}
	if scope == TimerSubscriptionPath {
		if service.Authorizer == nil {
			return time.Time{}, errInvalidDependencies
		}
		allowed, err := service.Authorizer.Check(ctx, "path", subject, "track", actor)
		if err != nil {
			return time.Time{}, err
		}
		if !allowed {
			return time.Time{}, service.timerSubscriptionFailure(ctx, actor, subject, ports.ErrNotFound)
		}
	}
	return now, nil
}

func (service *Service) timerSubscriptionFailure(ctx context.Context, actor, subject string, err error) error {
	if !errors.Is(err, ports.ErrNotFound) {
		return err
	}
	event := shared.NewAuditEvent(ctx, service.Clock, actor, actor, audit.ResourceAccessDenied, "timer_subscription", subject, audit.Denied)
	if auditErr := service.Audits.AppendAuditEvent(ctx, event); auditErr != nil {
		return auditErr
	}
	return ports.ErrNotFound
}
