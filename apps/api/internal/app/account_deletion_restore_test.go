package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type restoredAccounts struct{ remaining map[string]bool }

func (r *restoredAccounts) ReapplyAccountDeletion(_ context.Context, command AccountDeletionCommand) error {
	delete(r.remaining, command.UserID)
	return nil
}
func TestDeletionRestoreValidatesWholeManifestBeforeRemoval(t *testing.T) {
	records := []DeletionRecord{
		{UserID: "alice", DeletedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), AuditEventID: "audit-alice", ReceiptHash: strings.Repeat("01", 32)},
		{UserID: "bob", DeletedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), AuditEventID: "audit-bob", ReceiptHash: "invalid"},
	}
	accounts := &restoredAccounts{remaining: map[string]bool{"alice": true, "bob": true}}
	if err := ReapplyDeletionRecords(context.Background(), records, accounts, func() string { return "correlation" }); err == nil {
		t.Fatal("invalid manifest accepted")
	}
	if len(accounts.remaining) != 2 {
		t.Fatal("partially applied invalid manifest")
	}
	records[1].ReceiptHash = records[0].ReceiptHash
	if err := ReapplyDeletionRecords(context.Background(), records, accounts, func() string { return "correlation" }); err != nil {
		t.Fatal(err)
	}
	if len(accounts.remaining) != 0 {
		t.Fatal("valid manifest did not remove all accounts")
	}
}

type interruptedDeletions struct {
	restoredAccounts
	fail string
}

func (r *interruptedDeletions) ReapplyAccountDeletion(ctx context.Context, command AccountDeletionCommand) error {
	if command.UserID == r.fail {
		return errors.New("database temporarily unavailable")
	}
	return r.restoredAccounts.ReapplyAccountDeletion(ctx, command)
}
func TestPendingDeletionRecoveryContinuesAndRetries(t *testing.T) {
	records := []DeletionRecord{
		{UserID: "alice", DeletedAt: time.Now(), AuditEventID: "audit-alice", ReceiptHash: strings.Repeat("01", 32)},
		{UserID: "bob", DeletedAt: time.Now(), AuditEventID: "audit-bob", ReceiptHash: strings.Repeat("02", 32)},
	}
	accounts := &interruptedDeletions{restoredAccounts{map[string]bool{"alice": true, "bob": true}}, "alice"}
	if err := RecoverAcceptedDeletions(context.Background(), records, accounts, func() string { return "correlation" }); err == nil {
		t.Fatal("failure hidden")
	}
	if !accounts.remaining["alice"] || accounts.remaining["bob"] {
		t.Fatal("failed account prevented independent recovery")
	}
	accounts.fail = ""
	if err := RecoverAcceptedDeletions(context.Background(), records, accounts, func() string { return "correlation" }); err != nil {
		t.Fatal(err)
	}
	if len(accounts.remaining) != 0 {
		t.Fatal("accepted deletion not retried")
	}
}
