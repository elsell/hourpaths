package preferences

import "time"

// UnavailablePeriod is a daily wall-clock interval. Boundary adapters must
// validate minute ranges; equal-endpoint policy is not yet exposed.
type UnavailablePeriod struct {
	Enabled                bool
	StartMinute, EndMinute int
}

// ContainsLocal evaluates an instant already converted to the account's
// configured time zone. The start is included and the end is excluded.
func (p UnavailablePeriod) ContainsLocal(local time.Time) bool {
	if !p.Enabled {
		return false
	}
	minute := local.Hour()*60 + local.Minute()
	if p.StartMinute < p.EndMinute {
		return minute >= p.StartMinute && minute < p.EndMinute
	}
	if p.StartMinute > p.EndMinute {
		return minute >= p.StartMinute || minute < p.EndMinute
	}
	return false // Equal endpoints are not an admitted configuration.
}

// NextStart returns the next actual transition into the local daily period,
// including a transition exactly at local. Walking real minutes preserves both
// occurrences of a repeated wall-clock minute and skips nonexistent minutes.
func (p UnavailablePeriod) NextStart(local time.Time) (time.Time, bool) {
	if !p.Enabled || local.IsZero() || p.StartMinute < 0 || p.StartMinute >= 1440 || p.EndMinute < 0 || p.EndMinute >= 1440 || p.StartMinute == p.EndMinute {
		return time.Time{}, false
	}
	minute := local.Truncate(time.Minute)
	if minute.Before(local) {
		minute = minute.Add(time.Minute)
	}
	// Three real days cover the next recurrence even across a skipped local day.
	for end := minute.Add(72 * time.Hour); minute.Before(end); minute = minute.Add(time.Minute) {
		if p.ContainsLocal(minute) && !p.ContainsLocal(minute.Add(-time.Minute)) {
			return minute, true
		}
	}
	return time.Time{}, false
}
