package gormstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type socialInteractionSettingModel struct {
	UserID                            string
	CommentsEnabled, ReactionsEnabled bool
	CreatedAt, UpdatedAt              time.Time
}

func (socialInteractionSettingModel) TableName() string { return "social_interaction_setting_models" }

type socialInteractionSettingReplayModel struct {
	ActorUserID, Operation, IdempotencyKey string
	RequestHash                            []byte
	CommentsEnabled, ReactionsEnabled      bool
	CreatedAt                              time.Time
}

func (socialInteractionSettingReplayModel) TableName() string {
	return "social_interaction_setting_replay_models"
}

func (repository *SocialFeedRepository) GetInteractionSettings(ctx context.Context, userID string) (socialapp.InteractionSettings, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(userID) {
		return socialapp.InteractionSettings{}, ports.ErrInvalidArgument
	}
	var row struct{ CommentsEnabled, ReactionsEnabled bool }
	err := repository.db.WithContext(ctx).Table("user_models AS owner").
		Select("COALESCE(settings.comments_enabled, true) AS comments_enabled, COALESCE(settings.reactions_enabled, true) AS reactions_enabled").
		Joins("LEFT JOIN social_interaction_setting_models settings ON settings.user_id = owner.id").
		Where("owner.id = ? AND owner.status = 'active'", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return socialapp.InteractionSettings{}, ports.ErrNotFound
	}
	if err != nil {
		return socialapp.InteractionSettings{}, classifyInteractionSettingsError(err)
	}
	return socialapp.InteractionSettings{CommentsEnabled: row.CommentsEnabled, ReactionsEnabled: row.ReactionsEnabled}, nil
}

func (repository *SocialFeedRepository) UpdateInteractionSettings(ctx context.Context, command socialapp.InteractionSettingsCommand) (socialapp.InteractionSettingsResult, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(command.ActorUserID) || command.OccurredAt.IsZero() || command.OccurredAt.Location() != time.UTC ||
		command.Idempotency.PrincipalID != command.ActorUserID || command.Idempotency.Operation != socialapp.UpdateInteractionSettingsOperation || len(command.Idempotency.Key) < 16 || len(command.Idempotency.RequestHash) != 32 ||
		!validMutationAudit(command.Audit, audit.ResourceUpdated, "social_interaction_settings", command.ActorUserID, command.ActorUserID) {
		return socialapp.InteractionSettingsResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.InteractionSettingsResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialInteractionOwner(tx, command.ActorUserID); err != nil {
			return err
		}
		var replay socialInteractionSettingReplayModel
		err := tx.Where("actor_user_id = ? AND operation = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = socialapp.InteractionSettingsResult{Settings: socialapp.InteractionSettings{CommentsEnabled: replay.CommentsEnabled, ReactionsEnabled: replay.ReactionsEnabled}, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var active int64
		if err := tx.Table("user_models").Where("id = ? AND status = 'active'", command.ActorUserID).Count(&active).Error; err != nil {
			return err
		}
		if active != 1 {
			return ports.ErrNotFound
		}
		row := socialInteractionSettingModel{UserID: command.ActorUserID, CommentsEnabled: command.Settings.CommentsEnabled, ReactionsEnabled: command.Settings.ReactionsEnabled, CreatedAt: command.OccurredAt, UpdatedAt: command.OccurredAt}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.Assignments(map[string]any{"comments_enabled": row.CommentsEnabled, "reactions_enabled": row.ReactionsEnabled, "updated_at": row.UpdatedAt})}).Create(&row).Error; err != nil {
			return err
		}
		if !row.CommentsEnabled {
			if err := tombstoneInteractionNotifications(tx, command.ActorUserID, []string{"practice_comment", "comment_heart"}, "comments", command.OccurredAt); err != nil {
				return err
			}
		}
		if !row.ReactionsEnabled {
			if err := tombstoneInteractionNotifications(tx, command.ActorUserID, []string{"practice_reaction"}, "reactions", command.OccurredAt); err != nil {
				return err
			}
		}
		if err := tx.Create(&socialInteractionSettingReplayModel{ActorUserID: command.ActorUserID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), CommentsEnabled: row.CommentsEnabled, ReactionsEnabled: row.ReactionsEnabled, CreatedAt: command.OccurredAt}).Error; err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		result.Settings = command.Settings
		return nil
	})
	return result, classifyInteractionSettingsError(err)
}

func lockSocialInteractionOwner(tx *gorm.DB, owner string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", socialLockKey("social-interaction-owner", owner)).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock
}

func lockSocialInteractionOwners(tx *gorm.DB, owners []string) error {
	ordered := append([]string(nil), owners...)
	sort.Strings(ordered)
	for index, owner := range ordered {
		if index > 0 && owner == ordered[index-1] {
			continue
		}
		if err := lockSocialInteractionOwner(tx, owner); err != nil {
			return err
		}
	}
	return nil
}

type interactionPushOutboxModel struct{ NotificationID string }

func (interactionPushOutboxModel) TableName() string { return "notification_push_outbox_models" }

type interactionPushDeliveryModel struct{ NotificationID, InstallationID string }

func (interactionPushDeliveryModel) TableName() string { return "notification_push_delivery_models" }

func tombstoneInteractionNotifications(tx *gorm.DB, owner string, kinds []string, reason string, at time.Time) error {
	var ids []string
	if err := tx.Table("notification_models AS notice").
		Joins("JOIN social_feed_event_models event ON event.id = notice.social_feed_event_id").
		Where("event.participant_user_id = ? AND notice.kind IN ? AND notice.deleted_at IS NULL", owner, kinds).
		Pluck("notice.id", &ids).Error; err != nil || len(ids) == 0 {
		return err
	}
	if err := tx.Table("notification_models").Where("id IN ?", ids).Updates(map[string]any{"deleted_at": gorm.Expr("GREATEST(created_at, ?)", at), "interaction_disabled_reason": reason}).Error; err != nil {
		return err
	}
	if err := tx.Where("notification_id IN ?", ids).Delete(&interactionPushDeliveryModel{}).Error; err != nil {
		return err
	}
	return tx.Where("notification_id IN ?", ids).Delete(&interactionPushOutboxModel{}).Error
}

func socialInteractionEnabled(tx *gorm.DB, userID, column string) (bool, error) {
	if column != "comments_enabled" && column != "reactions_enabled" {
		return false, ports.ErrInvalidArgument
	}
	var row struct{ Enabled bool }
	err := tx.Table("user_models AS owner").Select("COALESCE(settings."+column+", true) AS enabled").
		Joins("LEFT JOIN social_interaction_setting_models settings ON settings.user_id = owner.id").
		Where("owner.id = ? AND owner.status = 'active'", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, ports.ErrNotFound
	}
	return row.Enabled, err
}

func classifyInteractionSettingsError(err error) error {
	switch {
	case err == nil, errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrIdempotencyConflict):
		return err
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ports.ErrConflict
	default:
		return fmt.Errorf("social interaction settings persistence: %w: %v", ports.ErrUnavailable, err)
	}
}
