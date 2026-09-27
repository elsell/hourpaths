package path

import (
	"regexp"
	"unicode/utf8"
)

type Appearance struct {
	Color    string
	Emoji    string
	Revision int64
}

var emojiSequence = regexp.MustCompile(`^(?:[\x{1F1E6}-\x{1F1FF}]{2}|[0-9#*]\x{FE0F}?\x{20E3}|[\x{1F000}-\x{1FAFF}\x{2600}-\x{27BF}\x{2300}-\x{23FF}\x{2B00}-\x{2BFF}\x{2194}-\x{21FF}\x{00A9}\x{00AE}\x{203C}\x{2049}\x{2122}\x{2139}\x{3030}\x{303D}\x{3297}\x{3299}]\x{FE0F}?[\x{1F3FB}-\x{1F3FF}]?(?:[\x{E0020}-\x{E007E}]+\x{E007F})?(?:\x{200D}[\x{1F000}-\x{1FAFF}\x{2600}-\x{27BF}\x{2300}-\x{23FF}\x{2B00}-\x{2BFF}\x{2194}-\x{21FF}\x{00A9}\x{00AE}\x{203C}\x{2049}\x{2122}\x{2139}\x{3030}\x{303D}\x{3297}\x{3299}]\x{FE0F}?[\x{1F3FB}-\x{1F3FF}]?)*)$`)

func (a Appearance) ValidChoice() bool {
	switch a.Color {
	case "coral", "lavender", "gold", "mint", "blue", "pink":
	default:
		return false
	}
	if !utf8.ValidString(a.Emoji) || utf8.RuneCountInString(a.Emoji) > 32 || !emojiSequence.MatchString(a.Emoji) {
		return false
	}
	first, _ := utf8.DecodeRuneInString(a.Emoji)
	if first >= 0x1F3FB && first <= 0x1F3FF {
		return false
	}
	if first >= 0x1F1E6 && first <= 0x1F1FF {
		return utf8.RuneCountInString(a.Emoji) == 2
	}
	return true
}
