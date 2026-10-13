package operationalmoderation

import (
	"context"
	"errors"
	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// AccountEnforcer is composed only by restricted operator tooling.
// Ordinary API and retention roles have no enforcement/case write privileges.
type AccountEnforcer struct {
	DB    *gorm.DB
	Clock ports.Clock
	NewID func() string
}

var _ app.AccountEnforcementPort = AccountEnforcer{}

type operationalCase struct{ ID, SubjectUserID, State string }

func (r AccountEnforcer) RestrictAccount(ctx context.Context, c app.AccountEnforcementCommand) (domain.Enforcement, error) {
	c.Until = c.Until.UTC()
	if c.Validate() != nil || r.DB == nil || r.Clock == nil || r.NewID == nil {
		return domain.Enforcement{}, ports.ErrInvalidArgument
	}
	var result domain.Enforcement
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var operator struct {
			Reviewer string
			Allowed  bool
		}
		// hourpaths-direct-sql: allow checking the actual database operational principal and its restricted write capability
		if err := tx.Raw("SELECT session_user AS reviewer, has_table_privilege(current_user, 'moderation_enforcement_models', 'INSERT') AS allowed").Scan(&operator).Error; err != nil {
			return err
		}
		if !operator.Allowed || operator.Reviewer == "" {
			return errors.New("operational moderation credential required")
		}
		var reviewed operationalCase
		if err := tx.Table("moderation_case_models").Where("id=?", c.CaseID).Take(&reviewed).Error; err != nil {
			return err
		}
		if reviewed.SubjectUserID == "" {
			return ports.ErrNotFound
		}
		var user struct{ ID string }
		if err := tx.Table("user_models").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND status='active'", reviewed.SubjectUserID).Take(&user).Error; err != nil {
			return err
		}
		if err := tx.Table("moderation_case_models").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND subject_user_id=?", c.CaseID, user.ID).Take(&reviewed).Error; err != nil {
			return err
		}
		var previous enforcementModel
		err := tx.Where("id=?", c.NoticeID).Take(&previous).Error
		if err == nil {
			until := time.Time{}
			if previous.UntilAt != nil {
				until = previous.UntilAt.UTC()
			}
			if previous.CaseID != c.CaseID || previous.SubjectUserID != user.ID || previous.Action != string(c.Action) || previous.PolicyReason != c.PolicyReason || !until.Equal(c.Until) {
				return ports.ErrIdempotencyConflict
			}
			result = domain.Enforcement{ID: previous.ID, SubjectUserID: user.ID, Action: c.Action, PolicyReason: c.PolicyReason, IssuedAt: previous.IssuedAt.UTC(), Until: until}
			return result.Validate()
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if reviewed.State != "open" && reviewed.State != "reviewing" {
			return ports.ErrConflict
		}
		at := r.Clock.Now().UTC().Truncate(time.Microsecond)
		result = domain.Enforcement{ID: c.NoticeID, SubjectUserID: user.ID, Action: c.Action, PolicyReason: c.PolicyReason, IssuedAt: at, Until: c.Until}
		if result.Validate() != nil {
			return ports.ErrInvalidArgument
		}
		row := enforcementModel{ID: c.NoticeID, CaseID: c.CaseID, SubjectUserID: user.ID, Action: string(c.Action), PolicyReason: c.PolicyReason, IssuedAt: at}
		if !c.Until.IsZero() {
			until := c.Until.UTC()
			row.UntilAt = &until
		}
		if err = tx.Omit("AffectedCommentID", "AffectedCommentCreatedAt").Create(&row).Error; err != nil {
			return err
		}
		var timers []struct{ PathID string }
		if err = tx.Table("running_timer_models").Select("path_id").Where("participant_id=?", user.ID).Order("path_id").Find(&timers).Error; err != nil {
			return err
		}
		for _, timer := range timers {
			event := audit.Event{ID: r.NewID(), OwnerUserID: user.ID, ActorUserID: user.ID, Action: audit.ActivityTimerStopped, TargetType: "timer", TargetID: "pending", Outcome: audit.Succeeded, CorrelationID: c.NoticeID, OccurredAt: at}
			if _, err = activitystore.New(tx).StopParticipantTimerForLifecycle(ctx, timer.PathID, user.ID, at, r.NewID(), event); err != nil {
				return err
			}
		}
		if err = tx.Table("moderation_case_models").Where("id=?", c.CaseID).Updates(map[string]any{"state": "actioned", "reviewer": operator.Reviewer, "closed_at": at}).Error; err != nil {
			return err
		}
		return tx.Table("moderation_review_event_models").Create(map[string]any{"case_id": c.CaseID, "actor_role": operator.Reviewer, "operation": string(c.Action), "reason": c.PolicyReason, "occurred_at": at}).Error
	})
	if err != nil {
		return domain.Enforcement{}, err
	}
	return result, nil
}

type enforcementModel struct {
	ID, CaseID, SubjectUserID, Action, PolicyReason string
	IssuedAt                                        time.Time
	UntilAt                                         *time.Time
}

func (enforcementModel) TableName() string { return "moderation_enforcement_models" }
