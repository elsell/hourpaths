package moderation

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidReport = errors.New("invalid report")

type Reason string

const (
	SpamOrScam                   Reason = "spam_or_scam"
	HarassmentOrBullying         Reason = "harassment_or_bullying"
	HateOrAbusiveContent         Reason = "hate_or_abusive_content"
	SexualOrInappropriateContent Reason = "sexual_or_inappropriate_content"
	Impersonation                Reason = "impersonation"
	PrivacyOrPersonalInformation Reason = "privacy_or_personal_information"
	DangerousOrSelfHarmContent   Reason = "dangerous_or_self_harm_content"
	SomethingElse                Reason = "something_else"
)

func (r Reason) Valid() bool {
	switch r {
	case SpamOrScam, HarassmentOrBullying, HateOrAbusiveContent,
		SexualOrInappropriateContent, Impersonation, PrivacyOrPersonalInformation,
		DangerousOrSelfHarmContent, SomethingElse:
		return true
	default:
		return false
	}
}

type TargetKind string

const (
	Profile   TargetKind = "profile"
	Path      TargetKind = "path"
	FeedEvent TargetKind = "feed_event"
	Comment   TargetKind = "comment"
	Nudge     TargetKind = "nudge"
)

type Target struct {
	Kind TargetKind
	ID   string
}

func (t Target) Valid() bool {
	if t.ID == "" || strings.TrimSpace(t.ID) != t.ID || len(t.ID) > 200 {
		return false
	}
	switch t.Kind {
	case Profile, Path, FeedEvent, Comment, Nudge:
		return true
	default:
		return false
	}
}

func NormalizeExplanation(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", ErrInvalidReport
	}
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > 1000 {
		return "", ErrInvalidReport
	}
	return value, nil
}
