package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestHomePreferenceOwnershipMigrationReplacesMembershipConstraint(t *testing.T) {
	if LatestVersion < 63 {
		t.Fatalf("latest version=%d want Home preference ownership migration", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000063_home_preference_ownership.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(up)
	for _, required := range []string{
		"DROP CONSTRAINT home_path_preference_models_path_id_user_id_fkey",
		"FOREIGN KEY (path_id) REFERENCES public.path_models(id) ON DELETE CASCADE",
		"FOREIGN KEY (user_id) REFERENCES public.user_models(id) ON DELETE CASCADE",
		"DELETE FROM public.home_path_preference_models",
		"WHERE path_id = OLD.path_id AND user_id = OLD.user_id",
		"AFTER DELETE ON public.path_membership_models",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	if strings.Contains(migration, "REFERENCES public.path_membership_models") {
		t.Fatal("up migration retained membership-dependent Home preferences")
	}
}

func TestHomePreferenceOwnershipDownMigrationPreservesOwnerPreferences(t *testing.T) {
	down, err := fs.ReadFile(Files, "000063_home_preference_ownership.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(down)
	preflight := strings.Index(migration, "RAISE EXCEPTION 'cannot roll back Home preference ownership while preferences without membership exist'")
	drop := strings.Index(migration, "DROP CONSTRAINT home_path_preference_models_path_id_fkey")
	if preflight < 0 || drop < 0 || preflight > drop {
		t.Fatal("down migration must reject owner preference data before replacing constraints")
	}
	if !strings.Contains(migration, "FOREIGN KEY (path_id, user_id)") || !strings.Contains(migration, "REFERENCES public.path_membership_models(path_id, user_id) ON DELETE CASCADE") {
		t.Fatal("down migration does not restore the prior composite constraint")
	}
}
