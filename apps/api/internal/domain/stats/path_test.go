package stats

import (
	"testing"
	"time"
)

func TestPathStatisticsCompleteHistoryAndOccurrenceCalendar(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 10, 10, 3, 30, 0, 0, time.UTC)
	got, err := AggregatePath([]Record{
		{PathID: "p", StartedAt: old, EndedAt: old.Add(time.Hour), TimeZone: "UTC"},
		{PathID: "p", StartedAt: start, EndedAt: start.Add(time.Hour), TimeZone: "America/New_York"},
	}, now, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalSeconds != 7200 || got.SessionCount != 2 || got.AverageSeconds != 3600 {
		t.Fatalf("summary: %+v", got)
	}
	if got.StartDate != "2025-10-11" || got.EndDate != "2026-10-10" || len(got.Calendar) != 365 {
		t.Fatalf("range: %+v", got)
	}
	if got.Calendar[0].Seconds != 0 || got.Calendar[363].Seconds != 1800 || got.Calendar[364].Seconds != 1800 {
		t.Fatalf("wrong occurrence dates: %+v", got.Calendar[363:])
	}
	var total int64
	for _, day := range got.Calendar {
		total += day.Seconds
	}
	if total != 3600 {
		t.Fatalf("calendar includes out-of-range history: %d", total)
	}
}

func TestPathStatisticsEmptyLeapRange(t *testing.T) {
	now := time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC)
	got, err := AggregatePath(nil, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.StartDate != "2023-03-01" || got.EndDate != "2024-02-29" || len(got.Calendar) != 366 || got.WeekStartsOn != 7 {
		t.Fatalf("range: %+v", got)
	}
	if got.TotalSeconds != 0 || got.SessionCount != 0 || got.AverageSeconds != 0 {
		t.Fatal("empty statistics must be zero")
	}
	for _, day := range got.Calendar {
		if day.Seconds != 0 {
			t.Fatalf("nonzero empty date: %+v", day)
		}
	}
}
