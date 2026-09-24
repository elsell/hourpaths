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
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
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

func (repository *NudgeNotificationChannelRepository) GetNudgeNotificationChannel(ctx context.Context, actor string) (socialapp.NudgeNotificationChannelPreference, bool, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(actor) {
		return socialapp.NudgeNotificationChannelPreference{}, false, ports.ErrInvalidArgument
	}
	var row struct {
		Enabled  *bool
		Revision *int64
	}
	err := repository.db.WithContext(ctx).Table("user_models AS actor").
		Select("preference.enabled, preference.revision").
		Joins("LEFT JOIN notification_channel_preference_models preference ON preference.user_id = actor.id AND preference.channel = ?", socialapp.NudgeNotificationChannel).
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

func (repository *NudgeNotificationChannelRepository) UpdateNudgeNotificationChannel(ctx context.Context, command socialapp.NudgeNotificationChannelCommand) (socialapp.NudgeNotificationChannelMutationResult, error) {
	if !validNudgeNotificationChannelCommand(repository, command) {
		return socialapp.NudgeNotificationChannelMutationResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.NudgeNotificationChannelMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialInteractionOwner(tx, socialLockKey("notification-channel", command.ActorUserID)); err != nil {
			return err
		}
		var replay nudgeNotificationChannelReplayModel
		err := tx.Where("actor_user_id = ? AND operation = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			if replay.Channel != socialapp.NudgeNotificationChannel || replay.ResultEnabled != command.Enabled || replay.ResultRevision != command.ExpectedRevision+1 {
				return errors.New("persisted nudge notification channel replay is invalid")
			}
			result = socialapp.NudgeNotificationChannelMutationResult{Preference: socialapp.NudgeNotificationChannelPreference{Enabled: replay.ResultEnabled, Revision: replay.ResultRevision}, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		current, found, err := currentNudgeNotificationChannelForUpdate(ctx, tx, command.ActorUserID)
		if err != nil {
			return err
		}
		if current.Revision != command.ExpectedRevision {
			return ports.ErrConflict
		}
		next := nudgeNotificationChannelPreferenceModel{UserID: command.ActorUserID, Channel: socialapp.NudgeNotificationChannel, Enabled: command.Enabled, Revision: current.Revision + 1, CreatedAt: command.OccurredAt, UpdatedAt: command.OccurredAt}
		if !found {
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
		} else {
			updated := tx.Model(&nudgeNotificationChannelPreferenceModel{}).
				Where("user_id = ? AND channel = ? AND revision = ?", command.ActorUserID, socialapp.NudgeNotificationChannel, current.Revision).
				Updates(map[string]any{"enabled": command.Enabled, "revision": next.Revision, "updated_at": command.OccurredAt})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ports.ErrConflict
			}
		}

		replay = nudgeNotificationChannelReplayModel{ActorUserID: command.ActorUserID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), Channel: socialapp.NudgeNotificationChannel, ResultEnabled: command.Enabled, ResultRevision: next.Revision, ResultUpdatedAt: command.OccurredAt, CreatedAt: command.OccurredAt}
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

func currentNudgeNotificationChannelForUpdate(ctx context.Context, tx *gorm.DB, actor string) (socialapp.NudgeNotificationChannelPreference, bool, error) {
	repository := NewNudgeNotificationChannelRepository(tx)
	return repository.GetNudgeNotificationChannel(ctx, actor)
}

func validNudgeNotificationChannelCommand(repository *NudgeNotificationChannelRepository, command socialapp.NudgeNotificationChannelCommand) bool {
	key := command.Idempotency.Key
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(command.ActorUserID) || command.ExpectedRevision < 0 || command.OccurredAt.IsZero() || command.OccurredAt.Location() != time.UTC ||
		command.Idempotency.PrincipalID != command.ActorUserID || command.Idempotency.Operation != socialapp.UpdateNudgeNotificationChannelOperation || len(key) < 16 || len(key) > 128 || strings.TrimSpace(key) != key || len(command.Idempotency.RequestHash) != sha256.Size ||
		!validMutationAudit(command.Audit, audit.ResourceUpdated, "notification_channel", socialapp.NudgeNotificationChannel, command.ActorUserID) || command.Audit.ActorUserID != command.ActorUserID || !command.Audit.OccurredAt.Equal(command.OccurredAt) {
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
