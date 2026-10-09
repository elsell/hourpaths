package activity

import (
	"math"
	"testing"
	"time"
)

func TestOrdinaryReminderScheduleUsesRemainingProgressAndEarlierDeadline(t *testing.T) {
	end := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	base := GoalReminderState{IntervalStartedAt: end.Add(-24 * time.Hour), IntervalEndedAt: end, TargetSeconds: 3600, RecordedSeconds: 1200}
	for _, tc := range []struct {
		name     string
		quiet    time.Time
		recorded int64
		running  bool
		want     time.Time
	}{
		{"ordinary", time.Time{}, 1200, false, end.Add(-70 * time.Minute)},
		{"before unavailable period", end.Add(-2 * time.Hour), 1200, false, end.Add(-190 * time.Minute)},
		{"unavailable after interval", end.Add(time.Hour), 1200, false, end.Add(-70 * time.Minute)},
		{"goal complete", time.Time{}, 3600, false, time.Time{}},
		{"active timer", time.Time{}, 1200, true, time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			s.NextUnavailableStart = tc.quiet
			s.RecordedSeconds = tc.recorded
			s.Running = tc.running
			got, eligible, err := s.Schedule()
			if err != nil || eligible != !tc.want.IsZero() || !got.Equal(tc.want) {
				t.Fatalf("schedule=%v eligible=%v err=%v want=%v", got, eligible, err, tc.want)
			}
		})
	}
	base.TargetSeconds = math.MaxInt64
	if _, eligible, err := base.Schedule(); err != nil || eligible {
		t.Fatalf("overflowing target eligible=%v err=%v", eligible, err)
	}
}

func TestReminderBundlesAnchorFiveMinutesToEarliestSchedule(t *testing.T) {
	at := time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)
	groups, err := BundleGoalReminders([]GoalReminderPlan{
		{PathID: "third", ScheduledAt: at.Add(6 * time.Minute)},
		{PathID: "second", ScheduledAt: at.Add(5 * time.Minute)},
		{PathID: "first", ScheduledAt: at},
	})
	if err != nil || len(groups) != 2 {
		t.Fatalf("groups=%v err=%v", groups, err)
	}
	if !groups[0].ScheduledAt.Equal(at) || len(groups[0].Paths) != 2 || groups[0].Paths[0].PathID != "first" || groups[0].Paths[1].PathID != "second" {
		t.Fatalf("first bundle=%v", groups[0])
	}
	if len(groups[1].Paths) != 1 || groups[1].Paths[0].PathID != "third" {
		t.Fatalf("second bundle=%v", groups[1])
	}
}
