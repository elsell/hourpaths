package activity

import (
	"errors"
	"math/big"
	"time"
)

// LongTimerNoticeDue evaluates the current completed-session aggregate for this
// timer's participant and Path. The repository supplies only current positive
// durations, including manual sessions. Delivery deduplication belongs to the
// transactional notification repository, not the calculation.
func (timer RunningTimer) LongTimerNoticeDue(count, totalSeconds int64, now time.Time) (bool, error) {
	if err := timer.validate(); err != nil {
		return false, err
	}
	if now.IsZero() || now.Before(timer.StartedAt) {
		return false, errInvalidTimer
	}
	if count < 0 || totalSeconds < 0 || totalSeconds < count || (count == 0 && totalSeconds != 0) {
		return false, errors.New("completed-session duration aggregate is invalid")
	}
	if count < 3 {
		return false, nil
	}
	// Compare elapsed * 2 * count with total * 3, keeping fractional means exact
	// and avoiding overflow for large histories or long-running timers.
	elapsed := new(big.Int).Sub(big.NewInt(now.Unix()), big.NewInt(timer.StartedAt.Unix()))
	elapsed.Mul(elapsed, big.NewInt(int64(time.Second)))
	elapsed.Add(elapsed, big.NewInt(int64(now.Nanosecond()-timer.StartedAt.Nanosecond())))
	elapsed.Mul(elapsed, big.NewInt(2))
	elapsed.Mul(elapsed, big.NewInt(count))
	threshold := new(big.Int).Mul(big.NewInt(totalSeconds), big.NewInt(3))
	threshold.Mul(threshold, big.NewInt(int64(time.Second)))
	return elapsed.Cmp(threshold) >= 0, nil
}
