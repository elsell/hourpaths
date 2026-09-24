package social

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizeCommentTextTrimsThenNormalizesValidUnicode(t *testing.T) {
	got, err := NormalizeCommentText(" \nCafe\u0301 practice\n\u7df4\u7fd2 \U0001f3b8  ")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Caf\u00e9 practice\n\u7df4\u7fd2 \U0001f3b8"; got != want {
		t.Fatalf("NormalizeCommentText() = %q, want %q", got, want)
	}
}

func TestNormalizeCommentTextCountsUnicodeCodePointsAfterNFC(t *testing.T) {
	valid := strings.Repeat("e\u0301", MaximumCommentCharacters-1) + "\U0001f642"
	got, err := NormalizeCommentText(valid)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Repeat("\u00e9", MaximumCommentCharacters-1) + "\U0001f642"; got != want {
		t.Fatalf("normalized boundary text does not match: got length %d, want length %d", len([]rune(got)), MaximumCommentCharacters)
	}

	if _, err := NormalizeCommentText(valid + "x"); !errors.Is(err, ErrInvalidCommentText) {
		t.Fatalf("over-length text error = %v, want %v", err, ErrInvalidCommentText)
	}
}

func TestNormalizeCommentTextRejectsEmptyInvalidEncodingAndControlsOtherThanLF(t *testing.T) {
	tests := map[string]string{
		"empty":           "",
		"whitespace only": " \n\u2003\t ",
		"invalid UTF-8":   string([]byte{0xff}),
		"carriage return": "first\rsecond",
		"trailing CR":     "first\r",
		"tab":             "first\tsecond",
		"leading tab":     "\tsecond",
		"nul":             "first\x00second",
		"delete":          "first\x7fsecond",
		"unicode control": "first\u0085second",
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			if got, err := NormalizeCommentText(value); !errors.Is(err, ErrInvalidCommentText) || got != "" {
				t.Fatalf("NormalizeCommentText(%q) = %q, %v; want empty, %v", value, got, err, ErrInvalidCommentText)
			}
		})
	}
}

func TestCommentValidRequiresCanonicalIdentityTextVersionAndLifecycle(t *testing.T) {
	createdAt := time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC)
	valid := Comment{
		ID: "comment-1", EventID: "practice:activity-1", AuthorID: "user-1",
		Text: "Great work!\n\U0001f389", Version: 1, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	if !valid.Valid() || valid.Edited() {
		t.Fatalf("new valid comment rejected or marked edited: %+v", valid)
	}

	edited := valid
	edited.Version = 2
	edited.UpdatedAt = createdAt.Add(time.Minute)
	if !edited.Valid() || !edited.Edited() {
		t.Fatalf("edited valid comment rejected or not marked edited: %+v", edited)
	}

	tests := map[string]func(*Comment){
		"blank ID":               func(value *Comment) { value.ID = " " },
		"blank event ID":         func(value *Comment) { value.EventID = "\t" },
		"blank author ID":        func(value *Comment) { value.AuthorID = "" },
		"noncanonical text":      func(value *Comment) { value.Text = "Cafe\u0301" },
		"invalid text":           func(value *Comment) { value.Text = "hello\rworld" },
		"zero version":           func(value *Comment) { value.Version = 0 },
		"negative version":       func(value *Comment) { value.Version = -1 },
		"missing creation time":  func(value *Comment) { value.CreatedAt = time.Time{} },
		"missing update time":    func(value *Comment) { value.UpdatedAt = time.Time{} },
		"non-UTC creation time":  func(value *Comment) { value.CreatedAt = value.CreatedAt.In(time.FixedZone("EST", -5*60*60)) },
		"non-UTC update time":    func(value *Comment) { value.UpdatedAt = value.UpdatedAt.In(time.FixedZone("EST", -5*60*60)) },
		"update before creation": func(value *Comment) { value.UpdatedAt = value.CreatedAt.Add(-time.Nanosecond) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			value := valid
			mutate(&value)
			if value.Valid() {
				t.Fatalf("invalid comment accepted: %+v", value)
			}
		})
	}
}

func TestCommentVersionValidatesImmutableHistoryValue(t *testing.T) {
	createdAt := time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC)
	valid := CommentVersion{CommentID: "comment-1", Text: "Original text", Version: 1, CreatedAt: createdAt}
	if !valid.Valid() {
		t.Fatalf("valid comment version rejected: %+v", valid)
	}

	tests := map[string]func(*CommentVersion){
		"blank comment ID":      func(value *CommentVersion) { value.CommentID = " " },
		"noncanonical text":     func(value *CommentVersion) { value.Text = "Cafe\u0301" },
		"invalid text":          func(value *CommentVersion) { value.Text = "one\ttwo" },
		"nonpositive version":   func(value *CommentVersion) { value.Version = 0 },
		"missing creation time": func(value *CommentVersion) { value.CreatedAt = time.Time{} },
		"non-UTC creation time": func(value *CommentVersion) { value.CreatedAt = value.CreatedAt.In(time.FixedZone("EST", -5*60*60)) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			value := valid
			mutate(&value)
			if value.Valid() {
				t.Fatalf("invalid comment version accepted: %+v", value)
			}
		})
	}
}
