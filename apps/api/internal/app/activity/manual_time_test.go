package activity

import (
	"testing"
	"time"
)

func TestManualStartInstantUsesParticipantTimeZone(t *testing.T) {
	got, err := manualStartInstant("2026-07-22", "09:30:15", "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.July, 22, 13, 30, 15, 0, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("manualStartInstant() = %v, want %v UTC", got, want)
	}
}

func TestManualStartInstantChoosesEarlierRepeatedWallTime(t *testing.T) {
	got, err := manualStartInstant("2026-11-01", "01:30:00", "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("manualStartInstant() = %v, want earlier occurrence %v", got, want)
	}
}

func TestManualStartInstantMovesForwardThroughGap(t *testing.T) {
	got, err := manualStartInstant("2026-03-08", "02:30:00", "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.March, 8, 7, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("manualStartInstant() = %v, want gap-adjusted %v", got, want)
	}
}

func TestManualStartInstantRejectsMalformedOrNonIANAInput(t *testing.T) {
	for name, input := range map[string][3]string{
		"date":       {"07/22/2026", "09:30:00", "America/New_York"},
		"time":       {"2026-07-22", "09:30", "America/New_York"},
		"zone":       {"2026-07-22", "09:30:00", "Local"},
		"whitespace": {"2026-07-22", "09:30:00", " America/New_York"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := manualStartInstant(input[0], input[1], input[2]); err == nil {
				t.Fatal("manualStartInstant() accepted invalid input")
			}
		})
	}
}
