package gormstore

import (
	channelstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationchannel"
	"gorm.io/gorm"
	"time"
)

func (repository *SocialRelationshipRepository) createSocialNotification(tx *gorm.DB, kind, recipient, actor, subject, requestID string, createdAt time.Time, push bool) error {
	enabled, err := channelstore.Allowed(tx, recipient, "following")
	if err != nil || !enabled {
		return err
	}
	notificationID := repository.nextID()
	if notificationID == "" {
		return errInvalidSocialRelationshipRepository
	}
	presentation := "informational"
	if kind == "follow_request_received" {
		presentation = "actionable"
	}
	row := map[string]any{
		"id": notificationID, "recipient_user_id": recipient, "actor_user_id": actor,
		"path_id": nil, "path_invitation_id": nil, "path_ownership_transfer_id": nil,
		"follow_request_id": nil, "follow_subject_user_id": subject,
		"kind": kind, "presentation_class": presentation, "channel": "following",
		"offered_role": nil, "created_at": createdAt,
	}
	if requestID != "" {
		row["follow_request_id"] = requestID
	}
	if err := tx.Table("notification_models").Create(row).Error; err != nil {
		return err
	}
	if !push {
		return nil
	}
	if err := tx.Table("notification_push_outbox_models").Create(map[string]any{
		"notification_id": notificationID, "created_at": createdAt,
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
		notificationID, recipient, createdAt, createdAt, recipient).Error; err != nil { // hourpaths-direct-sql: allow transactional notification fanout
		return err
	}
	return tx.Exec(`
UPDATE notification_push_outbox_models
SET suppressed_at = ?, failure_code = 'no_active_installation'
WHERE notification_id = ?
  AND NOT EXISTS (
    SELECT 1 FROM notification_push_delivery_models
    WHERE notification_id = ?
  )`, createdAt, notificationID, notificationID).Error // hourpaths-direct-sql: allow transactional notification suppression
}
