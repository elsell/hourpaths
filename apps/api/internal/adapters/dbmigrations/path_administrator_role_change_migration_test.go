package dbmigrations

import (
	"os"
	"strings"
	"testing"
)

func TestPathAdministratorRoleChangeMigrationIsAtomicLeastPrivilegeAndReversible(t *testing.T) {
	up, err := os.ReadFile("000058_path_administrator_role_changes.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("000058_path_administrator_role_changes.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"BEGIN;", "COMMIT;", "CREATE OR REPLACE FUNCTION public.change_path_member_role",
		"SECURITY DEFINER", "SET search_path = pg_catalog, public", "path.archived_at IS NULL",
		"administrator_grant", "administrator_revoke", "administrator_step_down",
		"manager_user_identifier <> member_user_identifier", "manager_user_identifier = member_user_identifier",
		"manager_user_identifier <> path_owner", "member_user_identifier = path_owner",
		"target_role = 'participant' AND requested_member_role = 'supporter'",
		"DELETE FROM public.recorded_activity_models", "SET role = requested_member_role",
		"REVOKE ALL ON FUNCTION", "GRANT EXECUTE ON FUNCTION",
		"offered_role IN ('participant', 'supporter', 'administrator')",
	} {
		if !strings.Contains(string(up), required) {
			t.Fatalf("administrator role migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"GRANT UPDATE ON public.path_membership_models",
	} {
		if strings.Contains(string(up), forbidden) {
			t.Fatalf("administrator role migration contains unsafe fragment %q", forbidden)
		}
	}
	for _, required := range []string{"CREATE OR REPLACE FUNCTION public.change_path_member_role", "offered_role IN ('participant', 'supporter')", "DELETE FROM public.notification_models"} {
		if !strings.Contains(string(down), required) {
			t.Fatalf("administrator role down migration missing %q", required)
		}
	}
	if LatestVersion < 58 {
		t.Fatalf("LatestVersion=%d want at least 58", LatestVersion)
	}
}
