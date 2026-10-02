package activity

import (
	"errors"
	"time"
)

var ErrOfflineClock = errors.New("offline occurrence requires timing correction")

// OfflineTimerWinner is symmetric so replay order cannot determine the winner.
func OfflineTimerWinner(first, second RunningTimer) (RunningTimer, error) {
	if first.validate() != nil || second.validate() != nil || first.PathID != second.PathID || first.ParticipantID != second.ParticipantID {
		return RunningTimer{}, errInvalidTimer
	}
	if first.StartedAt.After(second.StartedAt) || first.StartedAt.Equal(second.StartedAt) && first.ID > second.ID {
		return first, nil
	}
	return second, nil
}

type OfflineStopResult struct {
	Activity         RecordedActivity
	Saved            bool
	Archived         bool
	DiscardedSeconds int64
}

// ResolveOfflineStop uses device UTC instants and the authoritative archive cut.
// It never reassigns a discarded interval to another Path or creates zero time.
func ResolveOfflineStop(timer RunningTimer, activityID string, endedAt, recordedAt time.Time, archivedAt *time.Time) (OfflineStopResult, error) {
	if timer.validate() != nil || endedAt.IsZero() || recordedAt.IsZero() || endedAt.After(recordedAt) {
		return OfflineStopResult{}, errInvalidActivity
	}
	if !endedAt.After(timer.StartedAt) {
		return OfflineStopResult{}, ErrOfflineClock
	}
	if archivedAt != nil && (archivedAt.IsZero() || archivedAt.After(recordedAt)) {
		return OfflineStopResult{}, errInvalidActivity
	}
	result := OfflineStopResult{}
	effectiveEnd := endedAt
	if archivedAt != nil && endedAt.After(*archivedAt) {
		result.Archived = true
		if !timer.StartedAt.Before(*archivedAt) {
			result.DiscardedSeconds = elapsedWholeSeconds(timer.StartedAt, endedAt)
			return result, nil
		}
		effectiveEnd = *archivedAt
		// Difference of whole canonical durations preserves saved+discarded total.
		result.DiscardedSeconds = elapsedWholeSeconds(timer.StartedAt, endedAt) - elapsedWholeSeconds(timer.StartedAt, effectiveEnd)
	}
	activity, saved, err := timer.Stop(activityID, effectiveEnd, recordedAt)
	if err != nil {
		return OfflineStopResult{}, err
	}
	result.Activity, result.Saved = activity, saved
	return result, nil
}

// RestoreOfflineTimerIdentity validates an original device identity even when
// the device's former clock placed its start after the current server instant.
// It is only used to identify a correction, never to start a live timer.
func RestoreOfflineTimerIdentity(id, pathID, participantID string, startedAt time.Time, zone string) (RunningTimer, error) {
	timer := RunningTimer{ID: id, PathID: pathID, ParticipantID: participantID, StartedAt: startedAt.UTC(), OccurrenceTimeZone: zone}
	if err := timer.validate(); err != nil {
		return RunningTimer{}, err
	}
	return timer, nil
}
