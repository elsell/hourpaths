package activity

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"time"
)

// LongTimerCandidate is a bounded background-work identity, not a user-facing
// read. Admission must check the participant's current tracking permission.
type LongTimerCandidate struct{ TimerID, PathID, ParticipantID string }
type LongTimerNoticeCommand struct {
	Candidate LongTimerCandidate
	At        time.Time
	Audit     audit.Event
}
type LongTimerNoticeRepository interface {
	ListLongTimerCandidates(context.Context, time.Time, string, int) ([]LongTimerCandidate, error)
	// Publish rechecks the live timer, membership, channel and current session
	// aggregate atomically with the receipt, notification, push work and audit.
	PublishLongTimerNotice(context.Context, LongTimerNoticeCommand) (bool, error)
}
