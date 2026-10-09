package gormstore

import (
	"context"
	"errors"
	"time"

	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type enforcementModel struct {
	ID, SubjectUserID, Action, PolicyReason string
	AffectedCommentID                       string
	AffectedCommentCreatedAt                *time.Time
	IssuedAt                                time.Time
	UntilAt                                 *time.Time
}

func (enforcementModel) TableName() string { return "moderation_enforcement_models" }

type appealModel struct {
	EnforcementID, ID, Explanation string
	SubmittedAt                    time.Time
}

func (appealModel) TableName() string { return "moderation_appeal_models" }

type appealDecisionModel struct {
	EnforcementID, Outcome, Reviewer, Reason string
	DecidedAt                                time.Time
}

func (appealDecisionModel) TableName() string { return "moderation_appeal_decision_models" }

func readEnforcement(tx *gorm.DB, owner, id string) (app.Notice, error) {
	var e enforcementModel
	if err := tx.Where("subject_user_id=? AND id=?", owner, id).Take(&e).Error; err != nil {
		return app.Notice{}, reportPersistenceError(err)
	}
	n := app.Notice{Decision: domain.Enforcement{ID: e.ID, SubjectUserID: e.SubjectUserID, Action: domain.EnforcementAction(e.Action), PolicyReason: e.PolicyReason, IssuedAt: e.IssuedAt}}
	if e.AffectedCommentID != "" && e.AffectedCommentCreatedAt != nil {
		n.Decision.AffectedCommentID = e.AffectedCommentID
		n.Decision.AffectedCommentCreatedAt = *e.AffectedCommentCreatedAt
	}
	if e.UntilAt != nil {
		n.Decision.Until = *e.UntilAt
	}
	if n.Decision.Validate() != nil {
		return app.Notice{}, ports.ErrUnavailable
	}
	var a appealModel
	err := tx.Where("enforcement_id=?", e.ID).Take(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return n, nil
	}
	if err != nil {
		return app.Notice{}, err
	}
	n.Appeal = &domain.Appeal{ID: a.ID, EnforcementID: e.ID, Explanation: a.Explanation, SubmittedAt: a.SubmittedAt}
	var decision appealDecisionModel
	err = tx.Where("enforcement_id=?", e.ID).Take(&decision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return n, nil
	}
	if err != nil {
		return app.Notice{}, err
	}
	n.Appeal.Outcome = domain.AppealOutcome(decision.Outcome)
	n.Appeal.Reviewer = decision.Reviewer
	n.Appeal.DecisionReason = decision.Reason
	n.Appeal.DecidedAt = decision.DecidedAt
	return n, nil
}
func validNoticeAudit(e audit.Event, owner, id, kind string, action audit.Action) bool {
	return owner != "" && id != "" && e.Valid() && e.ActorUserID == owner && e.OwnerUserID == owner && e.TargetID == id && e.TargetType == kind && e.Action == action && e.Outcome == audit.Succeeded
}
func (r *ModerationRepository) ReadNotice(ctx context.Context, owner, id string, event audit.Event) (app.Notice, error) {
	if r == nil || r.db == nil || !validNoticeAudit(event, owner, id, "enforcement_notice", audit.ResourceViewed) {
		return app.Notice{}, ports.ErrInvalidArgument
	}
	var result app.Notice
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = readEnforcement(tx, owner, id)
		if err != nil {
			return err
		}
		return appendAuditEvent(tx, event)
	})
	return result, reportPersistenceError(err)
}
func (r *ModerationRepository) SubmitAppeal(ctx context.Context, c app.AppealCommand) (domain.Appeal, error) {
	if r == nil || r.db == nil || !validNoticeAudit(c.Audit, c.OwnerID, c.Appeal.EnforcementID, "moderation_appeal", audit.ResourceCreated) || !c.Audit.OccurredAt.Equal(c.Appeal.SubmittedAt) || c.Appeal.Outcome != "" || c.Appeal.Reviewer != "" || c.Appeal.DecisionReason != "" || !c.Appeal.DecidedAt.IsZero() {
		return domain.Appeal{}, ports.ErrInvalidArgument
	}
	var result domain.Appeal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Use the same account lock as deletion, serializing all submissions and
		// preventing a successful appeal from racing removal of its subject.
		var user userModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND status='active'", c.OwnerID).Take(&user).Error; err != nil {
			return err
		}
		notice, err := readEnforcement(tx, c.OwnerID, c.Appeal.EnforcementID)
		if err != nil {
			return err
		}
		if notice.Appeal != nil {
			if notice.Appeal.ID != c.Appeal.ID || notice.Appeal.Explanation != c.Appeal.Explanation {
				return ports.ErrConflict
			}
			result = *notice.Appeal
			return nil
		}
		candidate, err := notice.Decision.NewAppeal(c.Appeal.ID, c.Appeal.Explanation, c.Appeal.SubmittedAt)
		if err != nil || candidate != c.Appeal {
			return ports.ErrConflict
		}
		row := appealModel{EnforcementID: candidate.EnforcementID, ID: candidate.ID, Explanation: candidate.Explanation, SubmittedAt: candidate.SubmittedAt}
		if err = tx.Create(&row).Error; err != nil {
			return err
		}
		if err = appendAuditEvent(tx, c.Audit); err != nil {
			return err
		}
		result = candidate
		return nil
	})
	return result, reportPersistenceError(err)
}

func (r *ModerationRepository) ListNotices(ctx context.Context, owner string, p ports.PageRequest, event audit.Event) (app.NoticePage, error) {
	if r == nil || r.db == nil || p.Limit < 1 || p.Limit > 100 || p.Snapshot.IsZero() || !validNoticeAudit(event, owner, "owned", "enforcement_notice", audit.ResourceListed) {
		return app.NoticePage{}, ports.ErrInvalidArgument
	}
	result := app.NoticePage{Items: []app.Notice{}}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&enforcementModel{}).Select("id").Where("subject_user_id=? AND issued_at<=?", owner, p.Snapshot)
		if p.AfterID != "" {
			q = q.Where("(issued_at,id)<(?,?)", p.AfterCreated, p.AfterID)
		}
		var ids []string
		if err := q.Order("issued_at DESC,id DESC").Limit(p.Limit+1).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) > p.Limit {
			result.HasMore = true
			ids = ids[:p.Limit]
		}
		for _, id := range ids {
			notice, err := readEnforcement(tx, owner, id)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, notice)
		}
		return appendAuditEvent(tx, event)
	})
	return result, reportPersistenceError(err)
}
