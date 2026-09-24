package pathstore

import (
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
)

func TestInvitationNotificationPersistenceUsesExactlyOneDatabaseSubject(t *testing.T) {
	notification := application.InvitationNotification{
		ID: "notification", RecipientUserID: "recipient", ActorUserID: "actor",
		PathID: "path", InvitationID: "invitation", OfferedRole: "participant",
		Actionable: true, CreatedAt: time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
	row := invitationNotificationPersistence(notification, "path_invitation_received", "actionable")
	if row["path_invitation_id"] != "invitation" {
		t.Fatalf("path_invitation_id = %#v", row["path_invitation_id"])
	}
	if value, exists := row["path_ownership_transfer_id"]; !exists || value != nil {
		t.Fatalf("path_ownership_transfer_id = %#v, exists = %v; want explicit SQL NULL", value, exists)
	}
}
