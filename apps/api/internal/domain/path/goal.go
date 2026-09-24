package path

import "time"

type Recurrence string

const (
	RecurrenceHourly  Recurrence = "hourly"
	RecurrenceDaily   Recurrence = "daily"
	RecurrenceWeekly  Recurrence = "weekly"
	RecurrenceMonthly Recurrence = "monthly"
	RecurrenceYearly  Recurrence = "yearly"
)

// GoalAlignment stores the recurrence-specific calendar boundary. Fields not
// used by a recurrence remain zero so the persisted value has one canonical
// representation.
type GoalAlignment struct {
	Minute     int
	Hour       int
	ISOWeekday int
	Month      int
	Day        int
}

// IntervalGoal uses Present to distinguish an omitted goal from an invalid
// partial goal while retaining value comparability.
type IntervalGoal struct {
	Present       bool
	TargetSeconds int64
	Recurrence    Recurrence
	Alignment     GoalAlignment
}

// OverallTarget uses Present to distinguish an omitted goal from an invalid
// partial goal while retaining value comparability.
type OverallTarget struct {
	Present       bool
	TargetSeconds int64
}

// GoalConfiguration is the complete independently optional goal state of a
// Path. Its value form makes an exact reviewed configuration comparable at a
// mutation boundary.
type GoalConfiguration struct {
	IntervalGoal  IntervalGoal
	OverallTarget OverallTarget
}

func (configuration GoalConfiguration) Valid() bool {
	return configuration.IntervalGoal.valid() && configuration.OverallTarget.valid()
}

func (goal IntervalGoal) valid() bool {
	if !goal.Present {
		return goal == (IntervalGoal{})
	}
	if goal.TargetSeconds <= 0 {
		return false
	}
	return goal.Alignment.validFor(goal.Recurrence)
}

func (goal OverallTarget) valid() bool {
	if !goal.Present {
		return goal == (OverallTarget{})
	}
	return goal.TargetSeconds > 0
}

func (alignment GoalAlignment) validFor(recurrence Recurrence) bool {
	switch recurrence {
	case RecurrenceHourly:
		return alignment.Minute >= 0 && alignment.Minute <= 59 &&
			alignment.Hour == 0 && alignment.ISOWeekday == 0 && alignment.Month == 0 && alignment.Day == 0
	case RecurrenceDaily:
		return alignment.Hour >= 0 && alignment.Hour <= 23 &&
			alignment.Minute == 0 && alignment.ISOWeekday == 0 && alignment.Month == 0 && alignment.Day == 0
	case RecurrenceWeekly:
		return alignment.ISOWeekday >= 1 && alignment.ISOWeekday <= 7 &&
			alignment.Minute == 0 && alignment.Hour == 0 && alignment.Month == 0 && alignment.Day == 0
	case RecurrenceMonthly:
		return alignment.Day >= 1 && alignment.Day <= 31 &&
			alignment.Minute == 0 && alignment.Hour == 0 && alignment.ISOWeekday == 0 && alignment.Month == 0
	case RecurrenceYearly:
		return alignment.validYearlyDate() &&
			alignment.Minute == 0 && alignment.Hour == 0 && alignment.ISOWeekday == 0
	default:
		return false
	}
}

func (alignment GoalAlignment) validYearlyDate() bool {
	if alignment.Month < 1 || alignment.Month > 12 || alignment.Day < 1 {
		return false
	}
	// The leap year 2000 admits February 29 while rejecting every impossible
	// month/day pair that can never be a yearly boundary.
	date := time.Date(2000, time.Month(alignment.Month), alignment.Day, 0, 0, 0, 0, time.UTC)
	return int(date.Month()) == alignment.Month && date.Day() == alignment.Day
}
