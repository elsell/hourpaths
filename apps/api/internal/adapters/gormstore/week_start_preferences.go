package gormstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type weekStartPreferenceMutationModel struct {
	UserID, Operation, IdempotencyKey                                    string
	RequestHash                                                          []byte
	ReviewedFirstDayOfWeek, ProposedFirstDayOfWeek, ResultFirstDayOfWeek int
	ResultChanged                                                        bool
	CreatedAt                                                            time.Time
}

func (weekStartPreferenceMutationModel) TableName() string {
	return "user_week_start_preference_mutation_models"
}
func (s *Store) GetWeekStartPreference(ctx context.Context, userID string) (application.WeekStartPreference, error) {
	if s == nil || s.DB == nil || userID == "" {
		return application.WeekStartPreference{}, ports.ErrInvalidArgument
	}
	var row application.WeekStartPreference
	err := s.DB.WithContext(ctx).Table("user_preference_models AS preferences").Select("preferences.first_day_of_week").Joins("JOIN user_models AS users ON users.id = preferences.user_id AND users.status = ?", identity.StatusActive).Where("preferences.user_id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return application.WeekStartPreference{}, ports.ErrNotFound
	}
	if err != nil {
		return application.WeekStartPreference{}, classifyWeekStartError(err)
	}
	if !validStoredWeekStart(row.FirstDayOfWeek) {
		return application.WeekStartPreference{}, ports.ErrUnavailable
	}
	return row, nil
}
func (s *Store) UpdateWeekStartPreference(ctx context.Context, c application.WeekStartPreferenceCommand) (application.WeekStartPreferenceResult, error) {
	if s == nil || s.DB == nil || !validWeekStartCommand(c) {
		return application.WeekStartPreferenceResult{}, ports.ErrInvalidArgument
	}
	var result application.WeekStartPreferenceResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner struct{ ID string }
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("user_models").Select("id").Where("id = ? AND status = ?", c.ActorUserID, identity.StatusActive).Take(&owner).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrNotFound
		}
		if err != nil {
			return err
		}
		var replay weekStartPreferenceMutationModel
		err = tx.Where("user_id = ? AND operation = ? AND idempotency_key = ?", c.ActorUserID, c.Idempotency.Operation, c.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, c.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			if !validStoredWeekStart(replay.ResultFirstDayOfWeek) {
				return ports.ErrUnavailable
			}
			result = application.WeekStartPreferenceResult{Preference: application.WeekStartPreference{FirstDayOfWeek: replay.ResultFirstDayOfWeek}, Changed: replay.ResultChanged, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var current application.WeekStartPreference
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("user_preference_models").Select("first_day_of_week").Where("user_id = ?", c.ActorUserID).Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !validStoredWeekStart(current.FirstDayOfWeek) {
			return ports.ErrUnavailable
		}
		if current.FirstDayOfWeek != c.ReviewedFirstDayOfWeek {
			return ports.ErrConflict
		}
		changed := current.FirstDayOfWeek != c.ProposedFirstDayOfWeek
		if changed {
			update := tx.Table("user_preference_models").Where("user_id = ? AND first_day_of_week = ?", c.ActorUserID, c.ReviewedFirstDayOfWeek).Updates(map[string]any{"first_day_of_week": c.ProposedFirstDayOfWeek, "updated_at": c.ChangedAt})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return ports.ErrConflict
			}
		}
		replay = weekStartPreferenceMutationModel{UserID: c.ActorUserID, Operation: c.Idempotency.Operation, IdempotencyKey: c.Idempotency.Key, RequestHash: append([]byte(nil), c.Idempotency.RequestHash...), ReviewedFirstDayOfWeek: c.ReviewedFirstDayOfWeek, ProposedFirstDayOfWeek: c.ProposedFirstDayOfWeek, ResultFirstDayOfWeek: c.ProposedFirstDayOfWeek, ResultChanged: changed, CreatedAt: c.ChangedAt}
		if err = tx.Create(&replay).Error; err != nil {
			return err
		}
		if err = appendAuditEvent(tx, c.Audit); err != nil {
			return err
		}
		result = application.WeekStartPreferenceResult{Preference: application.WeekStartPreference{FirstDayOfWeek: c.ProposedFirstDayOfWeek}, Changed: changed}
		return nil
	})
	return result, classifyWeekStartError(err)
}
func validStoredWeekStart(value int) bool {
	return value >= 1 && value <= 7 && identity.ValidateFirstDayOfWeek(identity.FirstDayOfWeek(value)) == nil
}
func validWeekStartCommand(c application.WeekStartPreferenceCommand) bool {
	return c.ActorUserID != "" && !c.ChangedAt.IsZero() && c.ChangedAt.Location() == time.UTC && validStoredWeekStart(c.ReviewedFirstDayOfWeek) && validStoredWeekStart(c.ProposedFirstDayOfWeek) &&
		c.Idempotency.PrincipalID == c.ActorUserID && c.Idempotency.Operation == application.UpdateWeekStartPreferenceOperation && len(c.Idempotency.Key) >= 16 && len(c.Idempotency.Key) <= 128 && len(c.Idempotency.RequestHash) == 32 &&
		c.Audit.Valid() && c.Audit.OwnerUserID == c.ActorUserID && c.Audit.ActorUserID == c.ActorUserID && c.Audit.TargetID == c.ActorUserID && c.Audit.TargetType == "week_start_preference" && c.Audit.Action == audit.ResourceUpdated && c.Audit.Outcome == audit.Succeeded && c.Audit.OccurredAt == c.ChangedAt
}
func classifyWeekStartError(err error) error {
	switch {
	case err == nil, errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrConflict), errors.Is(err, ports.ErrIdempotencyConflict), errors.Is(err, ports.ErrUnavailable):
		return err
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ports.ErrConflict
	default:
		return fmt.Errorf("week-start preference persistence: %w: %v", ports.ErrUnavailable, err)
	}
}
