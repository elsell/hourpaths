package pathstore

import (
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestPendingInvitationProjectionFailsClosedOnMalformedContext(t *testing.T) {
	now := time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC)
	username := "Path.Owner"
	valid := pendingInvitationRow{
		InvitationPersistence: invitationModel{
			ID: "invitation", PathID: "path", InviterUserID: "inviter",
			RecipientUserID: "recipient", OfferedRole: string(domain.RoleParticipant), CreatedAt: now,
		},
		PathName: "Reading", InviterID: "inviter",
		InviterUsername: &username, InviterName: "Path Owner",
		PathVisibility: "followers", RecipientVisibility: identity.ProfileVisibilityPrivate,
		HasRetainedActivity: true,
	}
	pending, err := pendingInvitationFromRow(valid, "recipient")
	if err != nil || pending.PendingInvitation.PathName != "Reading" ||
		pending.PendingInvitation.Inviter != (application.InvitationPublicIdentity{
			UserID: "inviter", Username: "Path.Owner", DisplayName: "Path Owner",
		}) || pending.WarningInput != (application.InvitationWarningInput{
		RecipientVisibility: identity.ProfileVisibilityPrivate,
		PathVisibility:      "followers",
		OfferedRole:         domain.RoleParticipant,
		HasRetainedActivity: true,
	}) {
		t.Fatalf("valid pending projection = %+v, %v", pending, err)
	}
	for _, mutate := range []func(*pendingInvitationRow){
		func(row *pendingInvitationRow) { row.PathName = "" },
		func(row *pendingInvitationRow) { row.InviterID = "other" },
		func(row *pendingInvitationRow) { row.InviterUsername = nil },
		func(row *pendingInvitationRow) { row.InviterName = " " },
	} {
		row := valid
		mutate(&row)
		if result, err := pendingInvitationFromRow(row, "recipient"); err == nil ||
			result != (application.PendingInvitationCandidate{}) {
			t.Fatalf("malformed pending row mapped to %+v, %v", result, err)
		}
	}
	if result, err := pendingInvitationFromRow(valid, "other-recipient"); err == nil ||
		result != (application.PendingInvitationCandidate{}) {
		t.Fatalf("cross-recipient pending row mapped to %+v, %v", result, err)
	}
}
