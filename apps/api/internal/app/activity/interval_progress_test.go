package activity

import (
	"testing"
	"time"

	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestCurrentIntervalWindowUsesStoredCalendarAlignment(t *testing.T) {
	tests := []struct {
		name      string
		goal      pathdomain.IntervalGoal
		now       time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "hourly before this hours boundary",
			goal:      intervalGoal(pathdomain.RecurrenceHourly, pathdomain.GoalAlignment{Minute: 15}),
			now:       time.Date(2026, time.July, 22, 14, 10, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.July, 22, 13, 15, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.July, 22, 14, 15, 0, 0, time.UTC),
		},
		{
			name:      "instant at boundary starts the new half open interval",
			goal:      intervalGoal(pathdomain.RecurrenceHourly, pathdomain.GoalAlignment{Minute: 15}),
			now:       time.Date(2026, time.July, 22, 14, 15, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.July, 22, 14, 15, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.July, 22, 15, 15, 0, 0, time.UTC),
		},
		{
			name:      "daily before todays boundary",
			goal:      intervalGoal(pathdomain.RecurrenceDaily, pathdomain.GoalAlignment{Hour: 5}),
			now:       time.Date(2026, time.July, 22, 4, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.July, 21, 5, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.July, 22, 5, 0, 0, 0, time.UTC),
		},
		{
			name:      "weekly ISO weekday",
			goal:      intervalGoal(pathdomain.RecurrenceWeekly, pathdomain.GoalAlignment{ISOWeekday: 1}),
			now:       time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "monthly day clamps independently at each boundary",
			goal:      intervalGoal(pathdomain.RecurrenceMonthly, pathdomain.GoalAlignment{Day: 31}),
			now:       time.Date(2026, time.March, 30, 12, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "monthly clamp is itself the new boundary",
			goal:      intervalGoal(pathdomain.RecurrenceMonthly, pathdomain.GoalAlignment{Day: 31}),
			now:       time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "yearly leap day falls back in non leap years",
			goal:      intervalGoal(pathdomain.RecurrenceYearly, pathdomain.GoalAlignment{Month: 2, Day: 29}),
			now:       time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2027, time.February, 28, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			window, err := currentIntervalWindow(test.goal, "UTC", test.now)
			if err != nil {
				t.Fatal(err)
			}
			if !window.StartedAt.Equal(test.wantStart) || !window.EndedAt.Equal(test.wantEnd) {
				t.Fatalf("currentIntervalWindow() = %v..%v, want %v..%v", window.StartedAt, window.EndedAt, test.wantStart, test.wantEnd)
			}
		})
	}
}

func TestCurrentIntervalWindowResolvesOffsetTransitionsBySpecification(t *testing.T) {
	tests := []struct {
		name      string
		goal      pathdomain.IntervalGoal
		now       time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "nonexistent daily boundary moves forward by the gap",
			goal:      intervalGoal(pathdomain.RecurrenceDaily, pathdomain.GoalAlignment{Hour: 2}),
			now:       time.Date(2026, time.March, 8, 8, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.March, 8, 7, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.March, 9, 6, 0, 0, 0, time.UTC),
		},
		{
			name:      "repeated hourly boundary uses the earlier occurrence",
			goal:      intervalGoal(pathdomain.RecurrenceHourly, pathdomain.GoalAlignment{Minute: 30}),
			now:       time.Date(2026, time.November, 1, 6, 45, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.November, 1, 7, 30, 0, 0, time.UTC),
		},
		{
			name:      "duplicate gap boundaries do not create a zero length interval",
			goal:      intervalGoal(pathdomain.RecurrenceHourly, pathdomain.GoalAlignment{Minute: 30}),
			now:       time.Date(2026, time.March, 8, 7, 45, 0, 0, time.UTC),
			wantStart: time.Date(2026, time.March, 8, 7, 30, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.March, 8, 8, 30, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			window, err := currentIntervalWindow(test.goal, "America/New_York", test.now)
			if err != nil {
				t.Fatal(err)
			}
			if !window.StartedAt.Equal(test.wantStart) || !window.EndedAt.Equal(test.wantEnd) {
				t.Fatalf("currentIntervalWindow() = %v..%v, want %v..%v", window.StartedAt, window.EndedAt, test.wantStart, test.wantEnd)
			}
		})
	}
}

func TestCurrentIntervalWindowUsesParticipantsConfiguredTimeZone(t *testing.T) {
	goal := intervalGoal(pathdomain.RecurrenceDaily, pathdomain.GoalAlignment{Hour: 0})
	now := time.Date(2026, time.July, 22, 2, 0, 0, 0, time.UTC)

	utcWindow, err := currentIntervalWindow(goal, "UTC", now)
	if err != nil {
		t.Fatal(err)
	}
	newYorkWindow, err := currentIntervalWindow(goal, "America/New_York", now)
	if err != nil {
		t.Fatal(err)
	}
	if !utcWindow.StartedAt.Equal(time.Date(2026, time.July, 22, 0, 0, 0, 0, time.UTC)) || !utcWindow.EndedAt.Equal(time.Date(2026, time.July, 23, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("UTC window = %v..%v", utcWindow.StartedAt, utcWindow.EndedAt)
	}
	if !newYorkWindow.StartedAt.Equal(time.Date(2026, time.July, 21, 4, 0, 0, 0, time.UTC)) || !newYorkWindow.EndedAt.Equal(time.Date(2026, time.July, 22, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("New York window = %v..%v", newYorkWindow.StartedAt, newYorkWindow.EndedAt)
	}
}

func TestCurrentIntervalWindowFailsClosedForInvalidInputs(t *testing.T) {
	valid := intervalGoal(pathdomain.RecurrenceDaily, pathdomain.GoalAlignment{Hour: 0})
	for name, input := range map[string]struct {
		goal     pathdomain.IntervalGoal
		timeZone string
		now      time.Time
	}{
		"missing goal":      {timeZone: "UTC", now: time.Now().UTC()},
		"invalid alignment": {goal: intervalGoal(pathdomain.RecurrenceDaily, pathdomain.GoalAlignment{Hour: 24}), timeZone: "UTC", now: time.Now().UTC()},
		"invalid time zone": {goal: valid, timeZone: "Local", now: time.Now().UTC()},
		"missing now":       {goal: valid, timeZone: "UTC"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := currentIntervalWindow(input.goal, input.timeZone, input.now); err == nil {
				t.Fatal("currentIntervalWindow() accepted invalid input")
			}
		})
	}
}

func intervalGoal(recurrence pathdomain.Recurrence, alignment pathdomain.GoalAlignment) pathdomain.IntervalGoal {
	return pathdomain.IntervalGoal{Present: true, TargetSeconds: 3600, Recurrence: recurrence, Alignment: alignment}
}
