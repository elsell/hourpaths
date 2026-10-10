package notificationchannel

import (
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/preferences"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"time"
)

// QuietAt evaluates only an explicitly saved period in the configured account
// zone. Callers provide the event or handoff instant; device time is irrelevant.
func QuietAt(tx *gorm.DB, recipient string, at time.Time) (bool, error) {
	period, zone, err := unavailablePeriod(tx, recipient, at)
	if err != nil || !period.Enabled {
		return false, err
	}
	return period.ContainsLocal(at.In(zone)), nil
}

// NextUnavailableStart supplies reminder planning with the same saved period
// and account zone used by final push suppression.
func NextUnavailableStart(tx *gorm.DB, recipient string, at time.Time) (time.Time, error) {
	period, zone, err := unavailablePeriod(tx, recipient, at)
	if err != nil || !period.Enabled {
		return time.Time{}, err
	}
	start, ok := period.NextStart(at.In(zone))
	if !ok {
		return time.Time{}, ports.ErrUnavailable
	}
	return start.UTC(), nil
}

func unavailablePeriod(tx *gorm.DB, recipient string, at time.Time) (preferences.UnavailablePeriod, *time.Location, error) {
	empty := preferences.UnavailablePeriod{}
	if tx == nil || recipient == "" || at.IsZero() {
		return empty, nil, ports.ErrInvalidArgument
	}
	var row struct {
		Enabled                bool
		StartMinute, EndMinute int
		TimeZone               string
	}
	err := tx.Table("user_unavailable_period_models q").Select("q.enabled,q.start_minute,q.end_minute,COALESCE(p.current_time_zone,'') AS time_zone").Joins("LEFT JOIN user_preference_models p ON p.user_id=q.user_id").Where("q.user_id=?", recipient).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return empty, nil, nil
	}
	if err != nil {
		return empty, nil, err
	}
	if !row.Enabled {
		return empty, nil, nil
	}
	// Equal endpoints remain unadmitted until the product decision is resolved.
	if row.StartMinute < 0 || row.StartMinute >= 1440 || row.EndMinute < 0 || row.EndMinute >= 1440 || row.StartMinute == row.EndMinute || row.TimeZone == "" {
		return empty, nil, ports.ErrUnavailable
	}
	zone, err := time.LoadLocation(row.TimeZone)
	if err != nil {
		return empty, nil, ports.ErrUnavailable
	}
	return preferences.UnavailablePeriod{Enabled: true, StartMinute: row.StartMinute, EndMinute: row.EndMinute}, zone, nil
}
