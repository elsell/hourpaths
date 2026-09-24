package activity

import (
	"errors"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
)

var errInvalidManualOccurrence = errors.New("manual activity occurrence is invalid")

// manualStartInstant resolves a second-precision participant-local wall time.
// Repeated wall times select the earlier instant; nonexistent wall times retain
// their minute and second while moving forward by the clock-transition gap.
func manualStartInstant(localDate, localTime, timeZone string) (time.Time, error) {
	if strings.TrimSpace(localDate) != localDate || strings.TrimSpace(localTime) != localTime ||
		strings.TrimSpace(timeZone) != timeZone || timeZone == "" || timeZone == "Local" {
		return time.Time{}, errInvalidManualOccurrence
	}
	wall, err := time.Parse("2006-01-02 15:04:05", localDate+" "+localTime)
	if err != nil || wall.Format("2006-01-02 15:04:05") != localDate+" "+localTime {
		return time.Time{}, errInvalidManualOccurrence
	}
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return time.Time{}, errInvalidManualOccurrence
	}
	instant, ok := resolveLocalWallInstant(wall, location)
	if !ok {
		return time.Time{}, errInvalidManualOccurrence
	}
	return instant, nil
}

// resolveLocalWallInstant interprets wall as calendar fields rather than as a
// UTC instant. It deliberately resolves overlaps to the earlier instant and
// gaps by advancing the wall time by the transition gap.
func resolveLocalWallInstant(wall time.Time, location *time.Location) (time.Time, bool) {
	if wall.IsZero() || location == nil {
		return time.Time{}, false
	}
	offsets := map[int]struct{}{}
	for sample := wall.Add(-72 * time.Hour); !sample.After(wall.Add(72 * time.Hour)); sample = sample.Add(time.Hour) {
		_, offset := sample.In(location).Zone()
		offsets[offset] = struct{}{}
	}
	type resolution struct {
		instant time.Time
		wall    time.Time
	}
	var exact, forward []resolution
	for offset := range offsets {
		instant := wall.Add(-time.Duration(offset) * time.Second).UTC()
		localized := instant.In(location)
		candidateWall := time.Date(localized.Year(), localized.Month(), localized.Day(), localized.Hour(), localized.Minute(), localized.Second(), 0, time.UTC)
		candidate := resolution{instant: instant, wall: candidateWall}
		if candidateWall.Equal(wall) {
			exact = append(exact, candidate)
		} else if candidateWall.After(wall) {
			forward = append(forward, candidate)
		}
	}
	if len(exact) > 0 {
		sort.Slice(exact, func(i, j int) bool { return exact[i].instant.Before(exact[j].instant) })
		return exact[0].instant, true
	}
	if len(forward) == 0 {
		return time.Time{}, false
	}
	sort.Slice(forward, func(i, j int) bool {
		left, right := forward[i].wall.Sub(wall), forward[j].wall.Sub(wall)
		if left == right {
			return forward[i].instant.Before(forward[j].instant)
		}
		return left < right
	})
	return forward[0].instant, true
}
