package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestManualActivityMigrationAddsPrivateNotesImmutableRevisionsAndReplayState(t *testing.T) {
	if LatestVersion < 29 {
		t.Fatalf("latest migration version = %d, want at least manual activity migration 29", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000029_manual_activity_revisions.up.sql")
	if err != nil {
		t.Fatalf("manual activity migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"ALTER TABLE public.recorded_activity_models ADD COLUMN note text;",
		"ADD CONSTRAINT recorded_activity_models_note_check",
		"CREATE INDEX recorded_activity_models_path_created_idx\n  ON public.recorded_activity_models(path_id, created_at DESC, id DESC);",
		"CREATE TABLE public.recorded_activity_revision_models",
		"activity_id text NOT NULL REFERENCES public.recorded_activity_models(id) ON DELETE CASCADE",
		"version bigint NOT NULL CHECK (version > 0)",
		"started_at timestamptz NOT NULL",
		"ended_at timestamptz NOT NULL",
		"occurrence_time_zone text NOT NULL",
		"\n  note text,\n  public_changed boolean NOT NULL,\n  updated_at timestamptz NOT NULL",
		"replaced_at timestamptz NOT NULL",
		"PRIMARY KEY (activity_id, version)",
		"CREATE INDEX recorded_activity_revision_models_activity_replaced_idx\n  ON public.recorded_activity_revision_models(activity_id, replaced_at, version);",
		"ALTER TABLE public.activity_mutation_models\n  DROP CONSTRAINT activity_mutation_models_operation_check,",
		"DROP CONSTRAINT activity_mutation_models_check,",
		"DROP CONSTRAINT activity_mutation_models_check1;",
		"ALTER TABLE public.activity_mutation_models ALTER COLUMN timer_id DROP NOT NULL;",
		"ADD COLUMN result_note text",
		"ADD COLUMN result_version bigint",
		"'activity.manual.create'",
		"'activity.update'",
		"operation = 'activity.manual.create'\n      AND timer_id IS NULL\n      AND result_activity_saved\n      AND result_version = 1",
		"operation = 'activity.update'\n      AND timer_id IS NULL\n      AND result_activity_saved\n      AND result_version > 1",
		"ADD CONSTRAINT activity_mutation_models_saved_result_check",
		"ADD CONSTRAINT activity_mutation_models_result_note_check",
		"REVOKE ALL ON public.recorded_activity_models FROM app;",
		"GRANT SELECT, INSERT ON public.recorded_activity_models TO app;",
		"GRANT UPDATE (started_at, ended_at, occurrence_time_zone, note, updated_at)",
		"REVOKE ALL ON public.recorded_activity_revision_models FROM app;",
		"GRANT SELECT, INSERT ON public.recorded_activity_revision_models TO app;",
		"REVOKE ALL ON public.activity_mutation_models FROM app;",
		"GRANT SELECT, INSERT, UPDATE ON public.activity_mutation_models TO app;",
		"COMMIT;",
	})
	for _, forbidden := range []string{
		"GRANT ALL",
		"GRANT SELECT, INSERT, UPDATE ON public.recorded_activity_models TO app;",
		"GRANT UPDATE ON public.recorded_activity_revision_models",
		"GRANT DELETE ON public.recorded_activity_revision_models",
		"ALTER TABLE audit_event_models",
		"activity.manual_created",
		"activity.manual_updated",
	} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("manual activity migration contains forbidden expansion %q", forbidden)
		}
	}
}

func TestManualActivityDownMigrationFailsClosedAndRestoresPriorBoundary(t *testing.T) {
	down, err := fs.ReadFile(Files, "000029_manual_activity_revisions.down.sql")
	if err != nil {
		t.Fatalf("manual activity down migration is not embedded: %v", err)
	}
	migration := string(down)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"DO $$",
		"SELECT 1 FROM public.recorded_activity_revision_models",
		"WHERE note IS NOT NULL",
		"WHERE operation IN ('activity.manual.create', 'activity.update')",
		"RAISE EXCEPTION 'cannot remove manual activity persistence while note, revision, create, or update state exists';",
		"REVOKE ALL ON public.recorded_activity_revision_models FROM app;",
		"DROP TABLE public.recorded_activity_revision_models;",
		"ALTER TABLE public.activity_mutation_models\n  DROP CONSTRAINT activity_mutation_models_operation_check,",
		"ALTER TABLE public.activity_mutation_models DROP COLUMN result_version;",
		"ALTER TABLE public.activity_mutation_models DROP COLUMN result_note;",
		"ALTER TABLE public.activity_mutation_models ALTER COLUMN timer_id SET NOT NULL;",
		"ADD CONSTRAINT activity_mutation_models_operation_check CHECK (\n    operation IN ('activity.timer.start', 'activity.timer.stop')",
		"ADD CONSTRAINT activity_mutation_models_check CHECK",
		"ADD CONSTRAINT activity_mutation_models_check1 CHECK",
		"ALTER TABLE public.recorded_activity_models DROP CONSTRAINT recorded_activity_models_note_check;",
		"DROP INDEX public.recorded_activity_models_path_created_idx;",
		"ALTER TABLE public.recorded_activity_models DROP COLUMN note;",
		"REVOKE ALL ON public.recorded_activity_models FROM app;",
		"GRANT SELECT, INSERT ON public.recorded_activity_models TO app;",
		"COMMIT;",
	})
	firstDestructive := strings.Index(migration, "REVOKE ALL ON public.recorded_activity_revision_models FROM app;")
	for _, preflight := range []string{
		"SELECT 1 FROM public.recorded_activity_revision_models",
		"WHERE note IS NOT NULL",
		"WHERE operation IN ('activity.manual.create', 'activity.update')",
	} {
		if at := strings.Index(migration, preflight); at < 0 || firstDestructive < 0 || at > firstDestructive {
			t.Fatalf("rollback preflight %q must precede destructive statements", preflight)
		}
	}
	upper := strings.ToUpper(migration)
	if strings.Contains(upper, "DELETE FROM PUBLIC.AUDIT_EVENT_MODELS") || strings.Contains(upper, "TRUNCATE") || strings.Contains(migration, "ALTER TABLE audit_event_models") {
		t.Fatal("manual activity rollback must not mutate immutable audit history or its action constraint")
	}
	for _, forbidden := range []string{
		"GRANT UPDATE ON public.recorded_activity_revision_models",
		"GRANT DELETE ON public.recorded_activity_revision_models",
		"GRANT SELECT, INSERT, UPDATE ON public.recorded_activity_models TO app;",
	} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("manual activity rollback retained expanded privilege %q", forbidden)
		}
	}
}
