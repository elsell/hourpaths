package identity

import (
	"strings"
	"testing"
)

func TestValidateUsernameAcceptsTheCasePreservingContract(t *testing.T) {
	for _, username := range []string{
		"abc",
		"Hour.Paths_2026",
		strings.Repeat("A", 64),
	} {
		t.Run(username, func(t *testing.T) {
			if err := ValidateUsername(username); err != nil {
				t.Fatalf("ValidateUsername(%q) error = %v", username, err)
			}
		})
	}
}

func TestValidateUsernameRejectsValuesOutsideTheExactFormat(t *testing.T) {
	for _, username := range []string{
		"",
		"ab",
		strings.Repeat("a", 65),
		"has space",
		"has-hyphen",
		"café",
		"line\nbreak",
	} {
		t.Run(username, func(t *testing.T) {
			if err := ValidateUsername(username); err == nil {
				t.Fatalf("ValidateUsername(%q) accepted an invalid username", username)
			}
		})
	}
}

func TestUsernameSuggestionDerivesDeterministicValidLowercaseCandidates(t *testing.T) {
	for _, test := range []struct {
		displayName string
		want        string
	}{
		{displayName: "Provider Display Name", want: "provider.display.name"},
		{displayName: "  Mixed---Separators__Here  ", want: "mixed.separators.here"},
		{displayName: "Al", want: "al_"},
		{displayName: "A", want: "a__"},
		{displayName: "José Núñez", want: "jos.n.ez"},
		{displayName: strings.Repeat("LONG", 20), want: strings.Repeat("long", 16)},
	} {
		t.Run(test.displayName, func(t *testing.T) {
			got := UsernameSuggestion(test.displayName)
			if got != test.want {
				t.Fatalf("UsernameSuggestion(%q) = %q, want %q", test.displayName, got, test.want)
			}
			if err := ValidateUsername(got); err != nil {
				t.Fatalf("suggested username %q is invalid: %v", got, err)
			}
			if got != strings.ToLower(got) {
				t.Fatalf("suggested username %q is not lowercase", got)
			}
		})
	}
}

func TestUsernameSuggestionReturnsEmptyOnlyWithoutUsableASCII(t *testing.T) {
	for _, displayName := range []string{"", "   ", "李雷", "🎸"} {
		if got := UsernameSuggestion(displayName); got != "" {
			t.Fatalf("UsernameSuggestion(%q) = %q, want empty", displayName, got)
		}
	}
}
