package pathstore

import "testing"

func TestVisibilityCleanupKindsAndEffectiveAudienceAreExplicit(t *testing.T) {
	kinds := map[string]bool{
		"nudge_received": true, "practice_reaction": true, "practice_comment": true, "comment_heart": true,
		"path_visibility_changed": false, "path_member_removed": false, "path_member_role_changed": false,
	}
	for kind, want := range kinds {
		if got := visibilityTargetOpeningNotificationKind(kind); got != want {
			t.Fatalf("visibilityTargetOpeningNotificationKind(%q)=%t want %t", kind, got, want)
		}
	}
	cases := []struct {
		visibility               string
		member, follows, blocked bool
		want                     bool
	}{
		{"private", true, false, false, true}, {"private", false, true, false, false},
		{"followers", false, true, false, true}, {"followers", false, false, false, false},
		{"public", false, false, false, true}, {"public", false, false, true, false},
	}
	for _, tc := range cases {
		if got := retainsPathAccessAfterVisibility(tc.visibility, tc.member, tc.follows, tc.blocked); got != tc.want {
			t.Fatalf("retainsPathAccessAfterVisibility(%+v)=%t want %t", tc, got, tc.want)
		}
	}
}

func TestVisibilityNarrowingMatrix(t *testing.T) {
	tests := []struct {
		from, to                 string
		member, follows, blocked bool
		retains                  bool
	}{
		{"public", "private", false, false, false, false},
		{"public", "followers", false, true, false, true},
		{"public", "followers", false, false, false, false},
		{"followers", "private", false, true, false, false},
		{"public", "private", true, false, false, true},
	}
	for _, test := range tests {
		if visibilityRank(test.to) >= visibilityRank(test.from) {
			t.Fatalf("invalid narrowing fixture %+v", test)
		}
		if got := retainsPathAccessAfterVisibility(test.to, test.member, test.follows, test.blocked); got != test.retains {
			t.Fatalf("visibility matrix %+v retains=%t", test, got)
		}
	}
}
