package path

import (
	"context"
	"errors"
	"strings"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (service *InvitationService) GetNotification(
	ctx context.Context,
	authorization, notificationID string,
) (InvitationNotificationProjection, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return InvitationNotificationProjection{}, err
	}
	if strings.TrimSpace(notificationID) == "" ||
		notificationID != strings.TrimSpace(notificationID) {
		return InvitationNotificationProjection{}, ports.ErrInvalidArgument
	}
	if service.Invitations == nil || service.Audits == nil ||
		service.AuditRateLimiter == nil || service.Clock == nil {
		return InvitationNotificationProjection{}, errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC()
	if now.IsZero() {
		return InvitationNotificationProjection{}, errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return InvitationNotificationProjection{}, platformapp.ErrRateLimited
	}
	item, err := service.Invitations.GetNotification(
		ctx, principal.UserID, notificationID,
	)
	if err == nil && item.CreatedAt.After(now) {
		err = ports.ErrNotFound
	}
	if err == nil {
		if !validNotificationPage(
			NotificationPage{Items: []InvitationNotificationProjection{item}},
			1,
			now,
		) {
			return InvitationNotificationProjection{}, errInvalidInvitationDependencies
		}
		event := shared.NewAuditEvent(
			ctx, service.Clock, principal.UserID, principal.UserID,
			audit.ResourceViewed, "notification", notificationID, audit.Succeeded,
		)
		event.OccurredAt = now
		if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
			return InvitationNotificationProjection{}, err
		}
		return item, nil
	}
	if !errors.Is(err, ports.ErrNotFound) {
		return InvitationNotificationProjection{}, err
	}
	event := shared.NewAuditEvent(
		ctx, service.Clock, principal.UserID, principal.UserID,
		audit.ResourceAccessDenied, "notification", notificationID, audit.Denied,
	)
	event.OccurredAt = now
	if auditErr := service.Audits.AppendAuditEvent(ctx, event); auditErr != nil {
		return InvitationNotificationProjection{}, auditErr
	}
	return InvitationNotificationProjection{}, ports.ErrNotFound
}
