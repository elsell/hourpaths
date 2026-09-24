package path

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (service *InvitationService) Reject(
	ctx context.Context,
	authorization, idempotencyKey string,
	invitationID domain.InvitationID,
) (RejectInvitationResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return RejectInvitationResult{}, err
	}
	if invitationID == "" || !validIdempotencyKey(idempotencyKey) {
		return RejectInvitationResult{}, ports.ErrInvalidArgument
	}
	if service.Invitations == nil || service.Audits == nil ||
		service.AuditRateLimiter == nil || service.Clock == nil {
		return RejectInvitationResult{}, errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return RejectInvitationResult{}, errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return RejectInvitationResult{}, platformapp.ErrRateLimited
	}
	digest := canonicalRejectInvitationRequestHash(invitationID)
	idempotency := ports.Idempotency{
		PrincipalID: principal.UserID, Operation: RejectInvitationOperation,
		Key: idempotencyKey, RequestHash: digest[:],
	}
	decision, err := service.Invitations.RejectionDecision(ctx, principal.UserID, invitationID, idempotency)
	if err != nil {
		if errors.Is(err, domain.ErrInvitationUnavailable) {
			return RejectInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
		}
		return RejectInvitationResult{}, err
	}
	if decision.Replay != nil {
		result := *decision.Replay
		if !validRejectedInvitationResult(result, principal.UserID, invitationID) || !result.Replayed {
			return RejectInvitationResult{}, errInvalidInvitationDependencies
		}
		return result, nil
	}
	if decision.OwnerUserID == "" || decision.Invitation.ID != invitationID ||
		decision.Invitation.RecipientUserID != principal.UserID || !decision.Invitation.Pending() {
		return RejectInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
	}
	rejected, err := decision.Invitation.Reject(principal.UserID, now)
	if err != nil {
		return RejectInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
	}
	event := shared.NewAuditEvent(
		ctx, service.Clock, decision.OwnerUserID, principal.UserID,
		audit.PathInvitationRejected, "path_invitation", string(invitationID), audit.Succeeded,
	)
	event.OccurredAt = now
	result, err := service.Invitations.Reject(ctx, RejectInvitationCommand{
		Invitation: rejected, Idempotency: idempotency, Audit: event,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvitationUnavailable) {
			return RejectInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
		}
		return RejectInvitationResult{}, err
	}
	if !validRejectedInvitationResult(result, principal.UserID, invitationID) {
		return RejectInvitationResult{}, errInvalidInvitationDependencies
	}
	return result, nil
}

func canonicalRejectInvitationRequestHash(invitationID domain.InvitationID) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(invitationID))
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func validRejectedInvitationResult(result RejectInvitationResult, recipient string, invitationID domain.InvitationID) bool {
	invitation := result.Invitation
	return invitation.ID == invitationID && invitation.RecipientUserID == recipient &&
		invitation.OfferedRole.Valid() && invitation.AcceptedAt.IsZero() &&
		!invitation.RejectedAt.IsZero() && !invitation.RejectedAt.Before(invitation.CreatedAt) &&
		result.UnreadCount >= 0
}
