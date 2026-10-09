package gormstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	channelstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationchannel"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/preferences"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"time"
)

type unavailablePeriodModel struct {
	UserID                 string
	Enabled                bool
	StartMinute, EndMinute int
	Revision               int64
	UpdatedAt              time.Time
}

func (unavailablePeriodModel) TableName() string { return "user_unavailable_period_models" }

type unavailablePeriodMutationModel struct {
	UserID, Operation, IdempotencyKey  string
	RequestHash                        []byte
	ResultEnabled                      bool
	ResultStartMinute, ResultEndMinute int
	ResultRevision                     int64
	ResultTimeZone                     string
	CreatedAt                          time.Time
}

func (unavailablePeriodMutationModel) TableName() string {
	return "user_unavailable_period_mutation_models"
}
func validUnavailableMinutes(p preferences.UnavailablePeriod) bool {
	return p.StartMinute >= 0 && p.StartMinute < 1440 && p.EndMinute >= 0 && p.EndMinute < 1440
}

func (s *Store) GetUnavailablePeriod(ctx context.Context, userID string) (application.UnavailablePeriodPreference, error) {
	if s == nil || s.DB == nil || userID == "" {
		return application.UnavailablePeriodPreference{}, ports.ErrInvalidArgument
	}
	var row struct {
		TimeZone               string
		Enabled                bool
		StartMinute, EndMinute int
		Revision               int64
	}
	err := s.DB.WithContext(ctx).Table("user_preference_models p").Select("p.current_time_zone AS time_zone, COALESCE(q.enabled, false) AS enabled, COALESCE(q.start_minute, 0) AS start_minute, COALESCE(q.end_minute, 0) AS end_minute, COALESCE(q.revision, 0) AS revision").Joins("JOIN user_models u ON u.id=p.user_id AND u.status='active'").Joins("LEFT JOIN user_unavailable_period_models q ON q.user_id=p.user_id").Where("p.user_id=?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return application.UnavailablePeriodPreference{}, ports.ErrNotFound
	}
	if err != nil {
		return application.UnavailablePeriodPreference{}, classifyUnavailablePeriodError(err)
	}
	value := application.UnavailablePeriodPreference{Period: preferences.UnavailablePeriod{Enabled: row.Enabled, StartMinute: row.StartMinute, EndMinute: row.EndMinute}, Revision: row.Revision, TimeZone: row.TimeZone}
	if !validUnavailableMinutes(value.Period) || row.Revision < 0 || row.TimeZone == "" {
		return application.UnavailablePeriodPreference{}, ports.ErrUnavailable
	}
	if _, err := time.LoadLocation(row.TimeZone); err != nil {
		return application.UnavailablePeriodPreference{}, ports.ErrUnavailable
	}
	return value, nil
}
func (s *Store) UpdateUnavailablePeriod(ctx context.Context, c application.UnavailablePeriodCommand) (application.UnavailablePeriodResult, error) {
	if s == nil || s.DB == nil || !validUnavailablePeriodCommand(c) {
		return application.UnavailablePeriodResult{}, ports.ErrInvalidArgument
	}
	var result application.UnavailablePeriodResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Match notification producers and preference mutations before serializing handoff.
		if err := lockSocialInteractionOwner(tx, c.ActorUserID); err != nil {
			return err
		}
		var owner struct{ ID string }
		if err := tx.Table("user_models").Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND status='active'", c.ActorUserID).Take(&owner).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if err := lockSocialInteractionOwner(tx, socialLockKey("notification-channel", c.ActorUserID)); err != nil {
			return err
		}
		var replay unavailablePeriodMutationModel
		err := tx.Where("user_id=? AND operation=? AND idempotency_key=?", c.ActorUserID, c.Idempotency.Operation, c.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, c.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			p := preferences.UnavailablePeriod{Enabled: replay.ResultEnabled, StartMinute: replay.ResultStartMinute, EndMinute: replay.ResultEndMinute}
			if !validUnavailableMinutes(p) || p != c.Period || replay.ResultRevision != c.ExpectedRevision+1 || replay.ResultTimeZone != c.ReviewedTimeZone {
				return ports.ErrUnavailable
			}
			result = application.UnavailablePeriodResult{Preference: application.UnavailablePeriodPreference{Period: p, Revision: replay.ResultRevision, TimeZone: replay.ResultTimeZone}, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		current, err := (&Store{DB: tx}).GetUnavailablePeriod(ctx, c.ActorUserID)
		if err != nil {
			return err
		}
		if current.Revision != c.ExpectedRevision || current.TimeZone != c.ReviewedTimeZone {
			return ports.ErrConflict
		}
		next := unavailablePeriodModel{UserID: c.ActorUserID, Enabled: c.Period.Enabled, StartMinute: c.Period.StartMinute, EndMinute: c.Period.EndMinute, Revision: current.Revision + 1, UpdatedAt: c.ChangedAt}
		if current.Revision == 0 {
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
		} else {
			updated := tx.Model(&unavailablePeriodModel{}).Where("user_id=? AND revision=?", c.ActorUserID, current.Revision).Updates(map[string]any{"enabled": next.Enabled, "start_minute": next.StartMinute, "end_minute": next.EndMinute, "revision": next.Revision, "updated_at": next.UpdatedAt})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ports.ErrConflict
			}
		}
		if err := suppressQuietPeriodPush(tx, c.ActorUserID, c.ChangedAt); err != nil {
			return err
		}
		replay = unavailablePeriodMutationModel{UserID: c.ActorUserID, Operation: c.Idempotency.Operation, IdempotencyKey: c.Idempotency.Key, RequestHash: append([]byte(nil), c.Idempotency.RequestHash...), ResultEnabled: next.Enabled, ResultStartMinute: next.StartMinute, ResultEndMinute: next.EndMinute, ResultRevision: next.Revision, ResultTimeZone: current.TimeZone, CreatedAt: c.ChangedAt}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		if err := appendAuditEvent(tx, c.Audit); err != nil {
			return err
		}
		result.Preference = application.UnavailablePeriodPreference{Period: c.Period, Revision: next.Revision, TimeZone: current.TimeZone}
		return nil
	})
	return result, classifyUnavailablePeriodError(err)
}
func validUnavailablePeriodCommand(c application.UnavailablePeriodCommand) bool {
	return c.ActorUserID != "" && c.ExpectedRevision >= 0 && c.ExpectedRevision < math.MaxInt64 && c.ReviewedTimeZone != "" && validUnavailableMinutes(c.Period) && !c.ChangedAt.IsZero() && c.ChangedAt.Location() == time.UTC &&
		c.Idempotency.PrincipalID == c.ActorUserID && c.Idempotency.Operation == application.UpdateUnavailablePeriodOperation && len(c.Idempotency.Key) >= 16 && len(c.Idempotency.Key) <= 128 && len(c.Idempotency.RequestHash) == 32 &&
		c.Audit.Valid() && c.Audit.OwnerUserID == c.ActorUserID && c.Audit.ActorUserID == c.ActorUserID && c.Audit.TargetID == c.ActorUserID && c.Audit.TargetType == "unavailable_period" && c.Audit.Action == audit.ResourceUpdated && c.Audit.Outcome == audit.Succeeded && c.Audit.OccurredAt == c.ChangedAt
}
func classifyUnavailablePeriodError(err error) error {
	switch {
	case err == nil, errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrConflict), errors.Is(err, ports.ErrIdempotencyConflict), errors.Is(err, ports.ErrUnavailable):
		return err
	default:
		return fmt.Errorf("unavailable-period persistence: %w: %v", ports.ErrUnavailable, err)
	}
}

// The caller holds the notification-channel lock across preference changes and
// queue retirement, so ending quiet hours cannot revive work already suppressed.
func suppressQuietPeriodPush(tx *gorm.DB, owner string, at time.Time) error {
	quiet, err := channelstore.QuietAt(tx, owner, at)
	if err != nil || !quiet {
		return err
	}
	return suppressPendingPushDeliveries(tx, owner, "", "quiet_period", at)
}
