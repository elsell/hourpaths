package pathstore

import "testing"

func TestHiddenFeedTargetNotificationKindsAreExplicit(t *testing.T) {
	for kind, want := range map[string]bool{
		"practice_reaction":          true,
		"practice_comment":           true,
		"comment_heart":              true,
		"nudge_received":             false,
		"path_member_left":           false,
		"path_visibility_changed":    false,
		"path_member_role_changed":   false,
		"path_ownership_transferred": false,
	} {
		if got := hiddenFeedTargetNotificationKind(kind); got != want {
			t.Fatalf("hiddenFeedTargetNotificationKind(%q)=%t want %t", kind, got, want)
		}
	}
}
