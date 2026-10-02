package dto

import "time"

type OfflineTimerInput struct {
	CorrectedStartedAt *time.Time `json:"correctedStartedAt,omitempty"`
	TimerID            string     `json:"timerId" minLength:"1" maxLength:"128"`
	Kind               string     `json:"kind" enum:"start,stop,correct"`
	StartedAt          time.Time  `json:"startedAt"`
	EndedAt            *time.Time `json:"endedAt,omitempty"`
	OccurrenceTimeZone string     `json:"occurrenceTimeZone" minLength:"1" maxLength:"128"`
}

type OfflineTimerResult struct {
	Terminal         bool      `json:"terminal"`
	MustStop         bool      `json:"mustStop"`
	Outcome          string    `json:"outcome" enum:"accepted,conflict,archived"`
	Timer            *Timer    `json:"timer,omitempty"`
	Activity         *Activity `json:"activity,omitempty"`
	SavedSeconds     int64     `json:"savedSeconds" minimum:"0"`
	DiscardedSeconds int64     `json:"discardedSeconds" minimum:"0"`
}
