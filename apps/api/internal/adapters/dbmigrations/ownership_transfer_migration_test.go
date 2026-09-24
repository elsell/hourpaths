package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestOwnershipTransferMigrationAddsDurableSinglePendingLifecycle(t *testing.T) {
	if LatestVersion < 36 {
		t.Fatalf("latest migration version = %d, want ownership transfer migration 36 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000036_path_ownership_transfers.up.sql")
	if err != nil {
		t.Fatalf("ownership transfer migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"CREATE TABLE public.path_ownership_transfer_models",
		"path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE",
		"initiator_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE RESTRICT",
		"recipient_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE RESTRICT",
		"expires_at timestamptz NOT NULL",
		"num_nonnulls(accepted_at, declined_at, canceled_at, expired_at) <= 1",
		"expires_at > created_at",
		"CREATE UNIQUE INDEX path_ownership_transfer_models_one_pending_idx",
		"accepted_at IS NULL AND declined_at IS NULL AND canceled_at IS NULL AND expired_at IS NULL",
		"GRANT SELECT, INSERT, UPDATE ON public.path_ownership_transfer_models TO app",
		"COMMIT;",
	})
	if strings.Contains(migration, "GRANT ALL ON public.path_ownership_transfer_models") ||
		strings.Contains(migration, "GRANT DELETE ON public.path_ownership_transfer_models") {
		t.Fatal("ownership transfer migration overgrants mutable access")
	}
}

func TestOwnershipTransferDownMigrationRefusesToDestroyEvidence(t *testing.T) {
	down, err := fs.ReadFile(Files, "000036_path_ownership_transfers.down.sql")
	if err != nil {
		t.Fatalf("ownership transfer rollback is not embedded: %v", err)
	}
	migration := string(down)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"DO $$",
		"FROM public.path_ownership_transfer_models",
		"RAISE EXCEPTION 'cannot remove Path ownership transfer state while evidence exists';",
		"DROP TABLE public.path_ownership_transfer_models",
		"COMMIT;",
	})
	if strings.Contains(strings.ToUpper(migration), "DELETE FROM") ||
		strings.Contains(strings.ToUpper(migration), "TRUNCATE") {
		t.Fatal("ownership transfer rollback must not destroy evidence")
	}
}
