package activity

import (
	"testing"
	"time"
)

func TestLongTimerThresholdUsesExactCurrentAverage(t *testing.T) {
	start := time.Date(2026, 10, 9, 12, 0, 0, 250_000_000, time.UTC)
	timer, err := StartTimer("timer", "path", "owner", start, "UTC", start)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		count, total int64
		elapsed      time.Duration
		due          bool
	}{
		{"before threshold", 3, 360, 180*time.Second - time.Nanosecond, false},
		{"at threshold", 3, 360, 180 * time.Second, true},
		{"fractional mean not rounded down", 3, 5, 2500*time.Millisecond - time.Nanosecond, false},
		{"fractional threshold reached", 3, 5, 2500 * time.Millisecond, true},
		{"edited duration raises threshold", 3, 540, 180 * time.Second, false},
		{"edited threshold reached", 3, 540, 270 * time.Second, true},
		{"deleted session leaves too few", 2, 180, time.Hour, false},
		{"no history", 0, 0, time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			due, err := timer.LongTimerNoticeDue(tc.count, tc.total, start.Add(tc.elapsed))
			if err != nil || due != tc.due {
				t.Fatalf("due=%v error=%v; want %v", due, err, tc.due)
			}
		})
	}
}
