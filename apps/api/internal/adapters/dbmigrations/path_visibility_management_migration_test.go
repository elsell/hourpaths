package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestPathVisibilityMigrationPreservesEvidenceAndLeastPrivilege(t *testing.T) {
	content, err := fs.ReadFile(Files, "000066_path_visibility_management.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := string(content)
	for _, required := range []string{"path.visibility_changed", "path_visibility_changed", "path_visibility", "path_visibility_replay_models", "GRANT SELECT, INSERT ON public.path_visibility_replay_models TO app", "path.visibility.set"} {
		if !strings.Contains(up, required) {
			t.Fatalf("visibility migration missing %q", required)
		}
	}
	content, err = fs.ReadFile(Files, "000066_path_visibility_management.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	down := string(content)
	if !strings.Contains(down, "cannot roll back Path visibility management while evidence exists") || !strings.Contains(down, "DROP TABLE public.path_visibility_replay_models") {
		t.Fatal("visibility down migration can discard evidence or replay state")
	}
}
