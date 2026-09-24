package identity

import "testing"

func TestDuplicateEmailRecoveryDeclineIsScopedToOwnerAndNormalizedEmail(t *testing.T) {
	decline, err := NewDuplicateEmailRecoveryDecline("provisional-owner", " Person@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if decline.ProvisionalUserID != "provisional-owner" || decline.NormalizedEmail != "person@example.com" {
		t.Fatalf("decline = %+v", decline)
	}
	if !decline.AppliesTo("provisional-owner", "PERSON@example.com") {
		t.Fatal("decline did not apply to the same owner and normalized email")
	}
	if decline.AppliesTo("other-provisional", "person@example.com") {
		t.Fatal("decline leaked to another provisional owner")
	}
	if decline.AppliesTo("provisional-owner", "changed@example.com") {
		t.Fatal("decline leaked to a changed normalized email")
	}
}

func TestDuplicateEmailRecoveryDeclineRejectsMissingScope(t *testing.T) {
	for _, test := range []struct {
		name, userID, email string
	}{
		{name: "missing owner", email: "person@example.com"},
		{name: "missing email", userID: "provisional-owner"},
		{name: "blank email", userID: "provisional-owner", email: "  "},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewDuplicateEmailRecoveryDecline(test.userID, test.email); err == nil {
				t.Fatal("invalid recovery decline was accepted")
			}
		})
	}
}
