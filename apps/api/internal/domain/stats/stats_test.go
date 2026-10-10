package stats

import (
	"testing"
	"time"
)

func TestHistoricalCalendarConservesElapsedAcrossDSTAndMidnight(t *testing.T) {
	paths := []StatsPath{{ID: "p", Name: "Piano"}}
	for _, test := range []struct {
		name, start, end, anchor string
		total                    int64
		hour                     int
		hourSeconds              int64
	}{
		{"spring", "2026-03-08T05:00:00Z", "2026-03-09T04:00:00Z", "2026-03-08", 23 * 3600, 2, 0},
		{"fall", "2026-11-01T04:00:00Z", "2026-11-02T05:00:00Z", "2026-11-01", 25 * 3600, 1, 7200},
	} {
		t.Run(test.name, func(t *testing.T) {
			start, _ := time.Parse(time.RFC3339, test.start)
			end, _ := time.Parse(time.RFC3339, test.end)
			s, e := Aggregate("day", test.anchor, paths, []Record{{"p", start, end, "America/New_York"}}, end, 1)
			if e != nil || s.TotalSeconds != test.total || s.Buckets[test.hour].Seconds != test.hourSeconds {
				t.Fatalf("%+v %v", s, e)
			}
			var sum int64
			for _, b := range s.Buckets {
				sum += b.Seconds
			}
			if sum != test.total {
				t.Fatal(sum)
			}
		})
	}
	start := time.Date(2026, 1, 2, 4, 59, 59, 500000000, time.UTC)
	end := start.Add(2 * time.Second)
	s, e := Aggregate("month", "2026-01-10", paths, []Record{{"p", start, end, "America/New_York"}}, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), 1)
	if e != nil || s.TotalSeconds != 2 || s.Calendar[0].Seconds+s.Calendar[1].Seconds != 2 || s.Calendar[1].Seconds != 2 {
		t.Fatalf("fractional conservation %+v %v", s, e)
	}
}
func TestPeriodNavigationAndEmptyRange(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	s, e := Aggregate("week", "", nil, nil, now, 1)
	if e != nil || s.StartDate != "2026-09-21" || s.EndDate != "2026-09-27" || s.NextAnchor != "" || len(s.Calendar) != 7 || len(s.Buckets) != 7 || s.TotalSeconds != 0 {
		t.Fatalf("%+v %v", s, e)
	}
	s, e = Aggregate("week", "2026-09-20", nil, nil, now, 7)
	if e != nil || s.StartDate != "2026-09-20" || s.NextAnchor != "2026-09-27" {
		t.Fatalf("%+v %v", s, e)
	}
}

func TestCurrentCalendarEndsOnParticipantToday(t *testing.T) {
	// The application supplies now in the participant's configured time zone.
	instant := time.Date(2026, 10, 10, 0, 30, 0, 0, time.UTC)
	for _, zone := range []string{"America/Los_Angeles", "Pacific/Auckland"} {
		loc, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatal(err)
		}
		now := instant.In(loc)
		for _, kind := range []string{"day", "week", "month", "year"} {
			t.Run(zone+"/"+kind, func(t *testing.T) {
				got, err := Aggregate(kind, "", nil, nil, now, 1)
				if err != nil {
					t.Fatal(err)
				}
				today := now.Format("2006-01-02")
				if got.EndDate != today || len(got.Calendar) == 0 || got.Calendar[len(got.Calendar)-1].Date != today {
					t.Fatalf("current %s extends past local today %s: end=%s", kind, today, got.EndDate)
				}
				first, _ := time.Parse("2006-01-02", got.StartDate)
				last, _ := time.Parse("2006-01-02", today)
				if len(got.Calendar) != int(last.Sub(first)/(24*time.Hour))+1 {
					t.Fatal("missing zero-activity dates")
				}
				for _, day := range got.Calendar {
					if day.Seconds != 0 {
						t.Fatal("fabricated activity")
					}
				}
				if got.NextAnchor != "" {
					t.Fatal("current range permits future navigation")
				}
			})
		}
		past, err := Aggregate("month", "2026-09-01", nil, nil, now, 1)
		if err != nil || past.StartDate != "2026-09-01" || past.EndDate != "2026-09-30" || len(past.Calendar) != 30 || past.NextAnchor != "2026-10-01" {
			t.Fatalf("past period changed: %+v %v", past, err)
		}
	}
}
