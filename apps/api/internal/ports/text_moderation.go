package ports

import "context"

type TextSurface string

const (
	CommentText            TextSurface = "comment"
	ProfileDescriptionText TextSurface = "profile_description"
)

// TextModerator checks normalized text before publication. Implementations must
// keep processing on HourPaths infrastructure and must not log submitted text.
// A nil error admits publication; it does not assert a safety classification.
type TextModerator interface {
	Check(context.Context, TextSurface, string) error
}
