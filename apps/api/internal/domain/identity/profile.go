package identity

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ProfileText is application-owned public profile copy, not provider identity data.
type ProfileText struct {
	Username, DisplayName, Description string
}

func NormalizeProfileText(profile ProfileText) (ProfileText, error) {
	profile.DisplayName = strings.TrimSpace(profile.DisplayName)
	if strings.TrimSpace(profile.Description) == "" {
		profile.Description = ""
	}
	if ValidateUsername(profile.Username) != nil || !utf8.ValidString(profile.DisplayName) ||
		!utf8.ValidString(profile.Description) || utf8.RuneCountInString(profile.DisplayName) < 1 ||
		utf8.RuneCountInString(profile.DisplayName) > 100 || utf8.RuneCountInString(profile.Description) > 500 {
		return ProfileText{}, errors.New("invalid profile text")
	}
	return profile, nil
}
