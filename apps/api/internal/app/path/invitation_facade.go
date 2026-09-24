package path

import (
	"context"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func (s *Service) invitationService() *InvitationService {
	return NewInvitationService(InvitationDependencies{
		Auth:                    s.Auth,
		Paths:                   s.Repository,
		Directory:               s.Repository,
		Authorizer:              s.Authorizer,
		Invitations:             s.Repository,
		InvitationManagement:    s.InvitationManagement,
		WarningPolicy:           VisibilityInvitationWarningPolicy{},
		Audits:                  s.Audits,
		AuditRateLimiter:        s.AuditRateLimiter,
		Clock:                   s.Clock,
		NewID:                   s.NewID,
		AuthorizationOutbox:     s.AuthorizationOutbox,
		AuthorizationStatus:     s.Repository,
		AuthorizationSerializer: s.AuthorizationSerializer,
		AuthorizationWorker:     s.AuthorizationWorker,
		AuthorizationLease:      s.AuthorizationLease,
		CursorSigningKey:        s.CursorSigningKey,
	})
}

func (s *Service) Send(
	ctx context.Context,
	authorization, idempotencyKey string,
	pathID domain.ID,
	exactUsername string,
	expectedRecipientUserID string,
	offeredRole domain.MembershipRole,
) (SendInvitationResult, error) {
	return s.invitationService().Send(
		ctx, authorization, idempotencyKey, pathID, exactUsername, expectedRecipientUserID, offeredRole,
	)
}

func (s *Service) ReviewRecipient(
	ctx context.Context,
	authorization string,
	pathID domain.ID,
	exactUsername string,
) (InvitationRecipientReview, error) {
	return s.invitationService().ReviewRecipient(ctx, authorization, pathID, exactUsername)
}

func (s *Service) ListPending(
	ctx context.Context,
	authorization, cursor string,
	limit int,
) ([]PendingInvitation, string, error) {
	return s.invitationService().ListPending(ctx, authorization, cursor, limit)
}

func (s *Service) ListManagedPending(ctx context.Context, authorization string, pathID domain.ID, cursor string, limit int) ([]ManagedInvitation, string, error) {
	return s.invitationService().ListManagedPending(ctx, authorization, pathID, cursor, limit)
}

func (s *Service) ListNotifications(
	ctx context.Context,
	authorization, cursor string,
	limit int,
) ([]InvitationNotificationProjection, string, int64, error) {
	return s.invitationService().ListNotifications(ctx, authorization, cursor, limit)
}

func (s *Service) GetNotification(
	ctx context.Context,
	authorization, notificationID string,
) (InvitationNotificationProjection, error) {
	return s.invitationService().GetNotification(ctx, authorization, notificationID)
}

func (s *Service) MarkNotificationRead(
	ctx context.Context,
	authorization, notificationID string,
) (NotificationMutationResult, error) {
	return s.invitationService().MarkNotificationRead(ctx, authorization, notificationID)
}

func (s *Service) DeleteNotification(
	ctx context.Context,
	authorization, notificationID string,
) (NotificationMutationResult, error) {
	return s.invitationService().DeleteNotification(ctx, authorization, notificationID)
}

func (s *Service) MarkAllNotificationsRead(
	ctx context.Context,
	authorization string,
) (NotificationMutationResult, error) {
	return s.invitationService().MarkAllNotificationsRead(ctx, authorization)
}

func (s *Service) Accept(
	ctx context.Context,
	authorization, idempotencyKey string,
	invitationID domain.InvitationID,
) (AcceptInvitationResult, error) {
	return s.invitationService().Accept(ctx, authorization, idempotencyKey, invitationID)
}

func (s *Service) Reject(
	ctx context.Context,
	authorization, idempotencyKey string,
	invitationID domain.InvitationID,
) (RejectInvitationResult, error) {
	return s.invitationService().Reject(ctx, authorization, idempotencyKey, invitationID)
}

func (s *Service) CancelInvitation(ctx context.Context, authorization, idempotencyKey string, pathID domain.ID, invitationID domain.InvitationID) (CancelInvitationResult, error) {
	return s.invitationService().Cancel(ctx, authorization, idempotencyKey, pathID, invitationID)
}

func (s *InvitationService) CancelInvitation(ctx context.Context, authorization, idempotencyKey string, pathID domain.ID, invitationID domain.InvitationID) (CancelInvitationResult, error) {
	return s.Cancel(ctx, authorization, idempotencyKey, pathID, invitationID)
}

func (s *Service) AcceptConfirmed(
	ctx context.Context,
	authorization, idempotencyKey string,
	invitationID domain.InvitationID,
	acknowledgement InvitationWarningAcknowledgement,
) (AcceptInvitationResult, error) {
	return s.invitationService().AcceptConfirmed(ctx, authorization, idempotencyKey, invitationID, acknowledgement)
}
