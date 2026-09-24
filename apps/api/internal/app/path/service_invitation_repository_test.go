package path

import (
	"context"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (controlledRepository) ActiveByExactUsername(context.Context, string) (InvitationRecipient, error) {
	return InvitationRecipient{}, ports.ErrNotFound
}

func (controlledRepository) Send(context.Context, SendInvitationCommand) (SendInvitationResult, error) {
	return SendInvitationResult{}, ports.ErrNotFound
}

func (controlledRepository) ListPending(context.Context, string, InvitationPageRequest) (InvitationPage, error) {
	return InvitationPage{}, ports.ErrNotFound
}

func (controlledRepository) ListNotifications(context.Context, string, NotificationPageRequest) (NotificationPage, error) {
	return NotificationPage{}, ports.ErrNotFound
}

func (controlledRepository) GetNotification(context.Context, string, string) (InvitationNotificationProjection, error) {
	return InvitationNotificationProjection{}, ports.ErrNotFound
}

func (controlledRepository) MarkNotificationRead(context.Context, NotificationMutationCommand) (NotificationMutationResult, error) {
	return NotificationMutationResult{}, ports.ErrNotFound
}

func (controlledRepository) DeleteNotification(context.Context, NotificationMutationCommand) (NotificationMutationResult, error) {
	return NotificationMutationResult{}, ports.ErrNotFound
}

func (controlledRepository) MarkAllNotificationsRead(context.Context, NotificationMutationCommand) (NotificationMutationResult, error) {
	return NotificationMutationResult{}, ports.ErrNotFound
}

func (controlledRepository) AcceptanceDecision(context.Context, string, domain.InvitationID, ports.Idempotency) (InvitationDecision, error) {
	return InvitationDecision{}, ports.ErrNotFound
}

func (controlledRepository) Accept(context.Context, AcceptInvitationCommand) (AcceptInvitationResult, error) {
	return AcceptInvitationResult{}, ports.ErrNotFound
}

func (controlledRepository) RejectionDecision(context.Context, string, domain.InvitationID, ports.Idempotency) (RejectionDecision, error) {
	return RejectionDecision{}, ports.ErrNotFound
}

func (controlledRepository) Reject(context.Context, RejectInvitationCommand) (RejectInvitationResult, error) {
	return RejectInvitationResult{}, ports.ErrNotFound
}

func (controlledRepository) AuthorizationChangeState(context.Context, string) (AuthorizationChangeState, error) {
	return "", ports.ErrNotFound
}
