package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestOwnershipTransferAuthorizationBatchMigrationIsDurableLeastPrivilegeAndFailClosed(t *testing.T) {
	if LatestVersion < 39 {
		t.Fatalf("latest migration version = %d, want authorization batch migration 39 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000039_ownership_transfer_authorization_batches.up.sql")
	if err != nil {
		t.Fatalf("authorization batch migration is not embedded: %v", err)
	}
	for _, statement := range []string{
		"CREATE TABLE public.authorization_batch_outbox_models",
		"path_ownership_transfer_id text NOT NULL UNIQUE",
		"jsonb_array_length(relationship_updates) = 4",
		"CREATE TRIGGER authorization_batch_outbox_ordering",
		"SELECT created_at FROM public.authorization_outbox_models",
		"UNION ALL",
		"CREATE INDEX authorization_batch_outbox_claim_idx",
		"GRANT SELECT, INSERT, UPDATE ON public.authorization_batch_outbox_models TO app;",
	} {
		if !strings.Contains(string(up), statement) {
			t.Fatalf("authorization batch migration is missing %q", statement)
		}
	}
	if strings.Contains(string(up), "GRANT ALL") || strings.Contains(string(up), "GRANT DELETE") {
		t.Fatal("authorization batch migration overgrants runtime access")
	}

	down, err := fs.ReadFile(Files, "000039_ownership_transfer_authorization_batches.down.sql")
	if err != nil {
		t.Fatalf("authorization batch rollback is not embedded: %v", err)
	}
	if !strings.Contains(string(down), "RAISE EXCEPTION 'cannot remove ownership transfer authorization batches while evidence exists';") {
		t.Fatal("authorization batch rollback must refuse to destroy delivery evidence")
	}
}
