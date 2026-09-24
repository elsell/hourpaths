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

func (service *InvitationService) MarkNotificationRead(
	ctx context.Context,
	authorization, notificationID string,
) (NotificationMutationResult, error) {
	return service.mutateNotification(ctx, authorization, notificationID, audit.ResourceUpdated,
		func(command NotificationMutationCommand) (NotificationMutationResult, error) {
			return service.Invitations.MarkNotificationRead(ctx, command)
		})
}

func (service *InvitationService) DeleteNotification(
	ctx context.Context,
	authorization, notificationID string,
) (NotificationMutationResult, error) {
	return service.mutateNotification(ctx, authorization, notificationID, audit.ResourceDeleted,
		func(command NotificationMutationCommand) (NotificationMutationResult, error) {
			return service.Invitations.DeleteNotification(ctx, command)
		})
}

func (service *InvitationService) MarkAllNotificationsRead(
	ctx context.Context,
	authorization string,
) (NotificationMutationResult, error) {
	return service.mutateNotification(ctx, authorization, "history", audit.ResourceUpdated,
		func(command NotificationMutationCommand) (NotificationMutationResult, error) {
			return service.Invitations.MarkAllNotificationsRead(ctx, command)
		})
}

func (service *InvitationService) mutateNotification(
	ctx context.Context,
	authorization, targetID string,
	action audit.Action,
	mutate func(NotificationMutationCommand) (NotificationMutationResult, error),
) (NotificationMutationResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return NotificationMutationResult{}, err
	}
	if strings.TrimSpace(targetID) == "" || targetID != strings.TrimSpace(targetID) {
		return NotificationMutationResult{}, ports.ErrInvalidArgument
	}
	if service.Invitations == nil || service.Audits == nil ||
		service.AuditRateLimiter == nil || service.Clock == nil || mutate == nil {
		return NotificationMutationResult{}, errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC()
	if now.IsZero() {
		return NotificationMutationResult{}, errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return NotificationMutationResult{}, platformapp.ErrRateLimited
	}
	event := shared.NewAuditEvent(
		ctx, service.Clock, principal.UserID, principal.UserID,
		action, "notification", targetID, audit.Succeeded,
	)
	event.OccurredAt = now
	result, err := mutate(NotificationMutationCommand{
		RecipientUserID: principal.UserID,
		NotificationID:  targetID,
		ChangedAt:       now,
		Audit:           event,
	})
	if err == nil {
		if result.UnreadCount < 0 {
			return NotificationMutationResult{}, errInvalidInvitationDependencies
		}
		return result, nil
	}
	if !errors.Is(err, ports.ErrNotFound) {
		return NotificationMutationResult{}, err
	}
	denial := shared.NewAuditEvent(
		ctx, service.Clock, principal.UserID, principal.UserID,
		audit.ResourceAccessDenied, "notification", targetID, audit.Denied,
	)
	denial.OccurredAt = now
	if auditErr := service.Audits.AppendAuditEvent(ctx, denial); auditErr != nil {
		return NotificationMutationResult{}, auditErr
	}
	return NotificationMutationResult{}, ports.ErrNotFound
}
