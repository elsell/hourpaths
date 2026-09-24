package dbmigrations

import (
	"strings"
	"testing"
)

func TestSocialBlockingMigrationPersistsReplayAndLeastPrivilegeWrites(t *testing.T) {
	if LatestVersion < 53 {
		t.Fatalf("latest version=%d want blocking migration 53 or later", LatestVersion)
	}
	up, err := Files.ReadFile("000053_social_blocking.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(up)
	for _, required := range []string{
		"CREATE TABLE public.social_block_replay_models",
		"operation IN ('social.block', 'social.unblock')",
		"request_hash bytea NOT NULL",
		"target_username text NOT NULL",
		"target_display_name text NOT NULL",
		"authorization_change_ids text[] NOT NULL",
		"GRANT SELECT, INSERT, DELETE ON public.block_models TO app",
		"GRANT SELECT, INSERT ON public.social_block_replay_models TO app",
		"CREATE INDEX block_models_blocker_page_idx",
	} {
		if !strings.Contains(contents, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	down, err := Files.ReadFile("000053_social_blocking.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "IF EXISTS (SELECT 1 FROM public.social_block_replay_models)") ||
		!strings.Contains(string(down), "REVOKE INSERT, DELETE ON public.block_models FROM app") {
		t.Fatal("down migration does not fail closed before removing blocking support")
	}
}
