package dbmigrations_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
)

func TestHomePreferenceMigrationScopesPersonalStateAndPreservesReplay(t *testing.T) {
	contents, err := fs.ReadFile(dbmigrations.Files, "000062_home_preferences.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(contents)
	for _, required := range []string{
		"FOREIGN KEY (path_id, user_id)",
		"REFERENCES public.path_membership_models(path_id, user_id) ON DELETE CASCADE",
		"GRANT SELECT, INSERT ON public.home_preference_mutation_models TO app;",
		"GRANT UPDATE (home_order_method, home_order_revision, home_order_updated_at)",
		"recorded_activity_models(participant_id, path_id, ended_at DESC, id)",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(migration, "GRANT UPDATE ON public.user_preference_models") {
		t.Fatal("migration grants broad profile preference updates")
	}
}

func TestHomePreferenceDownMigrationRefusesToDiscardReplay(t *testing.T) {
	contents, err := fs.ReadFile(dbmigrations.Files, "000062_home_preferences.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(contents)
	if strings.Index(migration, "IF EXISTS (SELECT 1 FROM public.home_preference_mutation_models)") > strings.Index(migration, "DROP TABLE public.home_preference_mutation_models") {
		t.Fatal("down migration must preflight replay evidence")
	}
}
