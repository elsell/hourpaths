package gormstore

import (
	"testing"

	sociallock "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
)

func TestPathLifecycleAndSocialWritersShareInteractionOwnerLockKey(t *testing.T) {
	const owner = "event-owner"
	if writer, lifecycle := socialLockKey("social-interaction-owner", owner), sociallock.InteractionOwnerKey(owner); writer != lifecycle {
		t.Fatalf("social writer key %q differs from lifecycle key %q", writer, lifecycle)
	}
}
