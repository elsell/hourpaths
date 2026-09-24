package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestPathInvitationRejectionAuditMigrationExtendsTaxonomyAndProtectsRollback(t *testing.T) {
	if LatestVersion < 34 {
		t.Fatalf("latest migration version = %d, want rejection audit migration 34 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000034_path_invitation_rejection_audit.up.sql")
	if err != nil {
		t.Fatalf("rejection audit migration is not embedded: %v", err)
	}
	if !strings.Contains(string(up), "'path_invitation.rejected'") {
		t.Fatal("rejection audit migration does not add the action")
	}
	down, err := fs.ReadFile(Files, "000034_path_invitation_rejection_audit.down.sql")
	if err != nil {
		t.Fatalf("rejection audit rollback is not embedded: %v", err)
	}
	rollback := string(down)
	assertMigrationStatementsInOrder(t, rollback, []string{
		"BEGIN;", "IF EXISTS", "action = 'path_invitation.rejected'",
		"RAISE EXCEPTION 'cannot remove path invitation rejection audit taxonomy while evidence exists';",
		"DROP CONSTRAINT audit_event_models_action_check", "COMMIT;",
	})
	if strings.Contains(strings.ToUpper(rollback), "DELETE FROM") || strings.Contains(strings.ToUpper(rollback), "TRUNCATE") {
		t.Fatal("rejection audit rollback must not destroy evidence")
	}
}
