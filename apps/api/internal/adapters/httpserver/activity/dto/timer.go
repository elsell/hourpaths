package dto

import "time"

type Timer struct {
	ID                 string    `json:"id"`
	PathID             string    `json:"pathId"`
	StartedAt          time.Time `json:"startedAt"`
	OccurrenceTimeZone string    `json:"occurrenceTimeZone"`
}

type Activity struct {
	ID                 string    `json:"id"`
	PathID             string    `json:"pathId"`
	ParticipantID      string    `json:"participantId"`
	StartedAt          time.Time `json:"startedAt"`
	EndedAt            time.Time `json:"endedAt"`
	OccurrenceTimeZone string    `json:"occurrenceTimeZone"`
	DurationSeconds    int64     `json:"durationSeconds" minimum:"1"`
	Note               *string   `json:"note,omitempty" maxLength:"2000"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type TimerState struct {
	Running            bool              `json:"running"`
	Timer              *Timer            `json:"timer,omitempty"`
	AccumulatedSeconds int64             `json:"accumulatedSeconds" minimum:"0"`
	IntervalProgress   *IntervalProgress `json:"intervalProgress,omitempty"`
}

type StopResult struct {
	TimerState
	Saved     bool      `json:"saved"`
	Subsecond bool      `json:"subsecond"`
	Activity  *Activity `json:"activity,omitempty"`
}
