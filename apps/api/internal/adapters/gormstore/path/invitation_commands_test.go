package pathstore

import (
	"bytes"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func sendInvitationCommand(invitation domain.Invitation, notificationID, key string, hashByte byte, auditID, owner string) application.SendInvitationCommand {
	username := "Reader.One"
	if invitation.RecipientUserID == "path-invite-stranger" {
		username = "Stranger"
	}
	return application.SendInvitationCommand{
		Invitation: invitation, ExpectedRecipientUserID: invitation.RecipientUserID,
		ExpectedRecipientUsername: username,
		Notification: application.InvitationNotification{
			ID: notificationID, RecipientUserID: invitation.RecipientUserID, ActorUserID: invitation.InviterUserID,
			PathID: invitation.PathID, InvitationID: invitation.ID, OfferedRole: invitation.OfferedRole,
			Actionable: true, CreatedAt: invitation.CreatedAt,
		},
		Idempotency: ports.Idempotency{
			PrincipalID: invitation.InviterUserID, Operation: application.SendInvitationOperation,
			Key: key, RequestHash: bytes.Repeat([]byte{hashByte}, 32),
		},
		Audit: audit.Event{
			ID: auditID, OwnerUserID: owner, ActorUserID: invitation.InviterUserID,
			Action: audit.PathInvitationCreated, TargetType: "path_invitation", TargetID: string(invitation.ID),
			Outcome: audit.Succeeded, CorrelationID: "path-invitation", OccurredAt: invitation.CreatedAt,
		},
	}
}

func acceptInvitationCommand(invitation domain.Invitation, change ports.AuthorizationChange, idempotency ports.Idempotency, notificationID, auditID, owner string) application.AcceptInvitationCommand {
	return application.AcceptInvitationCommand{
		Invitation: invitation,
		Acknowledgement: application.InvitationWarningAcknowledgement{
			PathVisibility: "followers",
		},
		AuthorizationChange: change,
		Notification: application.InvitationNotification{
			ID: notificationID, RecipientUserID: invitation.InviterUserID, ActorUserID: invitation.RecipientUserID,
			PathID: invitation.PathID, InvitationID: invitation.ID, OfferedRole: invitation.OfferedRole,
			Actionable: false, CreatedAt: invitation.AcceptedAt,
		},
		Idempotency: idempotency,
		Audit: audit.Event{
			ID: auditID, OwnerUserID: owner, ActorUserID: invitation.RecipientUserID,
			Action: audit.PathInvitationAccepted, TargetType: "path_invitation", TargetID: string(invitation.ID),
			Outcome: audit.Succeeded, CorrelationID: "path-invitation", OccurredAt: invitation.AcceptedAt,
		},
	}
}

func rejectInvitationCommand(invitation domain.Invitation, idempotency ports.Idempotency, auditID, owner string) application.RejectInvitationCommand {
	return application.RejectInvitationCommand{
		Invitation:  invitation,
		Idempotency: idempotency,
		Audit: audit.Event{
			ID: auditID, OwnerUserID: owner, ActorUserID: invitation.RecipientUserID,
			Action: audit.PathInvitationRejected, TargetType: "path_invitation", TargetID: string(invitation.ID),
			Outcome: audit.Succeeded, CorrelationID: "path-invitation", OccurredAt: invitation.RejectedAt,
		},
	}
}
