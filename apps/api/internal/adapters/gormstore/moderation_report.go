package gormstore

import (
	"bytes"
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

type ModerationRepository struct{ db *gorm.DB }

func NewModerationRepository(db *gorm.DB) *ModerationRepository { return &ModerationRepository{db: db} }

type reportReceiptModel struct {
	ReporterID, IdempotencyKey, ReportID string
	SubjectUserID                        *string
	RequestHash                          []byte
	CreatedAt                            time.Time
}

func (reportReceiptModel) TableName() string { return "moderation_report_receipt_models" }

func (r *ModerationRepository) FindReportReplay(ctx context.Context, key ports.Idempotency) (app.Receipt, bool, error) {
	if r == nil || r.db == nil || key.PrincipalID == "" || key.Operation != app.SubmitOperation || len(key.RequestHash) != 32 {
		return app.Receipt{}, false, ports.ErrInvalidArgument
	}
	return findReportReplay(r.db.WithContext(ctx), key)
}
func findReportReplay(tx *gorm.DB, key ports.Idempotency) (app.Receipt, bool, error) {
	var row reportReceiptModel
	err := tx.Where("reporter_id=? AND idempotency_key=?", key.PrincipalID, key.Key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return app.Receipt{}, false, nil
	}
	if err != nil {
		return app.Receipt{}, false, err
	}
	if !bytes.Equal(row.RequestHash, key.RequestHash) {
		return app.Receipt{}, false, ports.ErrIdempotencyConflict
	}
	receipt, err := reportReceipt(tx, row.ReporterID, row.ReportID, row.SubjectUserID)
	return receipt, err == nil, err
}

func (r *ModerationRepository) ResolveReportTarget(ctx context.Context, viewer string, target domain.Target, at time.Time) (app.TargetAccess, error) {
	if r == nil || r.db == nil || viewer == "" || !target.Valid() || at.IsZero() {
		return app.TargetAccess{}, ports.ErrInvalidArgument
	}
	snapshot, err := resolveReportSnapshot(r.db.WithContext(ctx), viewer, target, at)
	return snapshot.Access, reportPersistenceError(err)
}

func (r *ModerationRepository) SubmitReport(ctx context.Context, c app.ReportCommand) (app.Receipt, error) {
	normalized, err := domain.NormalizeExplanation(c.Explanation)
	if r == nil || r.db == nil || err != nil || normalized != c.Explanation || c.ID == "" || c.ReporterID == "" || !c.Access.Target.Valid() || !c.Reason.Valid() || c.At.IsZero() || c.Idempotency.PrincipalID != c.ReporterID || c.Idempotency.Operation != app.SubmitOperation || len(c.Idempotency.RequestHash) != 32 || !c.Audit.Valid() || c.Audit.Action != audit.ResourceCreated || c.Audit.Outcome != audit.Succeeded || c.Audit.ActorUserID != c.ReporterID || c.Audit.OwnerUserID != c.ReporterID || c.Audit.TargetType != "moderation_report" || c.Audit.TargetID != c.ID || !c.Audit.OccurredAt.Equal(c.At) {
		return app.Receipt{}, ports.ErrInvalidArgument
	}
	var result app.Receipt
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The reporter lock serializes retries, account deletion and concurrent reports.
		var reporter userModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND status='active'", c.ReporterID).Take(&reporter).Error; err != nil {
			return err
		}
		receipt, found, err := findReportReplay(tx, c.Idempotency)
		if err != nil {
			return err
		}
		if found {
			result = receipt
			return nil
		}
		snapshot, err := resolveReportSnapshot(tx, c.ReporterID, c.Access.Target, c.At)
		if err != nil {
			return err
		}
		if snapshot.Access != c.Access {
			return ports.ErrNotFound
		}
		// Explicit INSERT avoids a RETURNING read privilege on restricted case data.
		if err = tx.Exec(`INSERT INTO moderation_case_models (id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,evidence_jpeg,created_at) VALUES (?,?,?,?,?,?,?,?::jsonb,?,?)`, c.ID, c.ReporterID, c.Access.SubjectUserID, string(c.Access.Target.Kind), c.Access.Target.ID, string(c.Reason), c.Explanation, string(snapshot.Evidence), snapshot.JPEG, c.At).Error; err != nil {
			return err
		}
		if err = tx.Create(&reportReceiptModel{ReporterID: c.ReporterID, IdempotencyKey: c.Idempotency.Key, RequestHash: c.Idempotency.RequestHash, ReportID: c.ID, SubjectUserID: &c.Access.SubjectUserID, CreatedAt: c.At}).Error; err != nil {
			return err
		}
		if err = appendAuditEvent(tx, c.Audit); err != nil {
			return err
		}
		result, err = reportReceipt(tx, c.ReporterID, c.ID, &c.Access.SubjectUserID)
		return err
	})
	return result, reportPersistenceError(err)
}
func reportPersistenceError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	return err
}

// A receipt retains an account reference, not a copy of their public identity.
// Replays resolve current active/unblocked identity and never return case data.
func reportReceipt(tx *gorm.DB, viewer, id string, subject *string) (app.Receipt, error) {
	result := app.Receipt{ID: id}
	if subject == nil || *subject == viewer {
		return result, nil
	}
	var row struct{ ID, Username, DisplayName string }
	err := tx.Table("user_models u").Select("u.id,u.username,u.display_name").
		Where("u.id=? AND u.status='active'", *subject).
		Where(`NOT EXISTS(SELECT 1 FROM block_models b WHERE (b.blocker_user_id=? AND b.blocked_user_id=u.id) OR (b.blocker_user_id=u.id AND b.blocked_user_id=?))`, viewer, viewer).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.BlockTarget = &app.BlockIdentity{UserID: row.ID, Username: row.Username, DisplayName: row.DisplayName}
	return result, nil
}
