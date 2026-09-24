package gormstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type timeZonePreferenceMutationModel struct {
	UserID, Operation, IdempotencyKey                  string
	RequestHash                                        []byte
	ReviewedTimeZone, ProposedTimeZone, ResultTimeZone string
	ResultEffectiveAt, CreatedAt                       time.Time
	ResultChanged                                      bool
}

func (timeZonePreferenceMutationModel) TableName() string {
	return "user_time_zone_preference_mutation_models"
}

func (s *Store) GetTimeZonePreference(ctx context.Context, userID string) (application.TimeZonePreference, error) {
	if s == nil || s.DB == nil || userID == "" {
		return application.TimeZonePreference{}, ports.ErrInvalidArgument
	}
	var row struct {
		CurrentTimeZone string
		HistoryTimeZone string
		EffectiveAt     time.Time
	}
	err := s.DB.WithContext(ctx).Table("user_preference_models AS preferences").
		Select("preferences.current_time_zone, history.time_zone AS history_time_zone, history.effective_at").
		Joins("JOIN user_models AS users ON users.id = preferences.user_id AND users.status = ?", identity.StatusActive).
		Joins("JOIN user_time_zone_history_models AS history ON history.user_id = preferences.user_id").
		Where("preferences.user_id = ?", userID).
		Order("history.effective_at DESC").Limit(1).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return application.TimeZonePreference{}, ports.ErrNotFound
	}
	if err != nil {
		return application.TimeZonePreference{}, classifyTimeZonePreferenceError(err)
	}
	preference := application.TimeZonePreference{TimeZone: row.CurrentTimeZone, EffectiveAt: row.EffectiveAt.UTC()}
	if row.HistoryTimeZone != row.CurrentTimeZone || !validPersistedTimeZonePreference(preference) {
		return application.TimeZonePreference{}, fmt.Errorf("time-zone preference persistence: %w", ports.ErrUnavailable)
	}
	return preference, nil
}

func (s *Store) UpdateTimeZonePreference(ctx context.Context, command application.TimeZonePreferenceCommand) (application.TimeZonePreferenceResult, error) {
	if s == nil || s.DB == nil || !validTimeZonePreferenceCommand(command) {
		return application.TimeZonePreferenceResult{}, ports.ErrInvalidArgument
	}
	var result application.TimeZonePreferenceResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner struct{ ID string }
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("user_models").Select("id").
			Where("id = ? AND status = ?", command.ActorUserID, identity.StatusActive).Take(&owner).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrNotFound
		}
		if err != nil {
			return err
		}

		var replay timeZonePreferenceMutationModel
		err = tx.Where("user_id = ? AND operation = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = timeZonePreferenceReplayResult(replay)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var preference struct {
			CurrentTimeZone string
			UpdatedAt       time.Time
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("user_preference_models").
			Select("current_time_zone, updated_at").Where("user_id = ?", command.ActorUserID).Take(&preference).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if identity.ValidateIANATimeZone(identity.IANATimeZone(preference.CurrentTimeZone)) != nil {
			return ports.ErrUnavailable
		}
		if preference.CurrentTimeZone != command.ReviewedTimeZone {
			return ports.ErrConflict
		}
		var currentHistory userTimeZoneHistoryModel
		if err := tx.Where("user_id = ?", command.ActorUserID).
			Order("effective_at DESC").Take(&currentHistory).Error; err != nil {
			return err
		}
		if currentHistory.EffectiveAt.IsZero() || currentHistory.TimeZone != preference.CurrentTimeZone ||
			identity.ValidateIANATimeZone(identity.IANATimeZone(currentHistory.TimeZone)) != nil {
			return ports.ErrUnavailable
		}

		changed := command.ProposedTimeZone != preference.CurrentTimeZone
		effectiveAt := currentHistory.EffectiveAt.UTC()
		if changed {
			if !command.ChangedAt.After(effectiveAt) {
				return ports.ErrConflict
			}
			effectiveAt = command.ChangedAt
			updated := tx.Table("user_preference_models").Where("user_id = ? AND current_time_zone = ?", command.ActorUserID, command.ReviewedTimeZone).
				Updates(map[string]any{"current_time_zone": command.ProposedTimeZone, "updated_at": command.ChangedAt})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ports.ErrConflict
			}
			if err := tx.Create(&userTimeZoneHistoryModel{UserID: command.ActorUserID, EffectiveAt: effectiveAt, TimeZone: command.ProposedTimeZone}).Error; err != nil {
				return err
			}
		}
		replay = timeZonePreferenceMutationModel{
			UserID: command.ActorUserID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key,
			RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), ReviewedTimeZone: command.ReviewedTimeZone,
			ProposedTimeZone: command.ProposedTimeZone, ResultTimeZone: command.ProposedTimeZone,
			ResultEffectiveAt: effectiveAt, ResultChanged: changed, CreatedAt: command.ChangedAt,
		}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		result = application.TimeZonePreferenceResult{Preference: application.TimeZonePreference{TimeZone: command.ProposedTimeZone, EffectiveAt: effectiveAt}, Changed: changed}
		return nil
	})
	return result, classifyTimeZonePreferenceError(err)
}

func validTimeZonePreferenceCommand(command application.TimeZonePreferenceCommand) bool {
	return command.ActorUserID != "" && command.ChangedAt.Location() == time.UTC && !command.ChangedAt.IsZero() &&
		identity.ValidateIANATimeZone(identity.IANATimeZone(command.ReviewedTimeZone)) == nil &&
		identity.ValidateIANATimeZone(identity.IANATimeZone(command.ProposedTimeZone)) == nil &&
		command.Idempotency.PrincipalID == command.ActorUserID && command.Idempotency.Operation == application.UpdateTimeZonePreferenceOperation &&
		len(command.Idempotency.Key) >= 16 && len(command.Idempotency.Key) <= 128 && len(command.Idempotency.RequestHash) == 32 &&
		command.Audit.Valid() && command.Audit.OwnerUserID == command.ActorUserID && command.Audit.ActorUserID == command.ActorUserID &&
		command.Audit.Action == audit.ResourceUpdated && command.Audit.TargetType == "time_zone_preference" && command.Audit.TargetID == command.ActorUserID &&
		command.Audit.Outcome == audit.Succeeded && command.Audit.OccurredAt == command.ChangedAt
}

func validPersistedTimeZonePreference(preference application.TimeZonePreference) bool {
	return !preference.EffectiveAt.IsZero() && identity.ValidateIANATimeZone(identity.IANATimeZone(preference.TimeZone)) == nil
}

func timeZonePreferenceReplayResult(replay timeZonePreferenceMutationModel) application.TimeZonePreferenceResult {
	return application.TimeZonePreferenceResult{
		Preference: application.TimeZonePreference{TimeZone: replay.ResultTimeZone, EffectiveAt: replay.ResultEffectiveAt.UTC()},
		Changed:    replay.ResultChanged, Replayed: true,
	}
}

func classifyTimeZonePreferenceError(err error) error {
	switch {
	case err == nil, errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrConflict), errors.Is(err, ports.ErrIdempotencyConflict), errors.Is(err, ports.ErrUnavailable):
		return err
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ports.ErrConflict
	default:
		return fmt.Errorf("time-zone preference persistence: %w: %v", ports.ErrUnavailable, err)
	}
}
