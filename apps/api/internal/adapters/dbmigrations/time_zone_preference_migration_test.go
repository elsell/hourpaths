package dbmigrations_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
)

func TestTimeZonePreferenceMigrationNarrowsMutationAuthorityAndPersistsReplay(t *testing.T) {
	contents, err := fs.ReadFile(dbmigrations.Files, "000060_time_zone_preferences.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(contents)
	for _, required := range []string{
		"CREATE TABLE public.user_time_zone_preference_mutation_models",
		"PRIMARY KEY (user_id, operation, idempotency_key)",
		"GRANT SELECT, INSERT ON public.user_time_zone_preference_mutation_models TO app;",
		"GRANT UPDATE (current_time_zone, updated_at) ON public.user_preference_models TO app;",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("migration is missing %q", required)
		}
	}
	if strings.Contains(migration, "GRANT UPDATE ON public.user_preference_models") {
		t.Fatal("migration granted unrestricted preference updates")
	}
}

func TestTimeZonePreferenceDownMigrationRefusesToDiscardReplayState(t *testing.T) {
	contents, err := fs.ReadFile(dbmigrations.Files, "000060_time_zone_preferences.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(contents)
	preflight := "IF EXISTS (SELECT 1 FROM public.user_time_zone_preference_mutation_models)"
	drop := "DROP TABLE public.user_time_zone_preference_mutation_models;"
	if strings.Index(migration, preflight) < 0 || strings.Index(migration, preflight) > strings.Index(migration, drop) {
		t.Fatal("down migration must preflight durable mutation evidence before dropping it")
	}
	if !strings.Contains(migration, "REVOKE UPDATE (current_time_zone, updated_at) ON public.user_preference_models FROM app;") {
		t.Fatal("down migration did not revoke narrow preference mutation authority")
	}
}
