package identity

import "testing"

func TestUserIDMatchesAcceptanceFixtureKnownAnswer(t *testing.T) {
	const expected = "4ce4570c-8fdd-5c76-8fa3-69c92efba210"
	if actual := UserID("http://localhost:5556/dex", "subject"); actual != expected {
		t.Fatalf("UserID known answer = %q, want %q", actual, expected)
	}
}

func TestOnlyActiveAccountsHaveFullApplicationAccess(t *testing.T) {
	for _, test := range []struct {
		status Status
		want   bool
	}{
		{status: StatusProvisional, want: false},
		{status: StatusActive, want: true},
		{status: StatusDisabled, want: false},
	} {
		user := User{Status: test.status}
		if got := user.HasFullApplicationAccess(); got != test.want {
			t.Fatalf("status %q full application access = %v, want %v", test.status, got, test.want)
		}
	}
}
