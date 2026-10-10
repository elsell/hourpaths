package pathstore

import (
	"errors"
	channelstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationchannel"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"gorm.io/gorm"
)

const invitationNotificationChannel = "path_access"

func createInvitationNotification(
	tx *gorm.DB,
	notification application.InvitationNotification,
	kind, presentationClass string,
) error {
	enabled, err := channelstore.Allowed(tx, notification.RecipientUserID, "path_access")
	if err != nil || !enabled {
		return err
	}
	if err := tx.Table("notification_models").Create(invitationNotificationPersistence(notification, kind, presentationClass)).Error; err != nil {
		return err
	}
	return createNotificationPushDelivery(tx, notification.ID, notification.RecipientUserID, notification.CreatedAt)
}

func invitationNotificationPersistence(notification application.InvitationNotification, kind, presentationClass string) map[string]any {
	return map[string]any{
		"id": notification.ID, "recipient_user_id": notification.RecipientUserID,
		"actor_user_id": notification.ActorUserID, "path_id": string(notification.PathID),
		"path_invitation_id": string(notification.InvitationID), "path_ownership_transfer_id": nil,
		"kind": kind, "presentation_class": presentationClass,
		"channel": invitationNotificationChannel, "offered_role": string(notification.OfferedRole),
		"created_at": notification.CreatedAt,
	}
}

func createOwnershipTransferNotification(
	tx *gorm.DB,
	notification application.OwnershipTransferNotification,
	transfer domain.OwnershipTransfer,
) error {
	if !validPersistedOwnershipTransferNotification(notification, transfer) {
		return applicationErrorInvalidNotification
	}
	row := map[string]any{
		"id": notification.ID, "recipient_user_id": notification.RecipientUserID,
		"actor_user_id": notification.ActorUserID, "path_id": string(notification.PathID),
		"path_invitation_id": nil, "path_ownership_transfer_id": string(notification.OwnershipTransferID),
		"kind": string(notification.Kind), "presentation_class": string(notification.Presentation),
		"channel": invitationNotificationChannel, "offered_role": nil,
		"created_at": notification.CreatedAt,
	}
	enabled, err := channelstore.Allowed(tx, notification.RecipientUserID, "path_access")
	if err != nil || !enabled {
		return err
	}
	if err := tx.Table("notification_models").Create(row).Error; err != nil {
		return err
	}
	if !notification.PushRequested {
		return nil
	}
	return createNotificationPushDelivery(tx, notification.ID, notification.RecipientUserID, notification.CreatedAt)
}

func createNotificationPushDelivery(tx *gorm.DB, notificationID, recipientUserID string, createdAt time.Time) error {
	return channelstore.QueuePush(tx, notificationID, recipientUserID, createdAt)
}

func createMemberAccessNotification(tx *gorm.DB, notification application.MemberAccessNotification, allowSelfStepDown bool) error {
	selfNotification := notification.RecipientUserID == notification.ActorUserID
	if tx == nil || notification.ID == "" || notification.RecipientUserID == "" || notification.ActorUserID == "" || (selfNotification && !allowSelfStepDown) || (!selfNotification && allowSelfStepDown) || notification.PathID == "" ||
		(notification.Kind != application.NotificationPathMemberRemoved && notification.Kind != application.NotificationPathMemberRoleChanged) || !notification.Role.ValidPathMemberRole() || notification.CreatedAt.IsZero() {
		return applicationErrorInvalidNotification
	}
	row := map[string]any{
		"id": notification.ID, "recipient_user_id": notification.RecipientUserID,
		"actor_user_id": notification.ActorUserID, "path_id": string(notification.PathID),
		"kind": string(notification.Kind), "presentation_class": string(application.NotificationInformational),
		"channel": invitationNotificationChannel, "offered_role": string(notification.Role),
		"created_at": notification.CreatedAt,
	}
	enabled, err := channelstore.Allowed(tx, notification.RecipientUserID, "path_access")
	if err != nil || !enabled {
		return err
	}
	if err := tx.Table("notification_models").Create(row).Error; err != nil {
		return err
	}
	return createNotificationPushDelivery(tx, notification.ID, notification.RecipientUserID, notification.CreatedAt)
}

var applicationErrorInvalidNotification = errors.New("ownership transfer notification is invalid")

func validPersistedOwnershipTransferNotification(notification application.OwnershipTransferNotification, transfer domain.OwnershipTransfer) bool {
	return strings.TrimSpace(notification.ID) == notification.ID && notification.ID != "" &&
		strings.TrimSpace(notification.RecipientUserID) == notification.RecipientUserID && notification.RecipientUserID != "" &&
		strings.TrimSpace(notification.ActorUserID) == notification.ActorUserID && notification.ActorUserID != "" &&
		notification.PathID == transfer.PathID && notification.OwnershipTransferID == transfer.ID &&
		!notification.CreatedAt.IsZero() && !notification.CreatedAt.Before(transfer.CreatedAt) &&
		((notification.Kind == application.NotificationPathOwnershipTransferReceived &&
			notification.Presentation == application.NotificationActionable && notification.PushRequested &&
			notification.ActorUserID == transfer.InitiatorUserID && notification.RecipientUserID == transfer.RecipientUserID) ||
			(notification.Kind == application.NotificationPathOwnershipTransferAccepted &&
				notification.Presentation == application.NotificationInformational && notification.PushRequested &&
				notification.ActorUserID == transfer.RecipientUserID && notification.RecipientUserID == transfer.InitiatorUserID) ||
			(notification.Kind == application.NotificationPathOwnershipTransferDeclined &&
				notification.Presentation == application.NotificationInformational && !notification.PushRequested &&
				notification.ActorUserID == transfer.RecipientUserID && notification.RecipientUserID == transfer.InitiatorUserID) ||
			(notification.Kind == application.NotificationPathOwnershipTransferCanceled &&
				notification.Presentation == application.NotificationInformational && !notification.PushRequested &&
				notification.ActorUserID == transfer.InitiatorUserID && notification.RecipientUserID == transfer.RecipientUserID))
}
