package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestOwnershipTransferNotificationMigrationGeneralizesPathAccessEvents(t *testing.T) {
	if LatestVersion < 38 {
		t.Fatalf("latest migration version = %d, want ownership transfer notification migration 38 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000038_path_ownership_transfer_notifications.up.sql")
	if err != nil {
		t.Fatalf("ownership transfer notification migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"ALTER COLUMN path_invitation_id DROP NOT NULL",
		"ADD COLUMN path_ownership_transfer_id text REFERENCES public.path_ownership_transfer_models(id) ON DELETE CASCADE",
		"ALTER COLUMN offered_role DROP NOT NULL",
		"path_ownership_transfer_received", "path_ownership_transfer_accepted",
		"path_ownership_transfer_declined", "path_ownership_transfer_canceled",
		"num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1",
		"(path_invitation_id IS NULL) = (offered_role IS NULL)",
		"CREATE UNIQUE INDEX notification_models_transfer_kind_recipient_idx",
		"COMMIT;",
	})
}

func TestOwnershipTransferNotificationDownMigrationRefusesToDestroyEvidence(t *testing.T) {
	down, err := fs.ReadFile(Files, "000038_path_ownership_transfer_notifications.down.sql")
	if err != nil {
		t.Fatalf("ownership transfer notification rollback is not embedded: %v", err)
	}
	migration := string(down)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;", "DO $$", "path_ownership_transfer_id IS NOT NULL",
		"RAISE EXCEPTION 'cannot remove Path ownership transfer notification support while evidence exists';",
		"DROP COLUMN path_ownership_transfer_id", "ALTER COLUMN path_invitation_id SET NOT NULL",
		"ALTER COLUMN offered_role SET NOT NULL", "COMMIT;",
	})
	if strings.Contains(strings.ToUpper(migration), "DELETE FROM") ||
		strings.Contains(strings.ToUpper(migration), "TRUNCATE") {
		t.Fatal("ownership transfer notification rollback must not destroy evidence")
	}
}
