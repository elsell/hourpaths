package dbmigrations

import (
	"os"
	"strings"
	"testing"
)

func TestPathMemberRoleChangeMigrationIsAtomicLeastPrivilegeAndReversible(t *testing.T) {
	up, err := os.ReadFile("000057_path_member_role_changes.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("000057_path_member_role_changes.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"'path_member_removed'", "'path_member_role_changed'", "presentation_class = 'informational'", "offered_role IN ('participant', 'supporter')", "DROP CONSTRAINT notification_models_offered_role_subject_check", "kind IN ('path_member_removed', 'path_member_role_changed')",
		"CREATE FUNCTION public.change_path_member_role", "SECURITY DEFINER", "SET search_path = pg_catalog, public", "path.archived_at IS NULL",
		"membership.role = expected_member_role", "membership.role IN ('participant', 'supporter')", "manager_membership.role = 'administrator'",
		"DELETE FROM public.running_timer_models", "DELETE FROM public.activity_mutation_models", "DELETE FROM public.social_goal_achievement_models", "DELETE FROM public.recorded_activity_models",
		"UPDATE public.path_membership_models", "SET role = requested_member_role", "REVOKE ALL ON FUNCTION", "GRANT EXECUTE ON FUNCTION",
	} {
		if !strings.Contains(string(up), required) {
			t.Errorf("up migration missing %q", required)
		}
	}
	if !strings.Contains(string(down), "DROP FUNCTION IF EXISTS public.change_path_member_role") {
		t.Error("down migration does not drop role-change function")
	}
	if LatestVersion < 57 {
		t.Errorf("LatestVersion=%d want at least 57", LatestVersion)
	}
}
