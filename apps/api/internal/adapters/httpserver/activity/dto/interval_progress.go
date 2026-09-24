package dto

type IntervalProgress struct {
	AccumulatedSeconds int64 `json:"accumulatedSeconds" minimum:"0"`
	TargetSeconds      int64 `json:"targetSeconds" minimum:"1"`
}
