package social

import (
	"errors"
	"strings"
	"time"
)

type AchievementKind string

const (
	AchievementInterval AchievementKind = "interval"
	AchievementOverall  AchievementKind = "overall"
)

var errInvalidGoalAchievement = errors.New("goal achievement is invalid")

type GoalAchievement struct {
	ID, ParticipantID, PathID          string
	Kind                               AchievementKind
	TargetSeconds                      int64
	IntervalStartedAt, IntervalEndedAt time.Time
	PublishedAt                        time.Time
}

func NewGoalAchievement(id, participantID, pathID string, kind AchievementKind, targetSeconds int64, intervalStartedAt, intervalEndedAt, publishedAt time.Time) (GoalAchievement, error) {
	value := GoalAchievement{ID: id, ParticipantID: participantID, PathID: pathID, Kind: kind, TargetSeconds: targetSeconds, IntervalStartedAt: intervalStartedAt, IntervalEndedAt: intervalEndedAt, PublishedAt: publishedAt}
	if !value.Valid() {
		return GoalAchievement{}, errInvalidGoalAchievement
	}
	return value, nil
}

func (value GoalAchievement) Valid() bool {
	identityValid := value.ID != "" && strings.TrimSpace(value.ID) == value.ID &&
		value.ParticipantID != "" && strings.TrimSpace(value.ParticipantID) == value.ParticipantID &&
		value.PathID != "" && strings.TrimSpace(value.PathID) == value.PathID && value.TargetSeconds > 0 &&
		!value.PublishedAt.IsZero() && value.PublishedAt.Location() == time.UTC
	if !identityValid {
		return false
	}
	switch value.Kind {
	case AchievementInterval:
		return !value.IntervalStartedAt.IsZero() && !value.IntervalEndedAt.IsZero() &&
			value.IntervalStartedAt.Location() == time.UTC && value.IntervalEndedAt.Location() == time.UTC &&
			value.IntervalEndedAt.After(value.IntervalStartedAt)
	case AchievementOverall:
		return value.IntervalStartedAt.IsZero() && value.IntervalEndedAt.IsZero()
	default:
		return false
	}
}

func (value GoalAchievement) SupportedBy(accumulatedSeconds int64) bool {
	return value.Valid() && accumulatedSeconds >= value.TargetSeconds
}
