package identity

import "errors"

const (
	usernameMinLength = 3
	usernameMaxLength = 64
)

var errInvalidUsername = errors.New("username must contain 3 to 64 ASCII letters, numbers, underscores, or periods")

// ValidateUsername applies the profile username format without changing the
// caller's chosen display casing.
func ValidateUsername(value string) error {
	if len(value) < usernameMinLength || len(value) > usernameMaxLength {
		return errInvalidUsername
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if !asciiUsernameLetterOrDigit(character) && character != '_' && character != '.' {
			return errInvalidUsername
		}
	}
	return nil
}

// UsernameSuggestion derives a deterministic lowercase candidate from the
// ASCII portions of a provider display name. It does not reserve the candidate;
// uniqueness remains an atomic confirmation-time concern.
func UsernameSuggestion(displayName string) string {
	candidate := make([]byte, 0, usernameMaxLength)
	separatorPending := false
	for _, character := range displayName {
		var ascii byte
		switch {
		case character >= 'A' && character <= 'Z':
			ascii = byte(character + ('a' - 'A'))
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			ascii = byte(character)
		default:
			if len(candidate) > 0 {
				separatorPending = true
			}
			continue
		}

		if separatorPending && len(candidate) < usernameMaxLength {
			candidate = append(candidate, '.')
		}
		separatorPending = false
		if len(candidate) < usernameMaxLength {
			candidate = append(candidate, ascii)
		}
		if len(candidate) == usernameMaxLength {
			break
		}
	}

	for len(candidate) > 0 && candidate[len(candidate)-1] == '.' {
		candidate = candidate[:len(candidate)-1]
	}
	if len(candidate) == 0 {
		return ""
	}
	for len(candidate) < usernameMinLength {
		candidate = append(candidate, '_')
	}
	return string(candidate)
}

func asciiUsernameLetterOrDigit(character byte) bool {
	return character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z' ||
		character >= '0' && character <= '9'
}
