package preferences

import (
	"testing"
	"time"
)

func TestUnavailablePeriodUsesLocalDailyWindow(t *testing.T) {
	for _, c := range []struct {
		start, end, at int
		want           bool
	}{
		{1320, 480, 1320, true}, {1320, 480, 479, true}, {1320, 480, 480, false}, {1320, 480, 1319, false},
		{540, 1020, 540, true}, {540, 1020, 1019, true}, {540, 1020, 1020, false},
	} {
		local := time.Date(2026, 10, 9, c.at/60, c.at%60, 0, 0, time.UTC)
		period := UnavailablePeriod{Enabled: true, StartMinute: c.start, EndMinute: c.end}
		if period.ContainsLocal(local) != c.want {
			t.Fatalf("%+v at %v", period, local)
		}
		period.Enabled = false
		if period.ContainsLocal(local) {
			t.Fatal("disabled interval suppressed delivery")
		}
	}
}

func TestUnavailablePeriodRepeatsAndSkipsWithConfiguredZone(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		instant    string
		start, end int
		want       bool
	}{
		{"2026-11-01T05:45:00Z", 90, 150, true},
		{"2026-11-01T06:45:00Z", 90, 150, true},
		{"2026-11-01T07:30:00Z", 90, 150, false},
		{"2026-03-08T07:00:00Z", 150, 210, true},
		{"2026-03-08T07:30:00Z", 150, 210, false},
	} {
		instant, err := time.Parse(time.RFC3339, c.instant)
		if err != nil {
			t.Fatal(err)
		}
		p := UnavailablePeriod{Enabled: true, StartMinute: c.start, EndMinute: c.end}
		if p.ContainsLocal(instant.In(zone)) != c.want {
			t.Fatalf("%s local=%v", c.instant, instant.In(zone))
		}
	}
}

func TestUnavailablePeriodNextStartFollowsWallClockTransitions(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		at, want   string
		start, end int
	}{
		{"2026-10-09T20:00:00Z", "2026-10-10T02:00:00Z", 1320, 480},
		{"2026-03-08T06:00:00Z", "2026-03-08T07:00:00Z", 150, 210},
		{"2026-11-01T06:15:00Z", "2026-11-01T06:30:00Z", 90, 150},
		{"2026-10-10T02:00:00Z", "2026-10-10T02:00:00Z", 1320, 480},
	} {
		at, _ := time.Parse(time.RFC3339, c.at)
		want, _ := time.Parse(time.RFC3339, c.want)
		period := UnavailablePeriod{Enabled: true, StartMinute: c.start, EndMinute: c.end}
		got, ok := period.NextStart(at.In(zone))
		if !ok || !got.Equal(want) {
			t.Fatalf("%s: next start=%v found=%v want=%v", c.at, got, ok, want)
		}
		period.Enabled = false
		if _, ok := period.NextStart(at.In(zone)); ok {
			t.Fatal("disabled period supplied a deadline")
		}
	}
}
