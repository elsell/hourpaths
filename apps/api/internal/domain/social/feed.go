package social

import (
	"errors"
	"strings"
	"time"

	activitydomain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
)

const practiceSessionEventIDPrefix = "practice:"

var errInvalidPracticeSessionEvent = errors.New("practice session feed event is invalid")

// PracticeSessionEvent is the immutable publication identity for one completed
// activity. Occurrence details and private notes deliberately remain on the
// source activity so a feed projection can apply current visibility and
// redaction rules instead of retaining a second copy.
type PracticeSessionEvent struct {
	ID               string
	SourceActivityID string
	ParticipantID    string
	PathID           string
	PublishedAt      time.Time
}

// NewPracticeSessionEvent deterministically publishes one event per source
// activity. RecordedActivity.CreatedAt is the server acceptance instant and is
// preserved by edits, so backdated occurrence time and later edits cannot move
// an event in the chronological feed.
func NewPracticeSessionEvent(activity activitydomain.RecordedActivity) (PracticeSessionEvent, error) {
	if err := activity.ValidateAt(activity.UpdatedAt); err != nil || strings.TrimSpace(activity.ID) == "" {
		return PracticeSessionEvent{}, errInvalidPracticeSessionEvent
	}
	return PracticeSessionEvent{
		ID: practiceSessionEventIDPrefix + activity.ID, SourceActivityID: activity.ID,
		ParticipantID: activity.ParticipantID, PathID: activity.PathID,
		PublishedAt: activity.CreatedAt,
	}, nil
}
