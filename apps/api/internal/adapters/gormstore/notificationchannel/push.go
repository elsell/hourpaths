package notificationchannel

import (
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// QueuePush snapshots only this recipient's active installations in the
// producer transaction. The delivery worker independently revalidates ownership.
func QueuePush(tx *gorm.DB, notificationID, recipient string, at time.Time) error {
	return QueuePushAvailable(tx, notificationID, recipient, at, at)
}

// QueuePushAvailable retains the occurrence instant across delayed social delivery.
func QueuePushAvailable(tx *gorm.DB, notificationID, recipient string, at, availableAt time.Time) error {
	if tx == nil || notificationID == "" || recipient == "" || at.IsZero() || availableAt.Before(at) {
		return ports.ErrInvalidArgument
	}
	if err := tx.Table("notification_push_outbox_models").Create(map[string]any{"notification_id": notificationID, "created_at": at}).Error; err != nil {
		return err
	}
	quiet, err := QuietAt(tx, recipient, at)
	if err != nil {
		return err
	}
	if !quiet && !availableAt.Equal(at) {
		quiet, err = QuietAt(tx, recipient, availableAt)
		if err != nil {
			return err
		}
	}
	if quiet {
		return tx.Table("notification_push_outbox_models").Where("notification_id = ?", notificationID).Updates(map[string]any{"suppressed_at": at, "failure_code": "quiet_period"}).Error
	}
	var installations []struct {
		ID, Provider, Platform, Locale         string
		TokenCiphertext, TokenNonce, TokenHash []byte
	}
	if err := tx.Table("push_installation_models").Select("id, provider, platform, locale, token_ciphertext, token_nonce, token_hash").Where("owner_user_id = ? AND deleted_at IS NULL", recipient).Find(&installations).Error; err != nil {
		return err
	}
	if len(installations) == 0 {
		return tx.Table("notification_push_outbox_models").Where("notification_id = ?", notificationID).Updates(map[string]any{"suppressed_at": at, "failure_code": "no_active_installation"}).Error
	}
	rows := make([]map[string]any, 0, len(installations))
	for _, installation := range installations {
		rows = append(rows, map[string]any{"notification_id": notificationID, "installation_id": installation.ID, "recipient_user_id": recipient, "provider": installation.Provider, "platform": installation.Platform, "locale": installation.Locale,
			"token_ciphertext": installation.TokenCiphertext, "token_nonce": installation.TokenNonce, "token_hash": installation.TokenHash, "available_at": availableAt, "created_at": at})
	}
	return tx.Table("notification_push_delivery_models").Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}
