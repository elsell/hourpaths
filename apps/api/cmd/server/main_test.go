package main

import (
	"strings"
	"testing"
)

func TestSocialRelationshipRateLimitKeyIsDatabaseSafeAndUnambiguous(t *testing.T) {
	first := socialRelationshipRateLimitKey("a", "bc")
	second := socialRelationshipRateLimitKey("ab", "c")
	if strings.ContainsRune(first, '\x00') || strings.ContainsRune(second, '\x00') {
		t.Fatal("social relationship rate-limit keys must be valid database text")
	}
	if first == second {
		t.Fatalf("length-prefixed rate-limit keys collided: %q", first)
	}
}
