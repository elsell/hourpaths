package pathstore

import (
	"errors"
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
	if err := tx.Table("notification_models").Create(row).Error; err != nil {
		return err
	}
	if !notification.PushRequested {
		return nil
	}
	return createNotificationPushDelivery(tx, notification.ID, notification.RecipientUserID, notification.CreatedAt)
}

func createNotificationPushDelivery(tx *gorm.DB, notificationID, recipientUserID string, createdAt time.Time) error {
	if err := tx.Create(&notificationPushOutboxModel{
		NotificationID: notificationID, CreatedAt: createdAt,
	}).Error; err != nil {
		return err
	}
	if err := tx.Exec(`
INSERT INTO notification_push_delivery_models (
  notification_id, installation_id, recipient_user_id,
  provider, platform, locale, token_ciphertext, token_nonce, token_hash,
  available_at, created_at
)
SELECT ?, i.id, ?, i.provider, i.platform, i.locale,
       i.token_ciphertext, i.token_nonce, i.token_hash, ?, ?
FROM push_installation_models i
WHERE i.owner_user_id = ? AND i.deleted_at IS NULL
ON CONFLICT (notification_id, installation_id) DO NOTHING`,
		notificationID, recipientUserID, createdAt,
		createdAt, recipientUserID).Error; err != nil {
		return err
	}
	return tx.Exec(`
UPDATE notification_push_outbox_models
SET suppressed_at = ?, failure_code = 'no_active_installation'
WHERE notification_id = ?
  AND NOT EXISTS (
    SELECT 1 FROM notification_push_delivery_models
    WHERE notification_id = ?
	)`, createdAt, notificationID, notificationID).Error
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
