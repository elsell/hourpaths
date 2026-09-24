package activity

import (
	"errors"
	"strings"
	"time"
	_ "time/tzdata"
)

var (
	errInvalidTimer    = errors.New("running timer state is invalid")
	errInvalidActivity = errors.New("completed activity state is invalid")
)

// RunningTimer is the goal-independent, incomplete occurrence captured when a
// participant starts tracking a Path. OccurrenceTimeZone is intentionally kept
// on the timer so a later preference change cannot rewrite calendar attribution.
type RunningTimer struct {
	ID                 string
	PathID             string
	ParticipantID      string
	StartedAt          time.Time
	OccurrenceTimeZone string
}

// RecordedActivity is the canonical completed occurrence. Duration is not a
// field: callers must derive it from the retained UTC instants.
type RecordedActivity struct {
	ID                 string
	PathID             string
	ParticipantID      string
	StartedAt          time.Time
	EndedAt            time.Time
	OccurrenceTimeZone string
	Note               string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func StartTimer(id, pathID, participantID string, startedAt time.Time, occurrenceTimeZone string, currentInstant time.Time) (RunningTimer, error) {
	if blank(id) || blank(pathID) || blank(participantID) || startedAt.IsZero() || currentInstant.IsZero() || startedAt.After(currentInstant) {
		return RunningTimer{}, errInvalidTimer
	}
	if !validIANATimeZone(occurrenceTimeZone) {
		return RunningTimer{}, errInvalidTimer
	}

	return RunningTimer{
		ID:                 id,
		PathID:             pathID,
		ParticipantID:      participantID,
		StartedAt:          startedAt.UTC(),
		OccurrenceTimeZone: occurrenceTimeZone,
	}, nil
}

// Stop ends the timer. A valid stop before one whole elapsed second deliberately
// returns saved=false rather than manufacturing a zero-duration activity.
func (timer RunningTimer) Stop(activityID string, stoppedAt, recordedAt time.Time) (RecordedActivity, bool, error) {
	if err := timer.validate(); err != nil {
		return RecordedActivity{}, false, err
	}
	if stoppedAt.IsZero() || recordedAt.IsZero() || stoppedAt.Before(timer.StartedAt) || stoppedAt.After(recordedAt) {
		return RecordedActivity{}, false, errInvalidActivity
	}

	stoppedAt = stoppedAt.UTC()
	recordedAt = recordedAt.UTC()
	if elapsedWholeSeconds(timer.StartedAt, stoppedAt) < 1 {
		return RecordedActivity{}, false, nil
	}
	if blank(activityID) {
		return RecordedActivity{}, false, errInvalidActivity
	}

	return RecordedActivity{
		ID:                 activityID,
		PathID:             timer.PathID,
		ParticipantID:      timer.ParticipantID,
		StartedAt:          timer.StartedAt,
		EndedAt:            stoppedAt,
		OccurrenceTimeZone: timer.OccurrenceTimeZone,
		CreatedAt:          recordedAt,
		UpdatedAt:          recordedAt,
	}, true, nil
}

// DurationSeconds derives whole elapsed seconds from UTC start and end instants.
// Integer seconds avoid time.Duration's approximately 290-year range limit.
func (activity RecordedActivity) DurationSeconds() int64 {
	return elapsedWholeSeconds(activity.StartedAt, activity.EndedAt)
}

func (timer RunningTimer) validate() error {
	if blank(timer.ID) || blank(timer.PathID) || blank(timer.ParticipantID) || timer.StartedAt.IsZero() || timer.StartedAt.Location() != time.UTC || !validIANATimeZone(timer.OccurrenceTimeZone) {
		return errInvalidTimer
	}
	return nil
}

func elapsedWholeSeconds(startedAt, endedAt time.Time) int64 {
	seconds := endedAt.Unix() - startedAt.Unix()
	if endedAt.Nanosecond() < startedAt.Nanosecond() {
		seconds--
	}
	return seconds
}

func validIANATimeZone(value string) bool {
	if value == "" || value == "Local" || strings.TrimSpace(value) != value {
		return false
	}
	_, err := time.LoadLocation(value)
	return err == nil
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}
