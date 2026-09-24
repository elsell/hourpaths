package activitystore

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
)

func TestActivityMigrationPreservesCanonicalAndRuntimeBoundaries(t *testing.T) {
	contents, err := fs.ReadFile(dbmigrations.Files, "000027_create_activities.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(contents)
	for _, required := range []string{
		"ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;",
		"'activity.timer_started'",
		"'activity.timer_stopped'",
		"UNIQUE (participant_id, path_id)",
		"path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE",
		"CHECK (ended_at >= started_at + interval '1 second')",
		"REVOKE ALL ON public.running_timer_models FROM app;",
		"GRANT SELECT, INSERT, DELETE ON public.running_timer_models TO app;",
		"REVOKE ALL ON public.recorded_activity_models FROM app;",
		"GRANT SELECT, INSERT ON public.recorded_activity_models TO app;",
		"REVOKE ALL ON public.activity_mutation_models FROM app;",
		"GRANT SELECT, INSERT, UPDATE ON public.activity_mutation_models TO app;",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("activity migration is missing %q", required)
		}
	}
	if strings.Count(migration, "'activity.timer_started'") != 1 || strings.Count(migration, "'activity.timer_stopped'") != 1 {
		t.Fatal("activity migration must admit each timer audit action exactly once")
	}
	prior, err := fs.ReadFile(dbmigrations.Files, "000023_atomic_account_activation.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	priorConstraint := strings.Split(strings.TrimSpace(string(prior)), "\n")[3]
	if !strings.Contains(migration, strings.TrimSuffix(priorConstraint, "));")+", 'activity.timer_started', 'activity.timer_stopped'));") {
		t.Fatal("activity migration did not preserve and expand the exact prior audit action constraint")
	}
	if strings.Contains(migration, "duration") {
		t.Fatal("activity migration persists duration independently")
	}
	if count := strings.Count(migration, "path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE"); count != 3 {
		t.Fatalf("Path cascade constraints = %d, want timers, activities, and mutation replay state", count)
	}
	for _, forbidden := range []string{
		"GRANT SELECT, INSERT, UPDATE ON public.recorded_activity_models TO app;",
		"GRANT SELECT, INSERT, DELETE ON public.recorded_activity_models TO app;",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON public.recorded_activity_models TO app;",
	} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("activity migration grants premature mutation capability: %q", forbidden)
		}
	}
}

func TestActivityDownMigrationProtectsDurableStateAndImmutableAuditHistory(t *testing.T) {
	down, err := fs.ReadFile(dbmigrations.Files, "000027_create_activities.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(down)
	preflight := "WHERE action IN ('activity.timer_started', 'activity.timer_stopped')"
	constraintDrop := "ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;"
	if preflightAt, dropAt := strings.Index(migration, preflight), strings.Index(migration, constraintDrop); preflightAt < 0 || dropAt < 0 || preflightAt > dropAt {
		t.Fatal("activity down migration must reject timer audit history before narrowing its action constraint")
	}
	firstDestructive := strings.Index(migration, "REVOKE ALL ON public.activity_mutation_models FROM app;")
	for _, table := range []string{"running_timer_models", "recorded_activity_models", "activity_mutation_models"} {
		preflightAt := strings.Index(migration, "SELECT 1 FROM public."+table)
		if preflightAt < 0 || firstDestructive < 0 || preflightAt > firstDestructive {
			t.Fatalf("activity down migration must preflight durable rows in %s before destructive statements", table)
		}
	}
	if strings.Contains(strings.ToUpper(migration), "DELETE FROM PUBLIC.AUDIT_EVENT_MODELS") || strings.Contains(strings.ToUpper(migration), "TRUNCATE") {
		t.Fatal("activity down migration must never remove immutable audit history")
	}
	if strings.Count(migration, "'activity.timer_started'") != 1 || strings.Count(migration, "'activity.timer_stopped'") != 1 {
		t.Fatal("activity down migration may reference removed audit actions only in its rollback preflight")
	}
	prior, err := fs.ReadFile(dbmigrations.Files, "000023_atomic_account_activation.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	priorConstraint := strings.Split(strings.TrimSpace(string(prior)), "\n")[3]
	if !strings.Contains(migration, priorConstraint) {
		t.Fatal("activity down migration did not restore the exact prior audit action constraint")
	}
}
