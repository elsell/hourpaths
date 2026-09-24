package dbmigrations

import (
	"strings"
	"testing"
)

func TestSocialDiscoveryMigrationPersistsOnlyExplicitRelationshipsAndSafeProfileFields(t *testing.T) {
	if LatestVersion < 43 {
		t.Fatalf("latest version=%d want social discovery migration", LatestVersion)
	}
	up, err := Files.ReadFile("000043_social_profile_discovery.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(up)
	for _, required := range []string{"description", "profile_picture_url", "CREATE TABLE public.follow_models", "CREATE TABLE public.block_models", "CHECK (follower_user_id <> following_user_id)", "CHECK (blocker_user_id <> blocked_user_id)", "GRANT SELECT ON public.follow_models TO app", "GRANT SELECT ON public.block_models TO app"} {
		if !strings.Contains(contents, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(contents, "GRANT INSERT") || strings.Contains(contents, "GRANT UPDATE") || strings.Contains(contents, "GRANT DELETE") {
		t.Fatal("read-only discovery slice granted relationship mutation privileges")
	}
}

func TestSocialDiscoveryRollbackRefusesToDestroyProfileOrRelationshipData(t *testing.T) {
	down, err := Files.ReadFile("000043_social_profile_discovery.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(down)
	for _, required := range []string{
		"IF EXISTS (SELECT 1 FROM public.follow_models)",
		"OR EXISTS (SELECT 1 FROM public.block_models)",
		"WHERE description IS NOT NULL OR profile_picture_url IS NOT NULL",
		"RAISE EXCEPTION 'cannot remove social profile discovery persistence while profile or relationship data exists'",
	} {
		if !strings.Contains(contents, required) {
			t.Fatalf("rollback missing fail-closed preflight %q", required)
		}
	}
	if strings.Index(contents, "RAISE EXCEPTION") > strings.Index(contents, "DROP TABLE public.block_models") {
		t.Fatal("rollback destroys relationship data before its fail-closed preflight")
	}
}
