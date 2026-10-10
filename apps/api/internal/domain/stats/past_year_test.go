package stats

import (
	"testing"
	"time"
)

func TestPastYearCalendarIncludesEmptyDatesAndMatchesPathGrid(t *testing.T) {
	for _, stamp := range []string{"2026-10-10", "2024-02-29", "2025-02-28"} {
		now, _ := time.Parse("2006-01-02", stamp)
		summary, err := Aggregate("past_year", "", nil, nil, now, 7)
		if err != nil {
			t.Fatal(err)
		}
		path, err := AggregatePath(nil, now, 7)
		if err != nil {
			t.Fatal(err)
		}
		if summary.StartDate != path.StartDate || summary.EndDate != stamp || len(summary.Calendar) != len(path.Calendar) || len(summary.Calendar) < 365 {
			t.Fatalf("incomplete year: %+v", summary)
		}
		if stamp == "2026-10-10" && (summary.StartDate != "2025-10-11" || len(summary.Calendar) != 365) {
			t.Fatal(summary.StartDate, len(summary.Calendar))
		}
		for _, day := range summary.Calendar {
			if day.Seconds != 0 {
				t.Fatal(day)
			}
		}
	}
}

func TestPastYearCalendarCountsHistoryOutsideChartWeek(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	start := time.Date(2025, 12, 1, 12, 0, 0, 0, time.UTC)
	records := []Record{{PathID: "p", StartedAt: start, EndedAt: start.Add(time.Hour), TimeZone: "UTC"}}
	week, err := Aggregate("week", "", nil, records, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	calendar, err := Aggregate("past_year", "", nil, records, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if week.TotalSeconds != 0 || calendar.TotalSeconds != 3600 || len(calendar.Calendar) != 365 {
		t.Fatal(week.TotalSeconds, calendar.TotalSeconds, len(calendar.Calendar))
	}
	historical, err := Aggregate("past_year", "2025-10-10", nil, nil, now, 7)
	if err != nil || historical.EndDate >= calendar.StartDate || historical.NextAnchor != "" {
		t.Fatal(historical, err)
	}
}
