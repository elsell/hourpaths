package path

import (
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestPendingInvitationPageFailsClosedOnMalformedJoinedContext(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	valid := PendingInvitation{
		Invitation: domain.Invitation{
			ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
			RecipientUserID: "recipient", OfferedRole: domain.RoleSupporter, CreatedAt: now,
		},
		PathName: "Reading",
		Inviter: InvitationPublicIdentity{
			UserID: "creator", Username: "Path.Owner", DisplayName: "Path Owner",
		},
	}
	candidate := PendingInvitationCandidate{
		PendingInvitation: valid,
		WarningInput: InvitationWarningInput{
			RecipientVisibility: identity.ProfileVisibilityPublic,
			PathVisibility:      "private",
			OfferedRole:         domain.RoleSupporter,
		},
	}
	if !validPendingInvitationPage(InvitationPage{Items: []PendingInvitationCandidate{candidate}}, "recipient", 25) {
		t.Fatal("valid pending projection was rejected")
	}
	participant := candidate
	participant.PendingInvitation.Invitation.OfferedRole = domain.RoleParticipant
	participant.WarningInput = InvitationWarningInput{
		RecipientVisibility: identity.ProfileVisibilityPrivate,
		PathVisibility:      "followers",
		OfferedRole:         domain.RoleParticipant,
		HasRetainedActivity: true,
	}
	if !validPendingInvitationPage(InvitationPage{Items: []PendingInvitationCandidate{participant}}, "recipient", 25) {
		t.Fatal("valid minimal warning context was rejected")
	}
	for _, mutate := range []func(*PendingInvitationCandidate){
		func(value *PendingInvitationCandidate) { value.PendingInvitation.PathName = "" },
		func(value *PendingInvitationCandidate) { value.PendingInvitation.Inviter.UserID = "other" },
		func(value *PendingInvitationCandidate) { value.PendingInvitation.Inviter.Username = " " },
		func(value *PendingInvitationCandidate) { value.PendingInvitation.Inviter.DisplayName = "" },
		func(value *PendingInvitationCandidate) {
			value.PendingInvitation.Warning = &InvitationWarningContext{PathVisibility: "private"}
		},
	} {
		item := candidate
		mutate(&item)
		if validPendingInvitationPage(InvitationPage{Items: []PendingInvitationCandidate{item}}, "recipient", 25) {
			t.Fatalf("malformed pending projection accepted: %+v", item)
		}
	}
}
