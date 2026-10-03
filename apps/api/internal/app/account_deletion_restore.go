package app

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"time"
)

// DeletionRecord contains no provider identity, user content, or receipt secret.
// It is an offline operational record, not a public API contract.
type DeletionRecord struct {
	UserID       string    `json:"userId"`
	DeletedAt    time.Time `json:"deletedAt"`
	AuditEventID string    `json:"auditEventId"`
	ReceiptHash  string    `json:"receiptHash"`
}
type DeletionRestorePort interface {
	ReapplyAccountDeletion(context.Context, AccountDeletionCommand) error
}

func ReapplyDeletionRecords(ctx context.Context, records []DeletionRecord, repository DeletionRestorePort, newID func() string) error {
	if repository == nil || newID == nil || len(records) > 100000 {
		return ports.ErrInvalidArgument
	}
	if err := validateDeletionRecords(records); err != nil {
		return err
	}
	for _, record := range records {
		digest, _ := hex.DecodeString(record.ReceiptHash)
		event := audit.Event{ID: record.AuditEventID, OwnerUserID: record.UserID, ActorUserID: record.UserID, Action: audit.ResourceDeleted, TargetType: "account", TargetID: record.UserID, Outcome: audit.Succeeded, CorrelationID: newID(), OccurredAt: record.DeletedAt}
		if err := repository.ReapplyAccountDeletion(ctx, AccountDeletionCommand{UserID: record.UserID, DeletedAt: record.DeletedAt, ReceiptHash: digest, Audit: event, NewID: newID}); err != nil {
			return err
		}
	}
	return nil
}

func validateDeletionRecords(records []DeletionRecord) error {
	if len(records) > 100000 {
		return ports.ErrInvalidArgument
	}
	seen := make(map[string]bool, len(records))
	// Validate every record before performing even the first removal.
	for _, record := range records {
		digest, err := hex.DecodeString(record.ReceiptHash)
		if record.UserID == "" || strings.TrimSpace(record.UserID) != record.UserID || record.AuditEventID == "" || record.DeletedAt.IsZero() || err != nil || len(digest) != 32 || seen[record.UserID] {
			return ports.ErrInvalidArgument
		}
		seen[record.UserID] = true
	}
	return nil
}

// RecoverAcceptedDeletions retries irreversible admissions independently so a
// failed account cannot starve later requests. Offline restore remains fail-fast.
func RecoverAcceptedDeletions(ctx context.Context, records []DeletionRecord, repository DeletionRestorePort, newID func() string) error {
	if repository == nil || newID == nil {
		return ports.ErrInvalidArgument
	}
	if err := validateDeletionRecords(records); err != nil {
		return err
	}
	var failures error
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return errors.Join(failures, err)
		}
		failures = errors.Join(failures, ReapplyDeletionRecords(ctx, []DeletionRecord{record}, repository, newID))
	}
	return failures
}
