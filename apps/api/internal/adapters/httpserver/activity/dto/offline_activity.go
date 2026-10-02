package dto

import "time"

type OfflineActivityInput struct {
	Kind               string    `json:"kind" enum:"create,edit"`
	ActivityID         string    `json:"activityId" minLength:"1" maxLength:"128"`
	StartedAt          time.Time `json:"startedAt"`
	DurationSeconds    int64     `json:"durationSeconds" minimum:"1"`
	OccurrenceTimeZone string    `json:"occurrenceTimeZone" minLength:"1" maxLength:"128"`
	Note               string    `json:"note" maxLength:"2000"`
	AuthoredAt         time.Time `json:"authoredAt"`
	Counter            int64     `json:"counter" minimum:"0" maximum:"9007199254740991"`
}
type ActivityEditOrder struct {
	AuthoredAt  time.Time `json:"authoredAt"`
	Counter     int64     `json:"counter" minimum:"0" maximum:"9007199254740991"`
	OperationID string    `json:"operationId"`
}
type OfflineActivityResult struct {
	Outcome  string             `json:"outcome" enum:"accepted,deleted,archived"`
	Activity *Activity          `json:"activity,omitempty"`
	Order    *ActivityEditOrder `json:"order,omitempty"`
}
