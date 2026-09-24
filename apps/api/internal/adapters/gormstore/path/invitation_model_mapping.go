package pathstore

import domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"

func invitationToModel(invitation domain.Invitation) *invitationModel {
	row := &invitationModel{ID: string(invitation.ID), PathID: string(invitation.PathID), InviterUserID: invitation.InviterUserID, RecipientUserID: invitation.RecipientUserID, OfferedRole: string(invitation.OfferedRole), CreatedAt: invitation.CreatedAt}
	if !invitation.AcceptedAt.IsZero() {
		acceptedAt := invitation.AcceptedAt.UTC()
		row.AcceptedAt = &acceptedAt
	}
	if !invitation.RejectedAt.IsZero() {
		rejectedAt := invitation.RejectedAt.UTC()
		row.RejectedAt = &rejectedAt
	}
	if !invitation.CanceledAt.IsZero() {
		canceledAt := invitation.CanceledAt.UTC()
		row.CanceledAt = &canceledAt
	}
	return row
}
