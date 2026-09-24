package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestActivityDeletionMigrationAddsTombstonedReplayAndNarrowDeletePrivilege(t *testing.T) {
	if LatestVersion < 30 {
		t.Fatalf("latest migration version = %d, want activity deletion migration 30 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000030_delete_owned_activity.up.sql")
	if err != nil {
		t.Fatalf("activity deletion migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"ALTER TABLE public.activity_mutation_models",
		"DROP CONSTRAINT activity_mutation_models_operation_check",
		"DROP CONSTRAINT activity_mutation_models_operation_state_check",
		"DROP CONSTRAINT activity_mutation_models_saved_result_check",
		"ADD COLUMN result_activity_deleted boolean NOT NULL DEFAULT false",
		"ADD COLUMN result_accumulated_seconds bigint",
		"'activity.delete'",
		"operation IN ('activity.manual.create', 'activity.update')",
		"OR (NOT result_activity_saved AND result_activity_deleted)",
		"operation = 'activity.delete'",
		"result_accumulated_seconds IS NOT NULL",
		"ADD CONSTRAINT activity_mutation_models_accumulated_seconds_check",
		"GRANT SELECT, INSERT ON public.recorded_activity_models TO app;",
		"GRANT UPDATE (started_at, ended_at, occurrence_time_zone, note, updated_at)",
		"GRANT DELETE ON public.recorded_activity_models TO app;",
		"COMMIT;",
	})
	for _, forbidden := range []string{
		"GRANT ALL",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON public.recorded_activity_models TO app;",
		"GRANT DELETE ON public.recorded_activity_revision_models",
		"ALTER TABLE audit_event_models",
	} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("activity deletion migration contains forbidden expansion %q", forbidden)
		}
	}
}

func TestActivityDeletionDownMigrationFailsClosedAndRestoresManualActivityBoundary(t *testing.T) {
	down, err := fs.ReadFile(Files, "000030_delete_owned_activity.down.sql")
	if err != nil {
		t.Fatalf("activity deletion down migration is not embedded: %v", err)
	}
	migration := string(down)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"DO $$",
		"WHERE operation = 'activity.delete'",
		"OR result_activity_deleted",
		"OR result_accumulated_seconds IS NOT NULL",
		"RAISE EXCEPTION 'cannot remove activity deletion replay state while deletion evidence exists';",
		"REVOKE ALL ON public.recorded_activity_models FROM app;",
		"DROP CONSTRAINT activity_mutation_models_accumulated_seconds_check",
		"DROP COLUMN result_accumulated_seconds",
		"DROP COLUMN result_activity_deleted",
		"operation IN (",
		"'activity.manual.create'",
		"'activity.update'",
		"GRANT SELECT, INSERT ON public.recorded_activity_models TO app;",
		"GRANT UPDATE (started_at, ended_at, occurrence_time_zone, note, updated_at)",
		"COMMIT;",
	})
	if strings.Contains(strings.ToUpper(migration), "DELETE FROM PUBLIC.AUDIT_EVENT_MODELS") || strings.Contains(strings.ToUpper(migration), "TRUNCATE") {
		t.Fatal("activity deletion rollback must retain immutable audit history")
	}
}
