package stats

import "time"

// PathSummary contains completed activity only. Lifetime metrics and the
// trailing calendar are deliberately independent of recent-session pagination.
type PathSummary struct {
	TotalSeconds   int64              `json:"totalSeconds"`
	SessionCount   int64              `json:"sessionCount"`
	AverageSeconds int64              `json:"averageSeconds"`
	WeekStartsOn   int                `json:"weekStartsOn"`
	StartDate      string             `json:"startDate"`
	EndDate        string             `json:"endDate"`
	Calendar       []StatsCalendarDay `json:"calendar" nullable:"false"`
}

func AggregatePath(records []Record, now time.Time, weekday int) (PathSummary, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	anniversaryDay := today.Day()
	last := time.Date(today.Year()-1, today.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if anniversaryDay > last {
		anniversaryDay = last
	}
	first := time.Date(today.Year()-1, today.Month(), anniversaryDay, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	result := PathSummary{WeekStartsOn: weekday, StartDate: date(first), EndDate: date(today), Calendar: []StatsCalendarDay{}}
	// Reuse Stats' occurrence-zone/DST allocation rather than introducing a
	// second interpretation of historical dates for Path details.
	for year := first.Year(); year <= today.Year(); year++ {
		anchor := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		summary, err := Aggregate("year", date(anchor), nil, records, now, weekday)
		if err != nil {
			return PathSummary{}, err
		}
		for _, day := range summary.Calendar {
			if day.Date >= result.StartDate && day.Date <= result.EndDate {
				result.Calendar = append(result.Calendar, day)
			}
		}
	}
	for _, record := range records {
		result.TotalSeconds += int64(record.EndedAt.Sub(record.StartedAt) / time.Second)
	}
	result.SessionCount = int64(len(records))
	if result.SessionCount > 0 {
		result.AverageSeconds = result.TotalSeconds / result.SessionCount
	}
	return result, nil
}
