package activity

import (
	"errors"
	"math/big"
	"time"
)

// GoalDeadlineState contains the current participant's recorded progress and
// canonical calendar interval. A running timer suppresses delivery even when
// provisional progress would still leave the goal impossible.
type GoalDeadlineState struct {
	IntervalStartedAt, IntervalEndedAt time.Time
	TargetSeconds, RecordedSeconds     int64
	Running                            bool
}

func (state GoalDeadlineState) NoLongerAchievable(now time.Time) (bool, error) {
	if state.IntervalStartedAt.IsZero() || !state.IntervalEndedAt.After(state.IntervalStartedAt) || now.IsZero() || state.TargetSeconds <= 0 || state.RecordedSeconds < 0 {
		return false, errors.New("goal deadline state is invalid")
	}
	if state.Running || state.RecordedSeconds >= state.TargetSeconds || now.Before(state.IntervalStartedAt) || !now.Before(state.IntervalEndedAt) {
		return false, nil
	}
	// Keep the strict comparison exact at subsecond boundaries, without duration
	// overflow for very large goals. Equality still permits completion.
	remaining := new(big.Int).Sub(big.NewInt(state.IntervalEndedAt.Unix()), big.NewInt(now.Unix()))
	remaining.Mul(remaining, big.NewInt(int64(time.Second)))
	remaining.Add(remaining, big.NewInt(int64(state.IntervalEndedAt.Nanosecond()-now.Nanosecond())))
	needed := new(big.Int).Mul(big.NewInt(state.TargetSeconds-state.RecordedSeconds), big.NewInt(int64(time.Second)))
	return needed.Cmp(remaining) > 0, nil
}
