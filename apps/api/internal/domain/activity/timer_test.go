package activity

import (
	"testing"
	"time"
)

func TestStoppingTimerAfterOneSecondCreatesCanonicalActivity(t *testing.T) {
	startedAt := time.Date(2026, time.July, 21, 17, 30, 0, 250_000_000, time.FixedZone("EDT", -4*60*60))
	stoppedAt := startedAt.Add(1500 * time.Millisecond)
	recordedAt := stoppedAt.Add(2 * time.Second)

	timer, err := StartTimer("timer-1", "path-1", "participant-1", startedAt, "America/New_York", startedAt)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	entry, saved, err := timer.Stop("activity-1", stoppedAt, recordedAt)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if !saved {
		t.Fatal("Stop() did not save a timer that ran for at least one second")
	}
	if entry.ID != "activity-1" || entry.PathID != "path-1" || entry.ParticipantID != "participant-1" {
		t.Fatalf("activity identity and attribution = %+v", entry)
	}
	if entry.StartedAt != startedAt.UTC() || entry.EndedAt != stoppedAt.UTC() {
		t.Fatalf("activity instants = (%v, %v), want (%v, %v)", entry.StartedAt, entry.EndedAt, startedAt.UTC(), stoppedAt.UTC())
	}
	if entry.OccurrenceTimeZone != "America/New_York" {
		t.Fatalf("occurrence time zone = %q", entry.OccurrenceTimeZone)
	}
	if entry.CreatedAt != recordedAt.UTC() || entry.UpdatedAt != recordedAt.UTC() {
		t.Fatalf("activity timestamps = (%v, %v), want %v", entry.CreatedAt, entry.UpdatedAt, recordedAt.UTC())
	}
	if got := entry.DurationSeconds(); got != 1 {
		t.Fatalf("DurationSeconds() = %v, want one-second precision derived from the UTC instants", got)
	}
}

func TestStoppingTimerUsesWholeSecondBoundary(t *testing.T) {
	startedAt := time.Date(2026, time.July, 21, 21, 30, 0, 250_000_000, time.UTC)
	timer, err := StartTimer("timer-1", "path-1", "participant-1", startedAt, "America/New_York", startedAt)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	for name, elapsed := range map[string]time.Duration{
		"zero":                  0,
		"just under one second": time.Second - time.Nanosecond,
	} {
		t.Run(name, func(t *testing.T) {
			entry, saved, stopErr := timer.Stop("", startedAt.Add(elapsed), startedAt.Add(time.Second))
			if stopErr != nil || saved || entry != (RecordedActivity{}) {
				t.Fatalf("subsecond Stop() = (%+v, %v, %v), want successful stop without activity", entry, saved, stopErr)
			}
		})
	}

	entry, saved, err := timer.Stop("activity-1", startedAt.Add(time.Second), startedAt.Add(time.Second))
	if err != nil || !saved || entry.DurationSeconds() != 1 {
		t.Fatalf("one-second Stop() = (%+v, %v, %v), want one-second activity", entry, saved, err)
	}
	if _, _, err := timer.Stop("activity-1", startedAt.Add(-time.Nanosecond), startedAt); err == nil {
		t.Fatal("Stop() accepted an end before the timer start")
	}
}

func TestTimerRetainsStartingTimeZoneWhenStopped(t *testing.T) {
	startedAt := time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC)
	timer, err := StartTimer("timer-1", "path-1", "participant-1", startedAt, "America/New_York", startedAt)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	entry, saved, err := timer.Stop("activity-1", startedAt.Add(2*time.Hour), startedAt.Add(2*time.Hour))
	if err != nil || !saved {
		t.Fatalf("Stop() = (%+v, %v, %v), want saved activity", entry, saved, err)
	}
	if entry.OccurrenceTimeZone != "America/New_York" {
		t.Fatalf("occurrence time zone = %q, want timer's starting zone", entry.OccurrenceTimeZone)
	}
	if got := entry.DurationSeconds(); got != int64((2*time.Hour)/time.Second) {
		t.Fatalf("DurationSeconds() across offset transition = %v, want UTC elapsed time", got)
	}
}

func TestStartTimerRejectsInvalidCanonicalState(t *testing.T) {
	now := time.Date(2026, time.July, 21, 21, 30, 0, 0, time.UTC)
	tests := map[string]struct {
		id, pathID, participantID string
		startedAt, currentInstant time.Time
		timeZone                  string
	}{
		"missing timer identity":        {pathID: "path", participantID: "participant", startedAt: now, timeZone: "Etc/UTC", currentInstant: now},
		"missing Path":                  {id: "timer", participantID: "participant", startedAt: now, timeZone: "Etc/UTC", currentInstant: now},
		"missing participant":           {id: "timer", pathID: "path", startedAt: now, timeZone: "Etc/UTC", currentInstant: now},
		"missing start":                 {id: "timer", pathID: "path", participantID: "participant", timeZone: "Etc/UTC", currentInstant: now},
		"future start":                  {id: "timer", pathID: "path", participantID: "participant", startedAt: now.Add(time.Second), timeZone: "Etc/UTC", currentInstant: now},
		"fixed-offset zone":             {id: "timer", pathID: "path", participantID: "participant", startedAt: now, timeZone: "UTC-04:00", currentInstant: now},
		"process-local zone":            {id: "timer", pathID: "path", participantID: "participant", startedAt: now, timeZone: "Local", currentInstant: now},
		"unknown zone":                  {id: "timer", pathID: "path", participantID: "participant", startedAt: now, timeZone: "Mars/Olympus_Mons", currentInstant: now},
		"missing current-time evidence": {id: "timer", pathID: "path", participantID: "participant", startedAt: now, timeZone: "Etc/UTC"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := StartTimer(test.id, test.pathID, test.participantID, test.startedAt, test.timeZone, test.currentInstant); err == nil {
				t.Fatal("StartTimer() accepted invalid canonical state")
			}
		})
	}
}

func TestStopRejectsInvalidCompletedActivity(t *testing.T) {
	startedAt := time.Date(2026, time.July, 21, 21, 30, 0, 0, time.UTC)
	timer, err := StartTimer("timer", "path", "participant", startedAt, "Etc/UTC", startedAt)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	tests := map[string]struct {
		activityID string
		stoppedAt  time.Time
		recordedAt time.Time
	}{
		"missing activity identity": {stoppedAt: startedAt.Add(time.Second), recordedAt: startedAt.Add(time.Second)},
		"stop before start":         {activityID: "activity", stoppedAt: startedAt.Add(-time.Second), recordedAt: startedAt},
		"stop later than current":   {activityID: "activity", stoppedAt: startedAt.Add(2 * time.Second), recordedAt: startedAt.Add(time.Second)},
		"missing stop":              {activityID: "activity", recordedAt: startedAt.Add(time.Second)},
		"missing recorded instant":  {activityID: "activity", stoppedAt: startedAt.Add(time.Second)},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := timer.Stop(test.activityID, test.stoppedAt, test.recordedAt); err == nil {
				t.Fatal("Stop() accepted invalid completed activity state")
			}
		})
	}
}
