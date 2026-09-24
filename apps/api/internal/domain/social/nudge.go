package social

import (
	"strings"
	"time"
)

type NudgePreset string

const (
	NudgeYouHaveGotThis       NudgePreset = "you_have_got_this"
	NudgeLetsGo               NudgePreset = "lets_go"
	NudgeLittleProgressCounts NudgePreset = "little_progress_counts"
	NudgeKeepItGoing          NudgePreset = "keep_it_going"
	NudgeTimeToWork           NudgePreset = "time_to_work"
)

var nudgePresets = [...]NudgePreset{
	NudgeYouHaveGotThis,
	NudgeLetsGo,
	NudgeLittleProgressCounts,
	NudgeKeepItGoing,
	NudgeTimeToWork,
}

func NudgePresets() []NudgePreset {
	result := make([]NudgePreset, len(nudgePresets))
	copy(result, nudgePresets[:])
	return result
}

func (preset NudgePreset) Valid() bool {
	switch preset {
	case NudgeYouHaveGotThis, NudgeLetsGo, NudgeLittleProgressCounts, NudgeKeepItGoing, NudgeTimeToWork:
		return true
	default:
		return false
	}
}

type NudgeAudience string

const (
	NudgeAudienceNobody      NudgeAudience = "nobody"
	NudgeAudiencePathMembers NudgeAudience = "path_members"
	NudgeAudienceFollowers   NudgeAudience = "followers"
	NudgeAudienceEveryone    NudgeAudience = "everyone"

	DefaultNudgeAudience = NudgeAudiencePathMembers
)

var nudgeAudiences = [...]NudgeAudience{
	NudgeAudienceNobody,
	NudgeAudiencePathMembers,
	NudgeAudienceFollowers,
	NudgeAudienceEveryone,
}

func NudgeAudiences() []NudgeAudience {
	result := make([]NudgeAudience, len(nudgeAudiences))
	copy(result, nudgeAudiences[:])
	return result
}

func (audience NudgeAudience) Valid() bool {
	switch audience {
	case NudgeAudienceNobody, NudgeAudiencePathMembers, NudgeAudienceFollowers, NudgeAudienceEveryone:
		return true
	default:
		return false
	}
}

type NudgeContentKind string

const NudgeContentPreset NudgeContentKind = "preset"

type NudgeContent struct {
	Kind   NudgeContentKind
	Preset NudgePreset
}

func (content NudgeContent) Valid() bool {
	return content.Kind == NudgeContentPreset && content.Preset.Valid()
}

type Nudge struct {
	ID, SenderID, RecipientID, PathID string
	Content                           NudgeContent
	SentAt                            time.Time
}

func (nudge Nudge) Valid() bool {
	return validNudgeID(nudge.ID) && validNudgeID(nudge.SenderID) &&
		validNudgeID(nudge.RecipientID) && nudge.SenderID != nudge.RecipientID &&
		validNudgeID(nudge.PathID) && nudge.Content.Valid() &&
		!nudge.SentAt.IsZero() && nudge.SentAt.Location() == time.UTC
}

func validNudgeID(value string) bool {
	return value != "" && strings.TrimSpace(value) == value
}
