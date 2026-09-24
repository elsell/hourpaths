package dto

import "time"

type ManualActivityInputBody struct {
	LocalDate       string `json:"localDate" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	LocalStartTime  string `json:"localStartTime" pattern:"^[0-9]{2}:[0-9]{2}:[0-9]{2}$"`
	DurationSeconds int64  `json:"durationSeconds" minimum:"1"`
	// Length is enforced after NFC normalization by the domain. A raw JSON
	// maxLength would reject valid decomposed input before normalization.
	Note string `json:"note,omitempty"`
}

type ManualActivityDefaults struct {
	LocalDate      string    `json:"localDate"`
	LocalStartTime string    `json:"localStartTime"`
	TimeZone       string    `json:"timeZone"`
	CurrentInstant time.Time `json:"currentInstant"`
}

type ActivityMutationResult struct {
	Activity           Activity          `json:"activity"`
	Version            int64             `json:"version" minimum:"1"`
	AccumulatedSeconds int64             `json:"accumulatedSeconds" minimum:"0"`
	IntervalProgress   *IntervalProgress `json:"intervalProgress,omitempty"`
}

type ActivityDeletionResult struct {
	AccumulatedSeconds      int64             `json:"accumulatedSeconds" minimum:"0"`
	SessionCount            int64             `json:"sessionCount" minimum:"0"`
	UnreadNotificationCount int64             `json:"unreadNotificationCount" minimum:"0"`
	RemovedFeedEventIDs     []string          `json:"removedFeedEventIds" nullable:"false" minItems:"1"`
	IntervalProgress        *IntervalProgress `json:"intervalProgress,omitempty"`
}

type ActivityDetail struct {
	Activity Activity `json:"activity"`
	Version  int64    `json:"version" minimum:"1"`
}

type ActivityRevision struct {
	Activity
	Version    int64     `json:"version" minimum:"1"`
	ReplacedAt time.Time `json:"replacedAt"`
}
