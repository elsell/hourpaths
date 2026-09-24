package dbmigrations

import (
	"os"
	"strings"
	"testing"
)

func TestLeavePathActivityChoiceMigrationIsScopedAndReversible(t *testing.T) {
	up, err := os.ReadFile("000059_leave_path_activity_choice.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("000059_leave_path_activity_choice.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(up)
	for _, required := range []string{
		"SECURITY DEFINER", "SET search_path = pg_catalog, public",
		"path.archived_at IS NULL", "member_user_identifier <> path.owner_user_id",
		"membership.role IN ('administrator', 'participant', 'supporter')", "FOR UPDATE",
		"DELETE FROM public.running_timer_models", "DELETE FROM public.activity_mutation_models",
		"DELETE FROM public.social_goal_achievement_models", "DELETE FROM public.recorded_activity_models",
		"DELETE FROM public.social_practice_comment_replay_models", "DELETE FROM public.social_practice_comment_heart_replay_models",
		"REVOKE EXECUTE ON FUNCTION public.leave_path_membership(text, text) FROM app;",
		"DROP FUNCTION public.leave_path_membership(text, text);",
		"REVOKE ALL ON FUNCTION public.leave_path_membership(text, text, boolean) FROM PUBLIC;",
		"GRANT EXECUTE ON FUNCTION public.leave_path_membership(text, text, boolean) TO app;",
	} {
		if !strings.Contains(contents, required) {
			t.Fatalf("leave activity-choice migration missing %q", required)
		}
	}
	if !strings.Contains(string(down), "DROP FUNCTION public.leave_path_membership(text, text, boolean);") {
		t.Fatal("rollback does not remove the exact overload")
	}
	for _, required := range []string{
		"CREATE FUNCTION public.leave_path_membership(path_identifier text, member_user_identifier text)",
		"REVOKE ALL ON FUNCTION public.leave_path_membership(text, text) FROM PUBLIC;",
		"GRANT EXECUTE ON FUNCTION public.leave_path_membership(text, text) TO app;",
	} {
		if !strings.Contains(string(down), required) {
			t.Fatalf("rollback does not restore the prior function: missing %q", required)
		}
	}
	if LatestVersion < 59 {
		t.Fatalf("LatestVersion=%d want at least 59", LatestVersion)
	}
}
