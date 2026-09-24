package pathstore

import "testing"

func TestLeaveNotificationCleanupUsesEffectivePostLeaveAccess(t *testing.T) {
	for _, test := range []struct {
		name, visibility string
		followsOwner     bool
		blockedByOwner   bool
		retainsAccess    bool
	}{
		{name: "private", visibility: "private", followsOwner: true},
		{name: "followers without follow", visibility: "followers"},
		{name: "followers with follow", visibility: "followers", followsOwner: true, retainsAccess: true},
		{name: "public", visibility: "public", retainsAccess: true},
		{name: "public blocked by owner", visibility: "public", blockedByOwner: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := retainsPathAccessAfterMemberRemoval(test.visibility, test.followsOwner, test.blockedByOwner); got != test.retainsAccess {
				t.Fatalf("effective post-leave access=%t want %t", got, test.retainsAccess)
			}
		})
	}
}
