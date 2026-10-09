package activity

import (
	"errors"
	"math/big"
	"sort"
	"time"
)

// GoalReminderState belongs to one participant and one canonical goal interval.
// Calendar and unavailable-period adapters supply absolute instants in the
// participant's configured time zone; this calculation does not read a clock.
type GoalReminderState struct {
	IntervalStartedAt, IntervalEndedAt time.Time
	NextUnavailableStart               time.Time
	TargetSeconds, RecordedSeconds     int64
	Running                            bool
}

// Schedule computes the last-chance schedule, not delivery authorization or a
// once-per-interval receipt. Suppressed plans must not consume such a receipt.
func (s GoalReminderState) Schedule() (time.Time, bool, error) {
	if s.IntervalStartedAt.IsZero() || !s.IntervalEndedAt.After(s.IntervalStartedAt) || s.TargetSeconds <= 0 || s.RecordedSeconds < 0 {
		return time.Time{}, false, errors.New("goal reminder state is invalid")
	}
	if s.Running || s.RecordedSeconds >= s.TargetSeconds {
		return time.Time{}, false, nil
	}
	deadline := s.IntervalEndedAt
	if !s.NextUnavailableStart.IsZero() && s.NextUnavailableStart.Before(deadline) {
		deadline = s.NextUnavailableStart
	}
	seconds := new(big.Int).Sub(big.NewInt(deadline.Unix()), big.NewInt(s.TargetSeconds-s.RecordedSeconds))
	seconds.Sub(seconds, big.NewInt(30*60))
	if !seconds.IsInt64() {
		return time.Time{}, false, nil
	}
	scheduled := time.Unix(seconds.Int64(), int64(deadline.Nanosecond())).UTC()
	if scheduled.Before(s.IntervalStartedAt) {
		return time.Time{}, false, nil
	}
	return scheduled, true, nil
}

type GoalReminderPlan struct {
	PathID      string
	ScheduledAt time.Time
}
type GoalReminderBundle struct {
	ScheduledAt time.Time
	Paths       []GoalReminderPlan
}

// BundleGoalReminders accepts only one participant's already eligible plans.
// Each window stays anchored to its first member, never the previous member.
func BundleGoalReminders(plans []GoalReminderPlan) ([]GoalReminderBundle, error) {
	ordered := append([]GoalReminderPlan(nil), plans...)
	seen := make(map[string]bool, len(ordered))
	for _, p := range ordered {
		if p.PathID == "" || p.ScheduledAt.IsZero() || seen[p.PathID] {
			return nil, errors.New("goal reminder plans are invalid")
		}
		seen[p.PathID] = true
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].ScheduledAt.Equal(ordered[j].ScheduledAt) {
			return ordered[i].PathID < ordered[j].PathID
		}
		return ordered[i].ScheduledAt.Before(ordered[j].ScheduledAt)
	})
	bundles := make([]GoalReminderBundle, 0)
	for _, p := range ordered {
		if len(bundles) == 0 || p.ScheduledAt.After(bundles[len(bundles)-1].ScheduledAt.Add(5*time.Minute)) {
			bundles = append(bundles, GoalReminderBundle{ScheduledAt: p.ScheduledAt})
		}
		last := &bundles[len(bundles)-1]
		last.Paths = append(last.Paths, p)
	}
	return bundles, nil
}
