package gormstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/notification"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NudgeNotificationChannelRepository struct{ db *gorm.DB }

func NewNudgeNotificationChannelRepository(db *gorm.DB) *NudgeNotificationChannelRepository {
	return &NudgeNotificationChannelRepository{db: db}
}

type nudgeNotificationChannelPreferenceModel struct {
	UserID               string `gorm:"primaryKey"`
	Channel              string `gorm:"primaryKey"`
	Enabled              bool
	Revision             int64
	CreatedAt, UpdatedAt time.Time
}

func (nudgeNotificationChannelPreferenceModel) TableName() string {
	return "notification_channel_preference_models"
}

type nudgeNotificationChannelReplayModel struct {
	ActorUserID, Operation, IdempotencyKey, Channel string
	RequestHash                                     []byte
	ResultEnabled                                   bool
	ResultRevision                                  int64
	ResultUpdatedAt, CreatedAt                      time.Time
}

func (nudgeNotificationChannelReplayModel) TableName() string {
	return "notification_channel_preference_replay_models"
}

func (repository *NudgeNotificationChannelRepository) GetNotificationChannel(ctx context.Context, actor string, channel notification.Channel) (socialapp.NudgeNotificationChannelPreference, bool, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(actor) || !channel.Valid() {
		return socialapp.NudgeNotificationChannelPreference{}, false, ports.ErrInvalidArgument
	}
	var row struct {
		Enabled  *bool
		Revision *int64
	}
	err := repository.db.WithContext(ctx).Table("user_models AS actor").
		Select("preference.enabled, preference.revision").
		Joins("LEFT JOIN notification_channel_preference_models preference ON preference.user_id = actor.id AND preference.channel = ?", string(channel)).
		Where("actor.id = ? AND actor.status = 'active'", actor).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return socialapp.NudgeNotificationChannelPreference{}, false, ports.ErrNotFound
	}
	if err != nil {
		return socialapp.NudgeNotificationChannelPreference{}, false, classifyNudgeNotificationChannelError(err)
	}
	if row.Enabled == nil && row.Revision == nil {
		return socialapp.NudgeNotificationChannelPreference{}, false, nil
	}
	if row.Enabled == nil || row.Revision == nil || *row.Revision < 1 {
		return socialapp.NudgeNotificationChannelPreference{}, false, classifyNudgeNotificationChannelError(errors.New("persisted nudge notification preference is invalid"))
	}
	return socialapp.NudgeNotificationChannelPreference{Enabled: *row.Enabled, Revision: *row.Revision}, true, nil
}

func (repository *NudgeNotificationChannelRepository) UpdateNotificationChannel(ctx context.Context, command socialapp.NotificationChannelCommand) (socialapp.NudgeNotificationChannelMutationResult, error) {
	if !validNotificationChannelCommand(repository, command) {
		return socialapp.NudgeNotificationChannelMutationResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.NudgeNotificationChannelMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Match social mutation/push lock order before taking the channel lock.
		if err := lockSocialInteractionOwner(tx, command.ActorUserID); err != nil {
			return err
		}
		// Serialize with every notification producer before reading preferences.
		// Producers retain KEY SHARE on this user until their notification commits.
		var actor struct{ ID string }
		if err := tx.Table("user_models").Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = 'active'", command.ActorUserID).Take(&actor).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if err := lockSocialInteractionOwner(tx, socialLockKey("notification-channel", command.ActorUserID)); err != nil {
			return err
		}
		var replay nudgeNotificationChannelReplayModel
		err := tx.Where("actor_user_id = ? AND operation = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			if replay.Channel != string(command.Channel) || replay.ResultEnabled != command.Enabled || replay.ResultRevision != command.ExpectedRevision+1 {
				return errors.New("persisted nudge notification channel replay is invalid")
			}
			result = socialapp.NudgeNotificationChannelMutationResult{Preference: socialapp.NudgeNotificationChannelPreference{Enabled: replay.ResultEnabled, Revision: replay.ResultRevision}, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		current, found, err := NewNudgeNotificationChannelRepository(tx).GetNotificationChannel(ctx, command.ActorUserID, command.Channel)
		if err != nil {
			return err
		}
		if current.Revision != command.ExpectedRevision {
			return ports.ErrConflict
		}
		next := nudgeNotificationChannelPreferenceModel{UserID: command.ActorUserID, Channel: string(command.Channel), Enabled: command.Enabled, Revision: current.Revision + 1, CreatedAt: command.OccurredAt, UpdatedAt: command.OccurredAt}
		if !found {
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
		} else {
			updated := tx.Model(&nudgeNotificationChannelPreferenceModel{}).
				Where("user_id = ? AND channel = ? AND revision = ?", command.ActorUserID, string(command.Channel), current.Revision).
				Updates(map[string]any{"enabled": command.Enabled, "revision": next.Revision, "updated_at": command.OccurredAt})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ports.ErrConflict
			}
		}

		if !command.Enabled {
			if err := suppressChannelPushDeliveries(tx, command.ActorUserID, string(command.Channel), command.OccurredAt); err != nil {
				return err
			}
		}

		replay = nudgeNotificationChannelReplayModel{ActorUserID: command.ActorUserID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), Channel: string(command.Channel), ResultEnabled: command.Enabled, ResultRevision: next.Revision, ResultUpdatedAt: command.OccurredAt, CreatedAt: command.OccurredAt}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return fmt.Errorf("%w: nudge notification channel audit: %v", ports.ErrUnavailable, err)
		}
		result.Preference = socialapp.NudgeNotificationChannelPreference{Enabled: command.Enabled, Revision: next.Revision}
		return nil
	})
	return result, classifyNudgeNotificationChannelError(err)
}

func validNotificationChannelCommand(repository *NudgeNotificationChannelRepository, command socialapp.NotificationChannelCommand) bool {
	key := command.Idempotency.Key
	if repository == nil || repository.db == nil || !command.Channel.Valid() || !validOpaquePersistenceID(command.ActorUserID) || command.ExpectedRevision < 0 || command.OccurredAt.IsZero() || command.OccurredAt.Location() != time.UTC ||
		command.Idempotency.PrincipalID != command.ActorUserID || command.Idempotency.Operation != socialapp.UpdateNudgeNotificationChannelOperation || len(key) < 16 || len(key) > 128 || strings.TrimSpace(key) != key || len(command.Idempotency.RequestHash) != sha256.Size ||
		!validMutationAudit(command.Audit, audit.ResourceUpdated, "notification_channel", string(command.Channel), command.ActorUserID) || command.Audit.ActorUserID != command.ActorUserID || !command.Audit.OccurredAt.Equal(command.OccurredAt) {
		return false
	}
	for index := range len(key) {
		if key[index] < 0x20 || key[index] > 0x7e {
			return false
		}
	}
	return true
}

func classifyNudgeNotificationChannelError(err error) error {
	switch {
	case err == nil, errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrConflict), errors.Is(err, ports.ErrIdempotencyConflict), errors.Is(err, ports.ErrUnavailable):
		return err
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return ports.ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ports.ErrConflict
	default:
		return fmt.Errorf("nudge notification channel persistence: %w: %v", ports.ErrUnavailable, err)
	}
}

var _ socialapp.NudgeNotificationChannelRepository = (*NudgeNotificationChannelRepository)(nil)

// The original Nudges endpoint remains compatible with already released clients.
func (repository *NudgeNotificationChannelRepository) GetNudgeNotificationChannel(ctx context.Context, actor string) (socialapp.NudgeNotificationChannelPreference, bool, error) {
	return repository.GetNotificationChannel(ctx, actor, notification.Channel(socialapp.NudgeNotificationChannel))
}
func (repository *NudgeNotificationChannelRepository) UpdateNudgeNotificationChannel(ctx context.Context, command socialapp.NudgeNotificationChannelCommand) (socialapp.NudgeNotificationChannelMutationResult, error) {
	return repository.UpdateNotificationChannel(ctx, socialapp.NotificationChannelCommand{Channel: notification.Channel(socialapp.NudgeNotificationChannel), NudgeNotificationChannelCommand: command})
}

var _ socialapp.NotificationChannelRepository = (*NudgeNotificationChannelRepository)(nil)

// The preference, queue retirement and audit commit together. Notifications stay
// visible in history, and enabling later cannot resurrect retired deliveries.
func suppressChannelPushDeliveries(tx *gorm.DB, recipient, channel string, at time.Time) error {
	return suppressPendingPushDeliveries(tx, recipient, channel, "channel_disabled", at)
}

func suppressPendingPushDeliveries(tx *gorm.DB, recipient, channel, reason string, at time.Time) error {
	notices := tx.Table("notification_models").Select("id").Where("recipient_user_id = ?", recipient)
	if channel != "" {
		notices = notices.Where("channel = ?", channel)
	}
	var ids []string
	if err := tx.Table("notification_push_delivery_models").Distinct("notification_id").Where("notification_id IN (?)", notices).Where("delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL AND provider_ticket = ''").Pluck("notification_id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Table("notification_push_delivery_models").Where("notification_id IN ?", ids).Where("delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL AND provider_ticket = ''").Updates(map[string]any{
		"suppressed_at": at, "failure_code": reason, "locked_by": nil, "locked_until": nil,
		"token_ciphertext": nil, "token_nonce": nil, "token_hash": nil,
	}).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := refreshNotificationPushOutbox(tx, id); err != nil {
			return err
		}
	}
	return nil
}
