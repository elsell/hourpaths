package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestPathArchivalMigrationAddsNullableLifecycleStateAndScopedIndexes(t *testing.T) {
	if LatestVersion < 31 {
		t.Fatalf("latest migration version = %d, want at least Path archival migration 31", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000031_path_archival.up.sql")
	if err != nil {
		t.Fatalf("Path archival migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"ALTER TABLE public.path_models",
		"ADD COLUMN archived_at timestamptz",
		"ADD CONSTRAINT path_models_archive_time_check",
		"archived_at IS NULL OR archived_at >= created_at",
		"CREATE INDEX path_models_active_page_idx",
		"WHERE archived_at IS NULL",
		"CREATE INDEX path_models_archived_page_idx",
		"WHERE archived_at IS NOT NULL",
		"COMMIT;",
	})
	for _, forbidden := range []string{"ADD COLUMN archived_at timestamptz NOT NULL", "ADD COLUMN archived_at timestamptz DEFAULT", "GRANT ALL"} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("Path archival migration contains forbidden expansion %q", forbidden)
		}
	}
}

func TestPathArchivalDownMigrationFailsClosedWhenArchivedPathsExist(t *testing.T) {
	down, err := fs.ReadFile(Files, "000031_path_archival.down.sql")
	if err != nil {
		t.Fatalf("Path archival down migration is not embedded: %v", err)
	}
	migration := string(down)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"DO $$",
		"WHERE archived_at IS NOT NULL",
		"RAISE EXCEPTION 'cannot remove Path archival state while archived Paths exist';",
		"DROP INDEX public.path_models_archived_page_idx",
		"DROP INDEX public.path_models_active_page_idx",
		"DROP CONSTRAINT path_models_archive_time_check",
		"DROP COLUMN archived_at",
		"COMMIT;",
	})
	if strings.Contains(strings.ToUpper(migration), "DELETE FROM") || strings.Contains(strings.ToUpper(migration), "TRUNCATE") {
		t.Fatal("Path archival rollback must not destroy lifecycle evidence")
	}
}
