package dto

import "time"

type NudgeAudiencePreference struct {
	PathID   string `json:"pathId"`
	UserID   string `json:"userId"`
	Audience string `json:"audience" enum:"nobody,path_members,followers,everyone"`
	Revision int64  `json:"revision" minimum:"0"`
}

type NudgeAudiencePreferenceInput struct {
	Audience         string `json:"audience" enum:"nobody,path_members,followers,everyone"`
	ExpectedRevision int64  `json:"expectedRevision" minimum:"0"`
}

type NudgeEligibility struct {
	PathID          string `json:"pathId"`
	RecipientUserID string `json:"recipientUserId"`
	Eligible        bool   `json:"eligible" required:"true"`
	Reason          string `json:"reason,omitempty" enum:"goal_complete,rate_limited"`
}

type NudgeContent struct {
	Kind   string `json:"kind" required:"true" enum:"preset"`
	Preset string `json:"preset" required:"true" enum:"you_have_got_this,lets_go,little_progress_counts,keep_it_going,time_to_work"`
}

type NudgeInput struct {
	Content NudgeContent `json:"content" required:"true"`
}

type Nudge struct {
	ID              string       `json:"id"`
	SenderUserID    string       `json:"senderUserId"`
	RecipientUserID string       `json:"recipientUserId"`
	PathID          string       `json:"pathId"`
	Content         NudgeContent `json:"content"`
	SentAt          time.Time    `json:"sentAt"`
}
