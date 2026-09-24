package dbmigrations

import (
	"strings"
	"testing"
)

func TestOwnershipTransferMembershipPrivilegeIsColumnScopedAndReversible(t *testing.T) {
	if LatestVersion < 41 {
		t.Fatalf("latest migration version = %d, want membership privilege migration 41 or later", LatestVersion)
	}
	up, err := Files.ReadFile("000041_ownership_transfer_membership_privilege.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := Files.ReadFile("000041_ownership_transfer_membership_privilege.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(up), "GRANT UPDATE (role) ON public.path_membership_models TO app") {
		t.Fatal("ownership transfer must grant the runtime role only column-scoped membership role updates")
	}
	if strings.Contains(string(up), "GRANT UPDATE ON") || strings.Contains(string(up), "GRANT ALL") {
		t.Fatal("ownership transfer membership privilege must not grant table-wide mutation")
	}
	if !strings.Contains(string(down), "REVOKE UPDATE (role) ON public.path_membership_models FROM app") {
		t.Fatal("ownership transfer membership privilege rollback must revoke the exact column grant")
	}
}
