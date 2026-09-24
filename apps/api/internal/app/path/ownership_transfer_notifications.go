package path

import (
	"strings"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func newOwnershipTransferNotification(
	id string,
	transfer domain.OwnershipTransfer,
	kind InvitationNotificationKind,
	actor, recipient string,
	presentation NotificationPresentation,
	pushRequested bool,
	createdAt time.Time,
) OwnershipTransferNotification {
	return OwnershipTransferNotification{
		ID: id, RecipientUserID: recipient, ActorUserID: actor,
		PathID: transfer.PathID, OwnershipTransferID: transfer.ID,
		Kind: kind, Presentation: presentation, PushRequested: pushRequested,
		CreatedAt: createdAt,
	}
}

func validOwnershipTransferNotification(notification OwnershipTransferNotification, transfer domain.OwnershipTransfer) bool {
	validKind := (notification.Kind == NotificationPathOwnershipTransferReceived &&
		notification.Presentation == NotificationActionable && notification.PushRequested &&
		notification.ActorUserID == transfer.InitiatorUserID && notification.RecipientUserID == transfer.RecipientUserID) ||
		(notification.Kind == NotificationPathOwnershipTransferAccepted &&
			notification.Presentation == NotificationInformational && notification.PushRequested &&
			notification.ActorUserID == transfer.RecipientUserID && notification.RecipientUserID == transfer.InitiatorUserID) ||
		(notification.Kind == NotificationPathOwnershipTransferDeclined &&
			notification.Presentation == NotificationInformational && !notification.PushRequested &&
			notification.ActorUserID == transfer.RecipientUserID && notification.RecipientUserID == transfer.InitiatorUserID) ||
		(notification.Kind == NotificationPathOwnershipTransferCanceled &&
			notification.Presentation == NotificationInformational && !notification.PushRequested &&
			notification.ActorUserID == transfer.InitiatorUserID && notification.RecipientUserID == transfer.RecipientUserID)
	return validKind && strings.TrimSpace(notification.ID) == notification.ID && notification.ID != "" &&
		strings.TrimSpace(notification.ActorUserID) == notification.ActorUserID &&
		strings.TrimSpace(notification.RecipientUserID) == notification.RecipientUserID &&
		notification.PathID == transfer.PathID && notification.OwnershipTransferID == transfer.ID &&
		!notification.CreatedAt.IsZero() && !notification.CreatedAt.Before(transfer.CreatedAt)
}
