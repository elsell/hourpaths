package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type AccountDeletionRequest struct {
	Confirmed      bool
	ReviewedUserID string
	ReceiptSecret  string
}

type AccountDeletionCommand struct {
	UserID      string
	DeletedAt   time.Time
	Audit       audit.Event
	NewID       func() string
	ReceiptHash []byte
}

type AccountDeletionJournal interface {
	Admit(context.Context, DeletionRecord) (DeletionRecord, error)
}

// AccountDeletionRepository commits removal and its audit evidence atomically.
// It must revoke sessions, discard timers, and remove all account-owned data.
type AccountDeletionRepository interface {
	DeleteAccount(context.Context, AccountDeletionCommand) error
	DeletionReceipt(context.Context, string, []byte, time.Time) (bool, error)
}

func (a App) DeleteAccount(ctx context.Context, authorization string, request AccountDeletionRequest) error {
	if a.AccountDeletion == nil || a.DeletionJournal == nil || a.Clock == nil || a.Auth == nil || a.Users == nil {
		return ports.ErrUnavailable
	}
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return err
	}
	hash, valid := deletionReceiptHash(request.ReceiptSecret)
	if !request.Confirmed || request.ReviewedUserID == "" || !valid {
		return ports.ErrInvalidArgument
	}
	if request.ReviewedUserID != user.ID {
		return ports.ErrConflict
	}
	deletedAt := a.Clock.Now().UTC().Truncate(time.Microsecond)
	event := a.auditEvent(ctx, user.ID, user.ID, audit.ResourceDeleted, "account", user.ID, audit.Succeeded)
	event.OccurredAt = deletedAt
	record, err := a.DeletionJournal.Admit(ctx, DeletionRecord{UserID: user.ID, DeletedAt: deletedAt, AuditEventID: event.ID, ReceiptHash: hex.EncodeToString(hash)})
	if err != nil {
		return err
	}
	if record.UserID != user.ID || record.ReceiptHash != hex.EncodeToString(hash) || record.DeletedAt.IsZero() || record.AuditEventID == "" {
		return ports.ErrUnavailable
	}
	deletedAt = record.DeletedAt
	event.ID = record.AuditEventID
	event.OccurredAt = deletedAt
	if err := a.AccountDeletion.DeleteAccount(ctx, AccountDeletionCommand{UserID: user.ID, DeletedAt: deletedAt, Audit: event, NewID: newID, ReceiptHash: hash}); err != nil {
		return err
	}
	a.probe(ctx, "identity.account_deleted", "succeeded")
	return nil
}

func deletionReceiptHash(secret string) ([]byte, bool) {
	if len(secret) != 64 || strings.ToLower(secret) != secret {
		return nil, false
	}
	value, err := hex.DecodeString(secret)
	if err != nil || len(value) != 32 {
		return nil, false
	}
	hash := sha256.Sum256(value)
	return hash[:], true
}

// ConfirmAccountDeletion accepts only the deletion-specific capability. It
// never authenticates a session or authorizes access to remaining account data.
func (a App) ConfirmAccountDeletion(ctx context.Context, userID, secret string) error {
	if a.AccountDeletion == nil || a.Clock == nil || a.Audits == nil {
		return ports.ErrUnavailable
	}
	hash, valid := deletionReceiptHash(secret)
	if !valid || userID == "" || strings.TrimSpace(userID) != userID || len(userID) > 128 {
		return ErrUnauthenticated
	}
	now := a.Clock.Now().UTC()
	found, err := a.AccountDeletion.DeletionReceipt(ctx, userID, hash, now)
	if err != nil {
		return err
	}
	if !found {
		return ErrUnauthenticated
	}
	if a.AuditRateLimiter == nil || !a.AuditRateLimiter.Allow(userID, now) {
		return ErrRateLimited
	}
	return a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, userID, userID, audit.ResourceViewed, "account_deletion", userID, audit.Succeeded))
}
