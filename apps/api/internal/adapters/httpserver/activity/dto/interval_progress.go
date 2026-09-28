package dto

import "time"

type IntervalProgress struct {
	StartedAt          *time.Time `json:"startedAt,omitempty"`
	EndedAt            *time.Time `json:"endedAt,omitempty"`
	AccumulatedSeconds int64      `json:"accumulatedSeconds" minimum:"0"`
	TargetSeconds      int64      `json:"targetSeconds" minimum:"1"`
}
