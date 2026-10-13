package gormstore

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"time"
)

func (s *Store) ActiveAccountRestriction(ctx context.Context, owner string, at time.Time) (*moderation.Enforcement, error) {
	if s == nil || s.DB == nil || owner == "" || at.IsZero() {
		return nil, ports.ErrInvalidArgument
	}
	var row enforcementModel
	err := s.DB.WithContext(ctx).Model(&enforcementModel{}).
		Where("subject_user_id=? AND issued_at<=? AND (action='ban' OR (action='suspension' AND until_at>?))", owner, at, at).
		Where("NOT EXISTS (SELECT 1 FROM moderation_appeal_decision_models reversal WHERE reversal.enforcement_id=moderation_enforcement_models.id AND reversal.outcome='reversed')").
		Order("CASE WHEN action='ban' THEN 0 ELSE 1 END, until_at DESC NULLS LAST, issued_at DESC, id DESC").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	decision := moderation.Enforcement{ID: row.ID, SubjectUserID: row.SubjectUserID, Action: moderation.EnforcementAction(row.Action), PolicyReason: row.PolicyReason, IssuedAt: row.IssuedAt}
	if row.UntilAt != nil {
		decision.Until = *row.UntilAt
	}
	if decision.SubjectUserID != owner || !decision.RestrictsAccount(at) {
		return nil, ports.ErrUnavailable
	}
	return &decision, nil
}
