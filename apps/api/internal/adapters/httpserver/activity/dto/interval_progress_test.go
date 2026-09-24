package dto

import (
	"encoding/json"
	"testing"
)

func TestIntervalProgressDTOExposesOnlyUncappedProgressAndTarget(t *testing.T) {
	values := []struct {
		name string
		body any
		want int64
	}{
		{name: "current timer zero", body: TimerState{IntervalProgress: &IntervalProgress{AccumulatedSeconds: 0, TargetSeconds: 60}}, want: 0},
		{name: "timer stop over target", body: StopResult{TimerState: TimerState{IntervalProgress: &IntervalProgress{AccumulatedSeconds: 75, TargetSeconds: 60}}}, want: 75},
		{name: "manual create or edit", body: ActivityMutationResult{IntervalProgress: &IntervalProgress{AccumulatedSeconds: 90, TargetSeconds: 60}}, want: 90},
		{name: "activity deletion", body: ActivityDeletionResult{IntervalProgress: &IntervalProgress{AccumulatedSeconds: 0, TargetSeconds: 60}}, want: 0},
	}
	for _, test := range values {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.body)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &body); err != nil {
				t.Fatal(err)
			}
			var progress map[string]int64
			if err := json.Unmarshal(body["intervalProgress"], &progress); err != nil {
				t.Fatalf("intervalProgress=%s error=%v body=%s", body["intervalProgress"], err, encoded)
			}
			if len(progress) != 2 || progress["accumulatedSeconds"] != test.want || progress["targetSeconds"] != 60 {
				t.Fatalf("intervalProgress=%v; want uncapped %d of 60 with no internal window", progress, test.want)
			}
		})
	}
}

func TestIntervalProgressDTOIsOmittedWhenNoGoalExists(t *testing.T) {
	for _, value := range []any{TimerState{}, StopResult{}, ActivityMutationResult{}, ActivityDeletionResult{}} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &body); err != nil {
			t.Fatal(err)
		}
		if _, exists := body["intervalProgress"]; exists {
			t.Fatalf("absent interval goal serialized false progress: %s", encoded)
		}
	}
}
