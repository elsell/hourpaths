package pathstore

import (
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func samePendingInvitation(pending, accepted domain.Invitation) bool {
	return pending.ID == accepted.ID && pending.PathID == accepted.PathID &&
		pending.InviterUserID == accepted.InviterUserID &&
		pending.RecipientUserID == accepted.RecipientUserID &&
		pending.OfferedRole == accepted.OfferedRole &&
		pending.CreatedAt.Equal(accepted.CreatedAt) && !accepted.AcceptedAt.IsZero()
}

func acceptedInvitationReplay(tx *gorm.DB, recipientUserID string, invitationID domain.InvitationID) (application.AcceptInvitationResult, error) {
	var row invitationModel
	if err := tx.Where(
		"id = ? AND recipient_user_id = ? AND accepted_at IS NOT NULL AND authorization_change_id IS NOT NULL",
		invitationID, recipientUserID,
	).First(&row).Error; err != nil {
		return application.AcceptInvitationResult{}, err
	}
	invitation, err := invitationFromModel(row)
	if err != nil || row.AuthorizationChangeID == nil {
		return application.AcceptInvitationResult{}, errInvalidPersistedInvitation
	}
	var outbox authorizationOutboxModel
	if err := tx.Where("id = ?", *row.AuthorizationChangeID).First(&outbox).Error; err != nil {
		return application.AcceptInvitationResult{}, err
	}
	return application.AcceptInvitationResult{
		Invitation: invitation,
		AuthorizationChange: ports.AuthorizationChange{
			ID: outbox.ID, ResourceType: outbox.ResourceType, ResourceID: outbox.ResourceID,
			Relation: outbox.Relation, SubjectType: outbox.SubjectType, SubjectID: outbox.SubjectID,
			OwnerUserID: outbox.OwnerUserID, ActorUserID: outbox.ActorUserID,
			Operation: outbox.Operation, LockedBy: outbox.LockedBy,
		},
		Replayed: true,
	}, nil
}
