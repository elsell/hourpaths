package dbmigrations

import (
	"os"
	"strings"
	"testing"
)

func TestLeavePathMembershipFunctionMigrationIsScopedAndReversible(t *testing.T) {
	up, err := os.ReadFile("000055_leave_path_membership_function.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("000055_leave_path_membership_function.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(up)
	for _, required := range []string{
		"SECURITY DEFINER",
		"SET search_path = pg_catalog, public",
		"WHERE path_id = path_identifier",
		"AND user_id = member_user_identifier",
		"AND role IN ('administrator', 'participant', 'supporter')",
		"REVOKE ALL ON FUNCTION public.leave_path_membership(text, text) FROM PUBLIC;",
		"GRANT EXECUTE ON FUNCTION public.leave_path_membership(text, text) TO app;",
	} {
		if !strings.Contains(contents, required) {
			t.Fatalf("leave function migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"GRANT DELETE ON public.path_membership_models TO app",
		"GRANT ALL",
	} {
		if strings.Contains(contents, forbidden) {
			t.Fatalf("leave function migration overgrants runtime access: %q", forbidden)
		}
	}
	if !strings.Contains(string(down), "DROP FUNCTION public.leave_path_membership(text, text);") {
		t.Fatal("leave function rollback does not remove the exact function signature")
	}
}
