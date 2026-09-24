package activity

import (
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const maximumActivityNoteCharacters = 2000

// ManualActivity contains the participant-supplied state needed to construct a
// canonical completed activity. DurationSeconds is deliberately integral so a
// manual entry cannot create a fractional canonical duration.
type ManualActivity struct {
	ID                 string
	PathID             string
	ParticipantID      string
	StartedAt          time.Time
	DurationSeconds    int64
	OccurrenceTimeZone string
	Note               string
}

// ActivityEdit is a complete replacement of the editable occurrence state.
// Stable identity, ownership, and creation time remain on RecordedActivity.
type ActivityEdit struct {
	StartedAt          time.Time
	DurationSeconds    int64
	OccurrenceTimeZone string
	Note               string
}

// ActivityRevision is the exact prior synchronized state replaced by an edit.
// Both fields are values, so editing the returned activity cannot mutate the
// retained prior state.
type ActivityRevision struct {
	Activity   RecordedActivity
	ReplacedAt time.Time
}

// RecordManualActivity derives the end instant from a positive whole-second
// duration and canonicalizes all occurrence instants to UTC.
func RecordManualActivity(input ManualActivity, currentInstant time.Time) (RecordedActivity, error) {
	return completedActivity(
		input.ID,
		input.PathID,
		input.ParticipantID,
		input.StartedAt,
		input.DurationSeconds,
		input.OccurrenceTimeZone,
		input.Note,
		currentInstant,
	)
}

// EditByOwner replaces editable state only when requested by the attributed
// participant. The receiver is not mutated; the returned revision is a copy of
// the complete prior state for atomic persistence beside the replacement.
func (activity RecordedActivity) EditByOwner(ownerParticipantID string, edit ActivityEdit, currentInstant time.Time) (RecordedActivity, ActivityRevision, error) {
	if blank(ownerParticipantID) || ownerParticipantID != activity.ParticipantID || currentInstant.IsZero() || !activity.validAt(currentInstant) {
		return RecordedActivity{}, ActivityRevision{}, errInvalidActivity
	}

	edited, err := completedActivity(
		activity.ID,
		activity.PathID,
		activity.ParticipantID,
		edit.StartedAt,
		edit.DurationSeconds,
		edit.OccurrenceTimeZone,
		edit.Note,
		currentInstant,
	)
	if err != nil {
		return RecordedActivity{}, ActivityRevision{}, err
	}
	edited.CreatedAt = activity.CreatedAt

	return edited, ActivityRevision{Activity: activity, ReplacedAt: currentInstant.UTC()}, nil
}

// ValidateAt checks a completed activity without deriving or rewriting either
// retained instant. Timer-created activities may contain fractional elapsed
// seconds even though their reported duration is whole-second precision.
func (activity RecordedActivity) ValidateAt(currentInstant time.Time) error {
	if currentInstant.IsZero() || !activity.validAt(currentInstant) {
		return errInvalidActivity
	}
	return nil
}

func completedActivity(id, pathID, participantID string, startedAt time.Time, durationSeconds int64, occurrenceTimeZone, note string, currentInstant time.Time) (RecordedActivity, error) {
	if blank(id) || blank(pathID) || blank(participantID) || startedAt.IsZero() || currentInstant.IsZero() || durationSeconds <= 0 || !validIANATimeZone(occurrenceTimeZone) {
		return RecordedActivity{}, errInvalidActivity
	}

	normalizedNote, err := normalizeActivityNote(note)
	if err != nil {
		return RecordedActivity{}, err
	}
	startedAt = startedAt.UTC()
	currentInstant = currentInstant.UTC()
	endedAt, valid := addWholeSeconds(startedAt, durationSeconds)
	if !valid || endedAt.After(currentInstant) {
		return RecordedActivity{}, errInvalidActivity
	}

	return RecordedActivity{
		ID:                 id,
		PathID:             pathID,
		ParticipantID:      participantID,
		StartedAt:          startedAt,
		EndedAt:            endedAt,
		OccurrenceTimeZone: occurrenceTimeZone,
		Note:               normalizedNote,
		CreatedAt:          currentInstant,
		UpdatedAt:          currentInstant,
	}, nil
}

func (activity RecordedActivity) validAt(currentInstant time.Time) bool {
	if blank(activity.ID) || blank(activity.PathID) || blank(activity.ParticipantID) || activity.StartedAt.IsZero() || activity.EndedAt.IsZero() || activity.CreatedAt.IsZero() || activity.UpdatedAt.IsZero() {
		return false
	}
	if activity.StartedAt.Location() != time.UTC || activity.EndedAt.Location() != time.UTC || activity.CreatedAt.Location() != time.UTC || activity.UpdatedAt.Location() != time.UTC {
		return false
	}
	currentInstant = currentInstant.UTC()
	if elapsedWholeSeconds(activity.StartedAt, activity.EndedAt) < 1 || activity.EndedAt.After(currentInstant) || activity.CreatedAt.After(activity.UpdatedAt) || activity.UpdatedAt.After(currentInstant) || !validIANATimeZone(activity.OccurrenceTimeZone) {
		return false
	}
	normalizedNote, err := normalizeActivityNote(activity.Note)
	return err == nil && normalizedNote == activity.Note
}

func addWholeSeconds(startedAt time.Time, durationSeconds int64) (time.Time, bool) {
	startedUnix := startedAt.Unix()
	if durationSeconds <= 0 || startedUnix > math.MaxInt64-durationSeconds {
		return time.Time{}, false
	}
	endedAt := time.Unix(startedUnix+durationSeconds, int64(startedAt.Nanosecond())).UTC()
	if !endedAt.After(startedAt) || elapsedWholeSeconds(startedAt, endedAt) != durationSeconds {
		return time.Time{}, false
	}
	return endedAt, true
}

func normalizeActivityNote(note string) (string, error) {
	if !utf8.ValidString(note) {
		return "", errInvalidActivity
	}
	note = norm.NFC.String(note)
	if strings.TrimSpace(note) == "" {
		return "", nil
	}
	if utf8.RuneCountInString(note) > maximumActivityNoteCharacters {
		return "", errInvalidActivity
	}
	for _, character := range note {
		if unicode.IsControl(character) && character != '\n' && character != '\r' {
			return "", errInvalidActivity
		}
	}
	return note, nil
}
