package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestOwnershipTransferAuditMigrationRetainsEveryLifecycleAction(t *testing.T) {
	if LatestVersion < 37 {
		t.Fatalf("latest migration version = %d, want ownership transfer audit migration 37 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000037_path_ownership_transfer_audit.up.sql")
	if err != nil {
		t.Fatalf("ownership transfer audit migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;", "DROP CONSTRAINT audit_event_models_action_check",
		"path_ownership_transfer.created", "path_ownership_transfer.listed",
		"path_ownership_transfer.accepted", "path_ownership_transfer.declined",
		"path_ownership_transfer.canceled", "COMMIT;",
	})
}

func TestOwnershipTransferAuditDownMigrationRefusesToOrphanEvidence(t *testing.T) {
	down, err := fs.ReadFile(Files, "000037_path_ownership_transfer_audit.down.sql")
	if err != nil {
		t.Fatalf("ownership transfer audit rollback is not embedded: %v", err)
	}
	migration := string(down)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;", "DO $$", "FROM public.audit_event_models",
		"action LIKE 'path_ownership_transfer.%'",
		"RAISE EXCEPTION 'cannot remove Path ownership transfer audit taxonomy while evidence exists';",
		"DROP CONSTRAINT audit_event_models_action_check", "COMMIT;",
	})
	if strings.Contains(strings.ToUpper(migration), "DELETE FROM") ||
		strings.Contains(strings.ToUpper(migration), "TRUNCATE") {
		t.Fatal("ownership transfer audit rollback must not destroy evidence")
	}
}
