package sociallock

import "testing"

func TestInteractionOwnerKeyIsLengthDelimited(t *testing.T) {
	if got, want := InteractionOwnerKey("person"), "24:social-interaction-owner6:person"; got != want {
		t.Fatalf("InteractionOwnerKey()=%q want %q", got, want)
	}
}
