package auditretention

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
)

type accountDeletionRun struct {
	ID                  string `gorm:"primaryKey"`
	BatchLimit          int
	DeletedCount        int
	OutboxDeletedCount  int
	ReceiptDeletedCount int
	CompletedAt         time.Time
}

func (accountDeletionRun) TableName() string { return "account_deletion_retention_run_models" }

// DeletedAccountBatch uses the same restricted retention role, with eligibility
// calculated by the database instead of accepting a caller-controlled cutoff.
func (r Runner) DeletedAccountBatch(ctx context.Context, limit int) (int, error) {
	if r.DB == nil || limit < 1 || limit > 10000 {
		return 0, errors.New("invalid deleted account retention batch")
	}
	run := accountDeletionRun{ID: uuid.NewString(), BatchLimit: limit}
	if err := r.DB.WithContext(ctx).Omit("CompletedAt", "DeletedCount", "OutboxDeletedCount", "ReceiptDeletedCount").Create(&run).Error; err != nil {
		return 0, err
	}
	if err := r.DB.WithContext(ctx).First(&run, "id = ?", run.ID).Error; err != nil {
		return 0, err
	}
	return run.DeletedCount + run.OutboxDeletedCount + run.ReceiptDeletedCount, nil
}
