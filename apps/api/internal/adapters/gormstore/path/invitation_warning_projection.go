package pathstore

import (
	"context"
	"strings"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

type pendingInvitationRow struct {
	InvitationPersistence `gorm:"embedded"`
	PathName              string
	PathVisibility        string
	RecipientVisibility   identity.ProfileVisibility
	HasRetainedActivity   bool
	InviterID             string
	InviterUsername       *string
	InviterName           string
}

func pendingInvitationFromRow(
	row pendingInvitationRow,
	recipientUserID string,
) (application.PendingInvitationCandidate, error) {
	invitation, err := invitationFromModel(row.InvitationPersistence)
	if err != nil || !invitation.Pending() || invitation.RecipientUserID != recipientUserID ||
		row.InviterUsername == nil || row.InviterID != invitation.InviterUserID ||
		strings.TrimSpace(row.PathName) == "" || strings.TrimSpace(row.PathName) != row.PathName ||
		strings.TrimSpace(*row.InviterUsername) == "" ||
		strings.TrimSpace(*row.InviterUsername) != *row.InviterUsername ||
		strings.TrimSpace(row.InviterName) == "" ||
		strings.TrimSpace(row.InviterName) != row.InviterName {
		return application.PendingInvitationCandidate{}, errInvalidPersistedInvitation
	}
	return application.PendingInvitationCandidate{
		PendingInvitation: application.PendingInvitation{
			Invitation: invitation,
			PathName:   row.PathName,
			Inviter: application.InvitationPublicIdentity{
				UserID: row.InviterID, Username: *row.InviterUsername, DisplayName: row.InviterName,
			},
		},
		WarningInput: application.InvitationWarningInput{
			RecipientVisibility: row.RecipientVisibility,
			PathVisibility:      row.PathVisibility,
			OfferedRole:         invitation.OfferedRole,
			HasRetainedActivity: row.HasRetainedActivity,
		},
	}, nil
}

func retainedActivityExists(
	ctx context.Context,
	db *gorm.DB,
	recipientUserID string,
	pathID domain.ID,
) (bool, error) {
	if db == nil || strings.TrimSpace(recipientUserID) == "" || pathID == "" {
		return false, ports.ErrInvalidArgument
	}
	var count int64
	err := db.WithContext(ctx).
		Table("recorded_activity_models").
		Where("participant_id = ? AND path_id = ?", recipientUserID, pathID).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
