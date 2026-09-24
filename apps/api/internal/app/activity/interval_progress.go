package activity

import (
	"errors"
	"sort"
	"strings"
	"time"

	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

var errInvalidIntervalWindow = errors.New("interval window is invalid")

// IntervalWindow is a half-open pair of UTC instants: StartedAt is included
// and EndedAt is excluded.
type IntervalWindow struct {
	StartedAt time.Time
	EndedAt   time.Time
}

func currentIntervalWindow(goal pathdomain.IntervalGoal, participantTimeZone string, now time.Time) (IntervalWindow, error) {
	if !validIntervalGoal(goal) || now.IsZero() || strings.TrimSpace(participantTimeZone) != participantTimeZone || participantTimeZone == "" || participantTimeZone == "Local" {
		return IntervalWindow{}, errInvalidIntervalWindow
	}
	location, err := time.LoadLocation(participantTimeZone)
	if err != nil {
		return IntervalWindow{}, errInvalidIntervalWindow
	}

	localNow := now.In(location)
	walls := intervalBoundaryWalls(goal, localNow)
	instants := make([]time.Time, 0, len(walls))
	seen := make(map[int64]struct{}, len(walls))
	for _, wall := range walls {
		instant, ok := resolveLocalWallInstant(wall, location)
		if !ok {
			return IntervalWindow{}, errInvalidIntervalWindow
		}
		key := instant.UnixNano()
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		instants = append(instants, instant)
	}
	sort.Slice(instants, func(i, j int) bool { return instants[i].Before(instants[j]) })

	current := now.UTC()
	startIndex := -1
	for index, instant := range instants {
		if instant.After(current) {
			if startIndex < 0 {
				return IntervalWindow{}, errInvalidIntervalWindow
			}
			return IntervalWindow{StartedAt: instants[startIndex], EndedAt: instant}, nil
		}
		startIndex = index
	}
	return IntervalWindow{}, errInvalidIntervalWindow
}

// CurrentIntervalWindow returns the canonical participant-calendar interval
// containing now. Adapters use it when an atomic mutation must return the same
// projection as the activity application service.
func CurrentIntervalWindow(goal pathdomain.IntervalGoal, participantTimeZone string, now time.Time) (IntervalWindow, error) {
	return currentIntervalWindow(goal, participantTimeZone, now)
}

func intervalBoundaryWalls(goal pathdomain.IntervalGoal, localNow time.Time) []time.Time {
	const radius = 3
	walls := make([]time.Time, 0, radius*2+1)
	switch goal.Recurrence {
	case pathdomain.RecurrenceHourly:
		base := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), localNow.Hour(), goal.Alignment.Minute, 0, 0, time.UTC)
		for offset := -radius; offset <= radius; offset++ {
			walls = append(walls, base.Add(time.Duration(offset)*time.Hour))
		}
	case pathdomain.RecurrenceDaily:
		base := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), goal.Alignment.Hour, 0, 0, 0, time.UTC)
		for offset := -radius; offset <= radius; offset++ {
			walls = append(walls, base.AddDate(0, 0, offset))
		}
	case pathdomain.RecurrenceWeekly:
		base := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
		weekday := int(localNow.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		base = base.AddDate(0, 0, goal.Alignment.ISOWeekday-weekday)
		for offset := -radius; offset <= radius; offset++ {
			walls = append(walls, base.AddDate(0, 0, offset*7))
		}
	case pathdomain.RecurrenceMonthly:
		for offset := -radius; offset <= radius; offset++ {
			month := time.Date(localNow.Year(), localNow.Month()+time.Month(offset), 1, 0, 0, 0, 0, time.UTC)
			walls = append(walls, time.Date(month.Year(), month.Month(), min(goal.Alignment.Day, daysInMonth(month.Year(), month.Month())), 0, 0, 0, 0, time.UTC))
		}
	case pathdomain.RecurrenceYearly:
		for offset := -radius; offset <= radius; offset++ {
			year := localNow.Year() + offset
			day := goal.Alignment.Day
			if goal.Alignment.Month == int(time.February) && day == 29 && daysInMonth(year, time.February) == 28 {
				day = 28
			}
			walls = append(walls, time.Date(year, time.Month(goal.Alignment.Month), day, 0, 0, 0, 0, time.UTC))
		}
	}
	return walls
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func validIntervalGoal(goal pathdomain.IntervalGoal) bool {
	if !goal.Present || goal.TargetSeconds <= 0 {
		return false
	}
	alignment := goal.Alignment
	switch goal.Recurrence {
	case pathdomain.RecurrenceHourly:
		return alignment.Minute >= 0 && alignment.Minute <= 59 && alignment.Hour == 0 && alignment.ISOWeekday == 0 && alignment.Month == 0 && alignment.Day == 0
	case pathdomain.RecurrenceDaily:
		return alignment.Hour >= 0 && alignment.Hour <= 23 && alignment.Minute == 0 && alignment.ISOWeekday == 0 && alignment.Month == 0 && alignment.Day == 0
	case pathdomain.RecurrenceWeekly:
		return alignment.ISOWeekday >= 1 && alignment.ISOWeekday <= 7 && alignment.Minute == 0 && alignment.Hour == 0 && alignment.Month == 0 && alignment.Day == 0
	case pathdomain.RecurrenceMonthly:
		return alignment.Day >= 1 && alignment.Day <= 31 && alignment.Minute == 0 && alignment.Hour == 0 && alignment.ISOWeekday == 0 && alignment.Month == 0
	case pathdomain.RecurrenceYearly:
		if alignment.Minute != 0 || alignment.Hour != 0 || alignment.ISOWeekday != 0 || alignment.Month < 1 || alignment.Month > 12 || alignment.Day < 1 {
			return false
		}
		date := time.Date(2000, time.Month(alignment.Month), alignment.Day, 0, 0, 0, 0, time.UTC)
		return int(date.Month()) == alignment.Month && date.Day() == alignment.Day
	default:
		return false
	}
}
