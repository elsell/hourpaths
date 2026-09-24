package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestPathInvitationCancellationAuditMigrationPreservesEvidenceOnRollback(t *testing.T) {
	if LatestVersion < 65 {
		t.Fatalf("LatestVersion=%d", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000065_path_invitation_cancellation_audit.up.sql")
	if err != nil || !strings.Contains(string(up), "'path_invitation.canceled'") {
		t.Fatalf("up migration error=%v", err)
	}
	down, err := fs.ReadFile(Files, "000065_path_invitation_cancellation_audit.down.sql")
	if err != nil || !strings.Contains(string(down), "cannot remove path invitation cancellation audit taxonomy while evidence exists") {
		t.Fatalf("down migration error=%v", err)
	}
	if strings.Contains(strings.ToUpper(string(down)), "DELETE FROM") || strings.Contains(strings.ToUpper(string(down)), "TRUNCATE") {
		t.Fatal("rollback destroys evidence")
	}
}
