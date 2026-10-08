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
	return createSocialInteractionPush(tx, notificationID, recipient, createdAt)
}
