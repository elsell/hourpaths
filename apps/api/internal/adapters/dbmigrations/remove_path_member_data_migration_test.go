package dbmigrations

import (
	"os"
	"strings"
	"testing"
)

func TestRemovePathMemberDataFunctionIsRoleBoundLeastPrivilegeAndReversible(t *testing.T) {
	up, err := os.ReadFile("000056_remove_path_member_data.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("000056_remove_path_member_data.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"ADD COLUMN joined_at timestamptz", "WITH cutover AS", "SELECT statement_timestamp() AS occurred_at", "SELECT max(invitation.accepted_at)", "SELECT transfer.initiator_user_id", "ORDER BY transfer.accepted_at, transfer.created_at, transfer.id", "THEN path.created_at END", "cutover.occurred_at", "SET joined_at = current_incarnation.joined_at", "ALTER COLUMN joined_at SET NOT NULL", "ALTER COLUMN joined_at SET DEFAULT CURRENT_TIMESTAMP",
		"SECURITY DEFINER", "SET search_path = pg_catalog, public", "expected_member_role text",
		"path.archived_at IS NULL", "membership.role = expected_member_role", "membership.role IN ('participant', 'supporter')",
		"IF target_role = 'participant' THEN",
		"DELETE FROM public.running_timer_models", "DELETE FROM public.activity_mutation_models", "DELETE FROM public.social_goal_achievement_models", "DELETE FROM public.recorded_activity_models", "DELETE FROM public.path_membership_models",
		"REVOKE ALL ON FUNCTION public.remove_path_member_data(text, text, text, text) FROM PUBLIC;", "GRANT EXECUTE ON FUNCTION public.remove_path_member_data(text, text, text, text) TO app;",
	} {
		if !strings.Contains(string(up), required) {
			t.Errorf("up migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"invitation.offered_role = membership.role", "SELECT max(transfer.accepted_at)"} {
		if strings.Contains(string(up), forbidden) {
			t.Errorf("continuous membership backfill contains forbidden reset %q", forbidden)
		}
	}
	if !strings.Contains(string(down), "DROP FUNCTION public.remove_path_member_data(text, text, text, text);") {
		t.Errorf("down migration does not drop function")
	}
	if LatestVersion < 56 {
		t.Errorf("LatestVersion=%d want at least 56", LatestVersion)
	}
}
