package activity

import (
	"context"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const UpdateGoalReminderPreferenceOperation = "activity.goal_reminders.update"

type GoalReminderPreference struct {
	Enabled  bool
	Revision int64
}

type GoalReminderPreferenceCommand struct {
	ParticipantID, PathID string
	Enabled               bool
	ExpectedRevision      int64
	At                    time.Time
	Idempotency           ports.Idempotency
	Audit                 audit.Event
}

type GoalReminderPreferenceResult struct {
	Preference GoalReminderPreference
	Replayed   bool
}

type GoalReminderPreferenceRepository interface {
	GetGoalReminderPreference(context.Context, string, string) (GoalReminderPreference, error)
	UpdateGoalReminderPreference(context.Context, GoalReminderPreferenceCommand) (GoalReminderPreferenceResult, error)
}
