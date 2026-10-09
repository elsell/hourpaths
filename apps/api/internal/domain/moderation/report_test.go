package moderation

import (
	"strings"
	"testing"
)

func TestReportExplanationNormalizesWithoutLosingUnicode(t *testing.T) {
	for _, tc := range []struct {
		input, want string
		valid       bool
	}{
		{"  context\n", "context", true},
		{" \t\n", "", true},
		{strings.Repeat("🎸", 1000), strings.Repeat("🎸", 1000), true},
		{strings.Repeat("🎸", 1001), "", false},
		{string([]byte{0xff}), "", false},
	} {
		got, err := NormalizeExplanation(tc.input)
		if (err == nil) != tc.valid || got != tc.want {
			t.Fatalf("normalization: length=%d valid=%v err=%v", len(tc.input), tc.valid, err)
		}
	}
}
