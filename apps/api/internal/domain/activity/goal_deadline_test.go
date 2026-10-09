package activity

import (
	"math"
	"testing"
	"time"
)

func TestGoalDeadlineNoticeUsesActualRemainingTime(t *testing.T) {
	end := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	base := GoalDeadlineState{IntervalStartedAt: end.Add(-24 * time.Hour), IntervalEndedAt: end, TargetSeconds: 3600, RecordedSeconds: 1200}
	for _, tc := range []struct {
		name     string
		at       time.Time
		recorded int64
		running  bool
		want     bool
	}{
		{"exactly enough time", end.Add(-40 * time.Minute), 1200, false, false},
		{"first impossible instant", end.Add(-40*time.Minute + time.Nanosecond), 1200, false, true},
		{"completed", end.Add(-time.Minute), 3600, false, false},
		{"active own timer", end.Add(-time.Minute), 1200, true, false},
		{"after stop still incomplete", end.Add(-time.Minute), 3500, false, true},
		{"after stop enough progress", end.Add(-time.Minute), 3550, false, false},
		{"previous interval", end, 1200, false, false},
		{"future interval", base.IntervalStartedAt.Add(-time.Second), 1200, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			s.RecordedSeconds = tc.recorded
			s.Running = tc.running
			got, err := s.NoLongerAchievable(tc.at)
			if err != nil || got != tc.want {
				t.Fatalf("notice=%v error=%v want=%v", got, err, tc.want)
			}
		})
	}
	base.TargetSeconds = math.MaxInt64
	if due, err := base.NoLongerAchievable(end.Add(-time.Hour)); err != nil || !due {
		t.Fatalf("large target overflow: %v %v", due, err)
	}
	base.RecordedSeconds = -1
	if due, err := base.NoLongerAchievable(end.Add(-time.Hour)); err == nil || due {
		t.Fatalf("invalid progress accepted: %v %v", due, err)
	}
}
