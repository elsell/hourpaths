package stats

import (
	"fmt"
	"sort"
	"time"
)

type StatsPath struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}
type Record struct {
	PathID             string
	StartedAt, EndedAt time.Time
	TimeZone           string
}
type StatsDistribution struct {
	PathID  string `json:"pathId"`
	Name    string `json:"name"`
	Seconds int64  `json:"seconds"`
}
type StatsBucket struct {
	Key     string `json:"key"`
	Seconds int64  `json:"seconds"`
}
type StatsCalendarDay struct {
	Date    string `json:"date"`
	Seconds int64  `json:"seconds"`
}
type StatsSummary struct {
	WeekStartsOn   int                 `json:"weekStartsOn"`
	Range          string              `json:"range"`
	Anchor         string              `json:"anchor"`
	StartDate      string              `json:"startDate"`
	EndDate        string              `json:"endDate"`
	PreviousAnchor string              `json:"previousAnchor,omitempty"`
	NextAnchor     string              `json:"nextAnchor,omitempty"`
	BucketUnit     string              `json:"bucketUnit"`
	TotalSeconds   int64               `json:"totalSeconds"`
	Distribution   []StatsDistribution `json:"distribution" nullable:"false"`
	Buckets        []StatsBucket       `json:"buckets" nullable:"false"`
	Calendar       []StatsCalendarDay  `json:"calendar" nullable:"false"`
	AvailablePaths []StatsPath         `json:"availablePaths" nullable:"false"`
}

func date(t time.Time) string { return t.Format("2006-01-02") }
func period(t time.Time, kind string, weekday int) (time.Time, time.Time) {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	switch kind {
	case "day":
		return t, t.AddDate(0, 0, 1)
	case "week":
		t = t.AddDate(0, 0, -(int(t.Weekday())-weekday+7)%7)
		return t, t.AddDate(0, 0, 7)
	case "month":
		t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		return t, t.AddDate(0, 1, 0)
	default:
		t = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return t, t.AddDate(1, 0, 0)
	}
}
func Aggregate(kind, anchor string, paths []StatsPath, records []Record, now time.Time, weekday int) (StatsSummary, error) {
	s := StatsSummary{WeekStartsOn: weekday, Range: kind, AvailablePaths: paths, Distribution: []StatsDistribution{}, Buckets: []StatsBucket{}, Calendar: []StatsCalendarDay{}}
	if s.AvailablePaths == nil {
		s.AvailablePaths = []StatsPath{}
	}
	if kind != "day" && kind != "week" && kind != "month" && kind != "year" && kind != "all_time" {
		return s, fmt.Errorf("invalid range")
	}
	if anchor == "" {
		anchor = date(now)
	}
	a, err := time.Parse("2006-01-02", anchor)
	if err != nil {
		return s, err
	}
	current, _ := time.Parse("2006-01-02", date(now))
	if a.After(current) {
		return s, fmt.Errorf("future period")
	}
	start, end := period(a, kind, weekday%7)
	if kind == "all_time" {
		start = current
		end = current.AddDate(0, 0, 1)
		for _, r := range records {
			loc, e := time.LoadLocation(r.TimeZone)
			if e != nil {
				return s, e
			}
			d, _ := time.Parse("2006-01-02", date(r.StartedAt.In(loc)))
			if d.Before(start) {
				start = d
			}
			last, _ := time.Parse("2006-01-02", date(r.EndedAt.Add(-time.Nanosecond).In(loc)))
			if !last.Before(end) {
				end = last.AddDate(0, 0, 1)
			}
		}
	} else {
		prev, _ := period(start.AddDate(0, 0, -1), kind, weekday%7)
		s.PreviousAnchor = date(prev)
		currentStart, _ := period(current, kind, weekday%7)
		if end.Before(currentStart) || end.Equal(currentStart) {
			s.NextAnchor = date(end)
		}
	}
	s.Anchor = date(start)
	s.StartDate = date(start)
	s.EndDate = date(end.AddDate(0, 0, -1))
	unit := "day"
	switch kind {
	case "day":
		unit = "hour"
	case "year":
		unit = "month"
	case "all_time":
		if end.Year()-start.Year() >= 2 {
			unit = "year"
		} else if (end.Year()-start.Year())*12+int(end.Month()-start.Month()) >= 2 {
			unit = "month"
		}
	}
	if kind == "all_time" && end.Sub(start) == 24*time.Hour {
		unit = "hour"
	}
	s.BucketUnit = unit
	daily := map[string]int64{}
	buckets := map[string]int64{}
	totals := map[string]int64{}
	for _, r := range records {
		loc, e := time.LoadLocation(r.TimeZone)
		if e != nil {
			return s, e
		}
		if !r.EndedAt.After(r.StartedAt) {
			return s, fmt.Errorf("invalid recorded interval")
		}
		// Compare calendar boundaries in this record's historical zone, never in
		// the viewer's current zone. Clip before iterating so a Day request
		// does not walk every hour of unrelated historical activity.
		rangeStart := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
		rangeEnd := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, loc)
		cursor := r.StartedAt
		if cursor.Before(rangeStart) {
			cursor = rangeStart
		}
		recordEnd := r.EndedAt
		if recordEnd.After(rangeEnd) {
			recordEnd = rangeEnd
		}
		for cursor.Before(recordEnd) {
			local := cursor.In(loc)
			next := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
			if unit == "hour" {
				next = cursor.Add(time.Hour - time.Duration(local.Minute())*time.Minute - time.Duration(local.Second())*time.Second - time.Duration(local.Nanosecond()))
			}
			if next.After(recordEnd) {
				next = recordEnd
			}
			if !next.After(cursor) {
				return s, fmt.Errorf("invalid calendar boundary")
			}
			d := date(local)
			if d >= s.StartDate && d <= s.EndDate {
				seconds := int64(next.Sub(r.StartedAt)/time.Second) - int64(cursor.Sub(r.StartedAt)/time.Second)
				daily[d] += seconds
				key := d
				switch unit {
				case "hour":
					key = local.Format("2006-01-02T15")
				case "month":
					key = local.Format("2006-01")
				case "year":
					key = local.Format("2006")
				}
				buckets[key] += seconds
				totals[r.PathID] += seconds
				s.TotalSeconds += seconds
			}
			cursor = next
		}
	}
	for _, p := range paths {
		if n := totals[p.ID]; n > 0 {
			s.Distribution = append(s.Distribution, StatsDistribution{p.ID, p.Name, n})
		}
	}
	sort.Slice(s.Distribution, func(i, j int) bool {
		if s.Distribution[i].Seconds == s.Distribution[j].Seconds {
			return s.Distribution[i].PathID < s.Distribution[j].PathID
		}
		return s.Distribution[i].Seconds > s.Distribution[j].Seconds
	})
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		s.Calendar = append(s.Calendar, StatsCalendarDay{date(d), daily[date(d)]})
	}
	if unit == "hour" {
		for h := 0; h < 24; h++ {
			key := fmt.Sprintf("%sT%02d", s.StartDate, h)
			s.Buckets = append(s.Buckets, StatsBucket{key, buckets[key]})
		}
	} else {
		seen := map[string]bool{}
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			key := date(d)
			if unit == "month" {
				key = d.Format("2006-01")
			}
			if unit == "year" {
				key = d.Format("2006")
			}
			if !seen[key] {
				s.Buckets = append(s.Buckets, StatsBucket{key, buckets[key]})
				seen[key] = true
			}
		}
	}
	return s, nil
}
