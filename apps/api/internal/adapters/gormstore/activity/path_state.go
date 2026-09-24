package activitystore

import (
	"errors"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type pathStateRow struct {
	ArchivedAt *time.Time
}
type membershipStateRow struct{ JoinedAt time.Time }

func lockActivePath(tx *gorm.DB, pathID, participantID string) error {
	_, err := lockActivePathMembership(tx, pathID, participantID)
	return err
}

func lockActivePathAt(tx *gorm.DB, pathID, participantID string, occurrence time.Time) error {
	joinedAt, err := lockActivePathMembership(tx, pathID, participantID)
	if err != nil {
		return err
	}
	if occurrence.Before(joinedAt) {
		return ports.ErrNotFound
	}
	return nil
}

func lockActivePathMembership(tx *gorm.DB, pathID, participantID string) (time.Time, error) {
	if tx == nil || strings.TrimSpace(pathID) != pathID || pathID == "" || strings.TrimSpace(participantID) != participantID || participantID == "" {
		return time.Time{}, ports.ErrInvalidArgument
	}
	// The participant-and-Path progress lock is always acquired before row
	// locks. Consumers such as nudges follow the same order so progress writes
	// cannot deadlock with authorization or foreign-key row locks.
	if err := progresslock.Lock(tx, participantID, pathID); err != nil {
		return time.Time{}, err
	}
	var row pathStateRow
	err := tx.Table("path_models").Select("archived_at").Where("id = ?", pathID).
		Clauses(clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "path_models"}}).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return time.Time{}, ports.ErrNotFound
	}
	if err != nil {
		return time.Time{}, err
	}
	if row.ArchivedAt != nil {
		return time.Time{}, ports.ErrConflict
	}
	var membership membershipStateRow
	err = tx.Table("path_membership_models").Select("joined_at").Where("path_id = ? AND user_id = ? AND role IN ?", pathID, participantID, []string{"participant", "administrator"}).Clauses(clause.Locking{Strength: "SHARE"}).Take(&membership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return time.Time{}, ports.ErrNotFound
	}
	if err != nil {
		return time.Time{}, err
	}
	if membership.JoinedAt.IsZero() {
		return time.Time{}, ports.ErrNotFound
	}
	return membership.JoinedAt.UTC(), nil
}
