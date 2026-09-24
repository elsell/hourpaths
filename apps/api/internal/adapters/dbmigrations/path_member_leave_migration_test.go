package dbmigrations

import (
	"os"
	"strings"
	"testing"
)

func TestPathMemberLeaveNotificationMigrationIsBoundedAndReversible(t *testing.T) {
	up, err := os.ReadFile("000054_path_member_leave_notifications.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("000054_path_member_leave_notifications.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"'path_member_left'", "presentation_class = 'informational'", "channel = 'path_access'"} {
		if !strings.Contains(string(up), required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	for _, required := range []string{"cannot roll back", "kind = 'path_member_left'"} {
		if !strings.Contains(string(down), required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
