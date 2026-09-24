package gormstore

import (
	"strings"
	"testing"
)

func TestSocialLockKeyIsPostgresTextSafeAndUnambiguous(t *testing.T) {
	first := socialLockKey("social-user-pair", "a", "bc")
	second := socialLockKey("social-user-pair", "ab", "c")
	if strings.ContainsRune(first, '\x00') || strings.ContainsRune(second, '\x00') {
		t.Fatal("social lock keys must be valid PostgreSQL text")
	}
	if first == second {
		t.Fatalf("length-prefixed social lock keys collided: %q", first)
	}
}
