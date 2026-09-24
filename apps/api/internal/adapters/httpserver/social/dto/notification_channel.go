package dto

type NudgeNotificationChannelPreference struct {
	Channel  string `json:"channel" enum:"nudges"`
	Enabled  bool   `json:"enabled" required:"true"`
	Revision int64  `json:"revision" minimum:"0"`
}

type NudgeNotificationChannelPreferenceInput struct {
	Enabled          bool  `json:"enabled" required:"true"`
	ExpectedRevision int64 `json:"expectedRevision" minimum:"0"`
}
