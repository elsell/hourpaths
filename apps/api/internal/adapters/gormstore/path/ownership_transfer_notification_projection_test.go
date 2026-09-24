package pathstore

import (
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
)

func TestOwnershipTransferNotificationProjectionEnforcesLifecycleActorsAndPresentation(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 20, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(72 * time.Hour)
	transitionAt := createdAt.Add(time.Hour)
	username := "New.Creator"
	base := invitationNotificationRow{
		NotificationPersistence: notificationModel{
			ID: "notification-1", RecipientUserID: "creator", ActorUserID: "recipient",
			PathID: "path-1", PathOwnershipTransferID: "transfer-1",
			Kind:              string(application.NotificationPathOwnershipTransferAccepted),
			PresentationClass: string(application.NotificationInformational),
			Channel:           invitationNotificationChannel, CreatedAt: transitionAt,
		},
		PathName: "Practice", ActorID: "recipient", ActorUsername: &username, ActorName: "New Creator",
		TransferPathID: "path-1", TransferInitiatorID: "creator", TransferRecipientID: "recipient",
		TransferCreatedAt: &createdAt, TransferExpiresAt: &expiresAt, TransferAcceptedAt: &transitionAt,
	}
	projection, err := invitationNotificationFromRow(base, "creator")
	if err != nil || projection.OwnershipTransferID != "transfer-1" || projection.InvitationID != "" || projection.OfferedRole != "" {
		t.Fatalf("accepted transfer projection = %+v, %v", projection, err)
	}

	for name, mutate := range map[string]func(*invitationNotificationRow){
		"subject collision":  func(row *invitationNotificationRow) { row.PathInvitationID = "invitation" },
		"wrong recipient":    func(row *invitationNotificationRow) { row.RecipientUserID = "recipient" },
		"wrong actor":        func(row *invitationNotificationRow) { row.ActorUserID = "creator"; row.ActorID = "creator" },
		"missing transition": func(row *invitationNotificationRow) { row.TransferAcceptedAt = nil },
		"wrong presentation": func(row *invitationNotificationRow) {
			row.PresentationClass = string(application.NotificationActionable)
		},
		"offered role": func(row *invitationNotificationRow) { row.OfferedRole = "participant" },
	} {
		t.Run(name, func(t *testing.T) {
			row := base
			mutate(&row)
			if got, err := invitationNotificationFromRow(row, row.RecipientUserID); err == nil || got != (application.InvitationNotificationProjection{}) {
				t.Fatalf("malformed transfer notification mapped to %+v, %v", got, err)
			}
		})
	}
}

func TestOwnershipTransferNotificationProjectionSupportsReceivedDeclinedAndCanceledSemantics(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 20, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(72 * time.Hour)
	transitionAt := createdAt.Add(time.Hour)
	username := "Actor"
	tests := []struct {
		kind             application.InvitationNotificationKind
		recipient, actor string
		presentation     application.NotificationPresentation
		transition       func(*invitationNotificationRow)
	}{
		{application.NotificationPathOwnershipTransferReceived, "recipient", "creator", application.NotificationActionable, func(*invitationNotificationRow) {}},
		{application.NotificationPathOwnershipTransferDeclined, "creator", "recipient", application.NotificationInformational, func(row *invitationNotificationRow) { row.TransferDeclinedAt = &transitionAt }},
		{application.NotificationPathOwnershipTransferCanceled, "recipient", "creator", application.NotificationInformational, func(row *invitationNotificationRow) { row.TransferCanceledAt = &transitionAt }},
	}
	for _, test := range tests {
		row := invitationNotificationRow{
			NotificationPersistence: notificationModel{ID: "notification", RecipientUserID: test.recipient, ActorUserID: test.actor, PathID: "path", PathOwnershipTransferID: "transfer", Kind: string(test.kind), PresentationClass: string(test.presentation), Channel: invitationNotificationChannel, CreatedAt: transitionAt},
			PathName:                "Practice", ActorID: test.actor, ActorUsername: &username, ActorName: "Actor",
			TransferPathID: "path", TransferInitiatorID: "creator", TransferRecipientID: "recipient", TransferCreatedAt: &createdAt, TransferExpiresAt: &expiresAt,
		}
		test.transition(&row)
		if got, err := invitationNotificationFromRow(row, test.recipient); err != nil || got.Kind != test.kind || got.OwnershipTransferID != "transfer" {
			t.Fatalf("%s projection = %+v, %v", test.kind, got, err)
		}
	}
}
