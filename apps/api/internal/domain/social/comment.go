package social

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const MaximumCommentCharacters = 2000

var ErrInvalidCommentText = errors.New("comment text must contain 1 to 2000 valid Unicode characters and no control characters other than line feed")

// Comment is the current authoritative version of a comment on one feed
// event. Version begins at one and increases whenever Text is replaced.
type Comment struct {
	ID        string
	EventID   string
	AuthorID  string
	Text      string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Valid reports whether a comment is safe to cross an application boundary.
// Text must already be in the canonical form returned by NormalizeCommentText.
func (comment Comment) Valid() bool {
	return validCommentID(comment.ID) && validCommentID(comment.EventID) && validCommentID(comment.AuthorID) &&
		validCanonicalCommentText(comment.Text) && comment.Version > 0 &&
		validCommentInstant(comment.CreatedAt) && validCommentInstant(comment.UpdatedAt) &&
		!comment.UpdatedAt.Before(comment.CreatedAt)
}

func (comment Comment) Edited() bool { return comment.Version > 1 }

// CommentVersion is one immutable value in a comment's edit history. CreatedAt
// is the instant that this version became authoritative.
type CommentVersion struct {
	CommentID string
	Text      string
	Version   int64
	CreatedAt time.Time
}

func (version CommentVersion) Valid() bool {
	return validCommentID(version.CommentID) && validCanonicalCommentText(version.Text) &&
		version.Version > 0 && validCommentInstant(version.CreatedAt)
}

// NormalizeCommentText produces the only text representation accepted by
// Comment and CommentVersion. Surrounding Unicode whitespace is removed before
// NFC normalization, and length is measured in Unicode code points afterward.
func NormalizeCommentText(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", ErrInvalidCommentText
	}
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' {
			return "", ErrInvalidCommentText
		}
	}
	value = norm.NFC.String(strings.TrimSpace(value))
	characterCount := utf8.RuneCountInString(value)
	if characterCount < 1 || characterCount > MaximumCommentCharacters {
		return "", ErrInvalidCommentText
	}
	return value, nil
}

func validCanonicalCommentText(value string) bool {
	normalized, err := NormalizeCommentText(value)
	return err == nil && normalized == value
}

func validCommentID(value string) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) != ""
}

func validCommentInstant(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC
}
