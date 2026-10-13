package accountguard

import (
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LockActive serializes timer/progress writes with operational enforcement. The
// caller holds this shared account row lock until its transaction commits.
func LockActive(tx *gorm.DB, owner string) error {
	var row struct{ ID string }
	err := tx.Table("user_models").Select("id").Where("id=? AND status='active'", owner).Clauses(clause.Locking{Strength: "SHARE"}).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	if err != nil {
		return err
	}
	var count int64
	if err = tx.Table("moderation_enforcement_models").Where("subject_user_id=? AND issued_at<=clock_timestamp() AND (action='ban' OR (action='suspension' AND until_at>clock_timestamp()))", owner).
		Where("NOT EXISTS (SELECT 1 FROM moderation_appeal_decision_models reversal WHERE reversal.enforcement_id=moderation_enforcement_models.id AND reversal.outcome='reversed')").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ports.ErrAccountRestricted
	}
	return nil
}
