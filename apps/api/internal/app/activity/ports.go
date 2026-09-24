package activity

import (
	"context"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	StartTimerOperation           = "activity.timer.start"
	StopTimerOperation            = "activity.timer.stop"
	CreateManualActivityOperation = "activity.manual.create"
	UpdateActivityOperation       = "activity.update"
	DeleteActivityOperation       = "activity.delete"
)

type StartTimerCommand struct {
	Timer            domain.RunningTimer
	Idempotency      ports.Idempotency
	Audit            audit.Event
	IntervalProgress *IntervalProgressRequest
}

type StartTimerResult struct {
	Timer              domain.RunningTimer
	AccumulatedSeconds int64
	Replayed           bool
	IntervalProgress   *IntervalProgress
}

type StopTimerCommand struct {
	TimerID, PathID, ParticipantID string
	ActivityID                     string
	StoppedAt, RecordedAt          time.Time
	Idempotency                    ports.Idempotency
	Audit                          audit.Event
	IntervalProgress               *IntervalProgressRequest
}

type StopTimerResult struct {
	Activity           domain.RecordedActivity
	CurrentTimer       *domain.RunningTimer
	AccumulatedSeconds int64
	Saved              bool
	Replayed           bool
	IntervalProgress   *IntervalProgress
}

type CurrentTimerResult struct {
	Timer              *domain.RunningTimer
	AccumulatedSeconds int64
	IntervalProgress   *IntervalProgress
}

type IntervalProgressRequest struct {
	TargetSeconds int64
	Window        IntervalWindow
}

type IntervalProgress struct {
	TargetSeconds      int64
	AccumulatedSeconds int64
	Window             IntervalWindow
}

type ManualActivityInput struct {
	LocalDate, LocalStartTime string
	DurationSeconds           int64
	Note                      string
}

type ManualActivityDefaults struct {
	LocalDate, LocalStartTime, TimeZone string
	CurrentInstant                      time.Time
}

type CreateManualActivityCommand struct {
	Activity         domain.RecordedActivity
	Idempotency      ports.Idempotency
	Audit            audit.Event
	IntervalProgress *IntervalProgressRequest
}

type CreateManualActivityResult struct {
	Activity           domain.RecordedActivity
	Version            int64
	AccumulatedSeconds int64
	Replayed           bool
	IntervalProgress   *IntervalProgress
}

type UpdateActivityCommand struct {
	ActivityID, PathID, ParticipantID string
	Edit                              domain.ActivityEdit
	UpdatedAt                         time.Time
	Idempotency                       ports.Idempotency
	Audit                             audit.Event
	IntervalProgress                  *IntervalProgressRequest
}

type UpdateActivityResult struct {
	Activity           domain.RecordedActivity
	Revision           domain.ActivityRevision
	Version            int64
	AccumulatedSeconds int64
	Replayed           bool
	IntervalProgress   *IntervalProgress
}

type DeleteActivityCommand struct {
	ActivityID, PathID, ParticipantID string
	Idempotency                       ports.Idempotency
	Audit                             audit.Event
	IntervalProgress                  *IntervalProgressRequest
}

type DeleteActivityResult struct {
	AccumulatedSeconds      int64
	SessionCount            int64
	UnreadNotificationCount int64
	RemovedFeedEventIDs     []string
	Replayed                bool
	IntervalProgress        *IntervalProgress
}

type ActivityRevisionRecord struct {
	Revision domain.ActivityRevision
	Version  int64
}

type ActivityListRecord struct {
	Activity domain.RecordedActivity
	Version  int64
}

type ActivityPageRequest struct {
	AfterID        string
	AfterStartedAt time.Time
	ParticipantID  string
	Snapshot       time.Time
	Limit          int
}

type ActivityPage struct {
	Items   []ActivityListRecord
	HasMore bool
}

type ActivityRevisionPageRequest struct {
	BeforeVersion int64
	Snapshot      time.Time
	Limit         int
}

type ActivityRevisionPage struct {
	Items   []ActivityRevisionRecord
	HasMore bool
}

// ProfileReader exposes only the preference required to attribute new
// activity. It deliberately cannot read a profile without its owning user ID.
type ProfileReader interface {
	TimeZone(context.Context, string) (string, error)
}

type Repository interface {
	IntervalGoal(context.Context, string, string) (pathdomain.IntervalGoal, error)
	CurrentProjection(context.Context, string, string, *IntervalProgressRequest) (CurrentTimerResult, error)
	StartTimer(context.Context, StartTimerCommand) (StartTimerResult, error)
	StopTimer(context.Context, StopTimerCommand) (StopTimerResult, error)
	CreateManualActivity(context.Context, CreateManualActivityCommand) (CreateManualActivityResult, error)
	UpdateActivity(context.Context, UpdateActivityCommand) (UpdateActivityResult, error)
	DeleteActivity(context.Context, DeleteActivityCommand) (DeleteActivityResult, error)
	GetActivity(context.Context, string, string, string) (domain.RecordedActivity, int64, error)
	ListActivities(context.Context, string, string, ActivityPageRequest) (ActivityPage, error)
	ListActivityRevisions(context.Context, string, string, string, ActivityRevisionPageRequest) (ActivityRevisionPage, error)
	GetRunningTimer(context.Context, string, string) (domain.RunningTimer, error)
	AccumulatedSeconds(context.Context, string, string) (int64, error)
}
