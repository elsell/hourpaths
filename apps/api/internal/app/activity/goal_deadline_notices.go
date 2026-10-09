package activity

import (
	"context"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

// Candidates are internal work identities. Only eligible candidates consume
// admission rate limits; the scan cursor also advances over ineligible Paths.
type GoalDeadlineCandidate struct{ ParticipantID, PathID string }
type GoalDeadlinePage struct {
	Candidates []GoalDeadlineCandidate
	Next       GoalDeadlineCandidate
}
type GoalDeadlineNoticeCommand struct {
	Candidate GoalDeadlineCandidate
	At        time.Time
	Audit     audit.Event
}
type GoalDeadlineNoticeRepository interface {
	ListGoalDeadlineCandidates(context.Context, time.Time, GoalDeadlineCandidate, int) (GoalDeadlinePage, error)
	PublishGoalDeadlineNotice(context.Context, GoalDeadlineNoticeCommand) (bool, error)
}

func (c GoalDeadlineCandidate) After(other GoalDeadlineCandidate) bool {
	return c.ParticipantID > other.ParticipantID || (c.ParticipantID == other.ParticipantID && c.PathID > other.PathID)
}
