package pathstore

import (
	"testing"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestInvitationPersistenceMapsOnlyValidCancellation(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	canceledAt := now.Add(time.Minute)
	row := invitationModel{ID: "invitation", PathID: "path", InviterUserID: "inviter", RecipientUserID: "recipient", OfferedRole: string(domain.RoleParticipant), CreatedAt: now, CanceledAt: &canceledAt}
	invitation, err := invitationFromModel(row)
	if err != nil || invitation.CanceledAt != canceledAt || invitation.Pending() {
		t.Fatalf("mapped=%+v error=%v", invitation, err)
	}
	before := now.Add(-time.Second)
	row.CanceledAt = &before
	if invitation, err := invitationFromModel(row); err == nil || invitation != (domain.Invitation{}) {
		t.Fatalf("invalid cancellation mapped=%+v error=%v", invitation, err)
	}
}
