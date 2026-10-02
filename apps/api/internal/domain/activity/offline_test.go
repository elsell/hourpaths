package activity

import (
	"testing"
	"time"
)

func TestOfflineTimerConflictUsesOriginalStartAndStableTieBreaker(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	older, _ := StartTimer("a", "path", "owner", now.Add(-time.Hour), "Etc/UTC", now)
	newer, _ := StartTimer("b", "path", "owner", now.Add(-time.Minute), "Etc/UTC", now)
	for _, pair := range [][2]RunningTimer{{older, newer}, {newer, older}} {
		winner, err := OfflineTimerWinner(pair[0], pair[1])
		if err != nil || winner.ID != newer.ID {
			t.Fatalf("winner=%+v err=%v", winner, err)
		}
	}
	tied := newer
	tied.ID = "c"
	winner, err := OfflineTimerWinner(tied, newer)
	if err != nil || winner.ID != "c" {
		t.Fatalf("tie winner=%+v err=%v", winner, err)
	}
	foreign := newer
	foreign.ParticipantID = "someone-else"
	if _, err := OfflineTimerWinner(older, foreign); err == nil {
		t.Fatal("cross-owner conflict accepted")
	}
}

func TestOfflineStopPreservesOnlyTimeBeforeArchival(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	archive := now.Add(-5 * time.Minute)
	timer, _ := StartTimer("timer", "path", "owner", now.Add(-10*time.Minute), "America/New_York", now)
	result, err := ResolveOfflineStop(timer, "entry", now.Add(-time.Minute), now, &archive)
	if err != nil || !result.Saved || result.Activity.DurationSeconds() != 300 || result.DiscardedSeconds != 240 || !result.Archived {
		t.Fatalf("split=%+v err=%v", result, err)
	}
	if result.Activity.OccurrenceTimeZone != "America/New_York" {
		t.Fatal("occurrence zone changed")
	}
	result, err = ResolveOfflineStop(timer, "entry", archive, now, &archive)
	if err != nil || result.DiscardedSeconds != 0 || !result.Saved {
		t.Fatalf("boundary=%+v err=%v", result, err)
	}
	late, _ := StartTimer("late", "path", "owner", archive, "Etc/UTC", now)
	result, err = ResolveOfflineStop(late, "entry", now, now, &archive)
	if err != nil || result.Saved || result.DiscardedSeconds != 300 {
		t.Fatalf("late=%+v err=%v", result, err)
	}
	if _, err = ResolveOfflineStop(timer, "entry", timer.StartedAt, now, nil); err == nil {
		t.Fatal("invalid device clock accepted")
	}
}
