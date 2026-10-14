package app

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/liveactivity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

func (a App) UpsertLiveActivity(ctx context.Context, authorization string, registration liveactivity.Registration) error {
	principal, err := a.pushPrincipal(ctx, authorization)
	if err != nil {
		return err
	}
	if a.LiveActivities == nil || a.Clock == nil || a.AuditRateLimiter == nil || a.Audits == nil {
		return ports.ErrInvalidArgument
	}
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	registration.OwnerUserID = principal.UserID
	if !registration.Valid(now) {
		return ports.ErrInvalidArgument
	}
	if !a.AuditRateLimiter.Allow(principal.UserID, now) {
		return ErrRateLimited
	}
	event := a.auditEvent(ctx, principal.UserID, principal.UserID, audit.ResourceUpdated, "live_activity_registration", registration.ID, audit.Succeeded)
	event.OccurredAt = now
	err = a.LiveActivities.UpsertLiveActivity(ctx, registration, event)
	if errors.Is(err, ports.ErrNotFound) {
		return a.auditLiveActivityDenial(ctx, principal.UserID, registration.ID, now)
	}
	return err
}

func (a App) RevokeLiveActivity(ctx context.Context, authorization, id, lease string) error {
	principal, err := a.pushPrincipal(ctx, authorization)
	if err != nil {
		return err
	}
	if a.LiveActivities == nil || a.Clock == nil || a.AuditRateLimiter == nil || a.Audits == nil || !liveactivity.ValidID(id) || !liveactivity.ValidID(lease) {
		return ports.ErrInvalidArgument
	}
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	if !a.AuditRateLimiter.Allow(principal.UserID, now) {
		return ErrRateLimited
	}
	event := a.auditEvent(ctx, principal.UserID, principal.UserID, audit.ResourceDeleted, "live_activity_registration", id, audit.Succeeded)
	event.OccurredAt = now
	err = a.LiveActivities.RevokeLiveActivity(ctx, principal.UserID, id, lease, event)
	if errors.Is(err, ports.ErrNotFound) {
		return a.auditLiveActivityDenial(ctx, principal.UserID, id, now)
	}
	return err
}

func (a App) auditLiveActivityDenial(ctx context.Context, owner, id string, now time.Time) error {
	event := a.auditEvent(ctx, owner, owner, audit.ResourceAccessDenied, "live_activity_registration", id, audit.Denied)
	event.OccurredAt = now
	if err := a.Audits.AppendAuditEvent(ctx, event); err != nil {
		return err
	}
	return ports.ErrNotFound
}
