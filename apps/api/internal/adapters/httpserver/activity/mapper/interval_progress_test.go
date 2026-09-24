package mapper

import (
	"encoding/json"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
)

func TestActivityMappersPreserveOptionalUncappedIntervalProgress(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	timer, _ := domain.StartTimer("timer", "path", "participant", now.Add(-time.Minute), "Etc/UTC", now)
	entry, _, _ := timer.Stop("activity", now, now)
	edited, revision, err := entry.EditByOwner("participant", domain.ActivityEdit{
		StartedAt: now.Add(-2 * time.Minute), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC",
	}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	window := application.IntervalWindow{StartedAt: now.Add(-time.Hour), EndedAt: now.Add(time.Hour)}
	progress := func(seconds int64) *application.IntervalProgress {
		return &application.IntervalProgress{AccumulatedSeconds: seconds, TargetSeconds: 60, Window: window}
	}
	values := []struct {
		name string
		body any
		want int64
	}{
		{name: "current timer", body: Current(application.CurrentTimerResult{IntervalProgress: progress(0)}), want: 0},
		{name: "timer start", body: Started(application.StartTimerResult{Timer: timer, IntervalProgress: progress(75)}), want: 75},
		{name: "timer stop", body: Stopped(application.StopTimerResult{Activity: entry, Saved: true, IntervalProgress: progress(90)}), want: 90},
		{name: "manual create", body: Created(application.CreateManualActivityResult{Activity: entry, Version: 1, IntervalProgress: progress(0)}), want: 0},
		{name: "manual edit", body: Updated(application.UpdateActivityResult{Activity: edited, Revision: revision, Version: 2, IntervalProgress: progress(75)}), want: 75},
		{name: "activity delete", body: Deleted(application.DeleteActivityResult{IntervalProgress: progress(90)}), want: 90},
	}
	for _, test := range values {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.body)
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				IntervalProgress *struct {
					AccumulatedSeconds int64 `json:"accumulatedSeconds"`
					TargetSeconds      int64 `json:"targetSeconds"`
				} `json:"intervalProgress"`
			}
			if err := json.Unmarshal(encoded, &body); err != nil || body.IntervalProgress == nil || body.IntervalProgress.AccumulatedSeconds != test.want || body.IntervalProgress.TargetSeconds != 60 {
				t.Fatalf("mapped interval progress=%+v error=%v body=%s", body.IntervalProgress, err, encoded)
			}
			if string(encoded) == "" || json.Valid(encoded) == false {
				t.Fatalf("invalid mapped JSON: %s", encoded)
			}
			var fields map[string]json.RawMessage
			_ = json.Unmarshal(encoded, &fields)
			if raw := fields["intervalProgress"]; string(raw) == "" || containsJSONField(raw, "window") {
				t.Fatalf("internal interval window leaked: %s", encoded)
			}
		})
	}
}

func containsJSONField(value json.RawMessage, field string) bool {
	var fields map[string]json.RawMessage
	return json.Unmarshal(value, &fields) == nil && fields[field] != nil
}
