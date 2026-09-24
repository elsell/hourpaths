package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestPathInvitationRejectionReplayMigrationPersistsOriginalUnreadCount(t *testing.T) {
	if LatestVersion < 35 {
		t.Fatalf("latest migration version = %d, want rejection replay migration 35 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000035_path_invitation_rejection_replay.up.sql")
	if err != nil {
		t.Fatalf("rejection replay migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(up), []string{
		"BEGIN;", "ADD COLUMN rejection_unread_count bigint",
		"SET rejection_unread_count = 0", "WHERE rejected_at IS NOT NULL",
		"(rejected_at IS NULL) = (rejection_unread_count IS NULL)",
		"rejection_unread_count >= 0", "COMMIT;",
	})
	down, err := fs.ReadFile(Files, "000035_path_invitation_rejection_replay.down.sql")
	if err != nil {
		t.Fatalf("rejection replay rollback is not embedded: %v", err)
	}
	rollback := string(down)
	assertMigrationStatementsInOrder(t, rollback, []string{
		"BEGIN;", "IF EXISTS", "WHERE rejection_unread_count IS NOT NULL",
		"RAISE EXCEPTION 'cannot remove Path invitation rejection replay results while evidence exists';",
		"DROP CONSTRAINT path_invitation_models_rejection_result_check",
		"DROP COLUMN rejection_unread_count", "COMMIT;",
	})
	if strings.Contains(strings.ToUpper(rollback), "DELETE FROM") || strings.Contains(strings.ToUpper(rollback), "TRUNCATE") {
		t.Fatal("rejection replay rollback must not destroy evidence")
	}
}
