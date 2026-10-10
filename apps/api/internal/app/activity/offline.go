package activity

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const OfflineTimerOperation = "activity.offline.timer"

type OfflineTimerInput struct {
	// StillRunning is current device delivery context, not timer identity. Older
	// clients omit it and cannot produce a delayed start notification.
	StillRunning       bool
	CorrectedStartedAt *time.Time
	TimerID            string
	Kind               string
	StartedAt          time.Time
	EndedAt            *time.Time
	OccurrenceTimeZone string
}
type OfflineTimerCommand struct {
	NotificationRecipients []string
	NotifyStart            bool
	CorrectedStartedAt     *time.Time
	IdentityHash           []byte
	Timer                  domain.RunningTimer
	Kind                   string
	EndedAt                *time.Time
	RecordedAt             time.Time
	ActivityID             string
	Idempotency            ports.Idempotency
	Audit                  audit.Event
}
type OfflineTimerResult struct {
	Terminal         bool
	MustStop         bool
	Outcome          string
	Timer            *domain.RunningTimer
	Activity         *domain.RecordedActivity
	SavedSeconds     int64
	DiscardedSeconds int64
	Replayed         bool
}
type OfflineRepository interface {
	SynchronizeTimer(context.Context, OfflineTimerCommand) (OfflineTimerResult, error)
}

func (s *Service) SynchronizeTimer(ctx context.Context, authorization, pathID, key string, input OfflineTimerInput) (OfflineTimerResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return OfflineTimerResult{}, err
	}
	if !validPathID(pathID) || !validIdempotencyKey(key) || strings.TrimSpace(input.TimerID) != input.TimerID || input.TimerID == "" || len(input.TimerID) > 128 {
		return OfflineTimerResult{}, ports.ErrInvalidArgument
	}
	if input.Kind != "start" && input.Kind != "stop" && input.Kind != "correct" || (input.Kind == "start" && input.EndedAt != nil) || (input.Kind != "start" && input.EndedAt == nil) || (input.Kind == "correct") != (input.CorrectedStartedAt != nil) || (input.StillRunning && input.Kind != "start") {
		return OfflineTimerResult{}, ports.ErrInvalidArgument
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return OfflineTimerResult{}, err
	}
	if s.Repository == nil {
		return OfflineTimerResult{}, errInvalidDependencies
	}
	var timer domain.RunningTimer
	if input.Kind == "correct" {
		timer, err = domain.RestoreOfflineTimerIdentity(input.TimerID, pathID, principal.UserID, durableInstant(input.StartedAt), input.OccurrenceTimeZone)
	} else {
		timer, err = domain.StartTimer(input.TimerID, pathID, principal.UserID, durableInstant(input.StartedAt), input.OccurrenceTimeZone, now)
	}
	if err != nil {
		return OfflineTimerResult{}, ports.ErrInvalidArgument
	}
	var endedAt, correctedStartedAt *time.Time
	effectiveStart := timer.StartedAt
	correctionHash := ""
	if input.CorrectedStartedAt != nil {
		reviewed, reviewErr := domain.StartTimer(input.TimerID, pathID, principal.UserID, durableInstant(*input.CorrectedStartedAt), input.OccurrenceTimeZone, now)
		if reviewErr != nil {
			return OfflineTimerResult{}, ports.ErrInvalidArgument
		}
		effectiveStart = reviewed.StartedAt
		correctedStartedAt = &effectiveStart
		correctionHash = effectiveStart.Format(time.RFC3339Nano)
	}
	endHash := ""
	if input.EndedAt != nil {
		value := durableInstant(*input.EndedAt)
		if !value.After(effectiveStart) || value.After(now) {
			return OfflineTimerResult{}, ports.ErrInvalidArgument
		}
		endedAt = &value
		endHash = value.Format(time.RFC3339Nano)
	}
	action := audit.ActivityTimerStarted
	if input.Kind != "start" {
		action = audit.ActivityTimerStopped
	}
	command := OfflineTimerCommand{NotifyStart: input.StillRunning, IdentityHash: OfflineTimerIdentityHash(timer), Timer: timer, Kind: input.Kind, EndedAt: endedAt, CorrectedStartedAt: correctedStartedAt, RecordedAt: now, ActivityID: s.NewID(),
		Idempotency: ports.Idempotency{PrincipalID: principal.UserID, Operation: OfflineTimerOperation, Key: key,
			RequestHash: requestHash(OfflineTimerOperation, pathID, input.TimerID, input.Kind, timer.StartedAt.Format(time.RFC3339Nano), endHash, timer.OccurrenceTimeZone, correctionHash)},
		Audit: s.auditEvent(ctx, principal.UserID, action, input.TimerID)}
	if command.NotifyStart {
		command.NotificationRecipients, err = s.timerNotificationRecipients(ctx, principal.UserID, pathID)
		if err != nil {
			return OfflineTimerResult{}, err
		}
	}
	result, err := s.Repository.SynchronizeTimer(ctx, command)
	if err != nil {
		return OfflineTimerResult{}, err
	}
	if result.Outcome != "accepted" && result.Outcome != "conflict" && result.Outcome != "archived" {
		return OfflineTimerResult{}, errInvalidDependencies
	}
	if result.Terminal && result.MustStop || result.Timer != nil && (result.Terminal || result.MustStop) {
		return OfflineTimerResult{}, errInvalidDependencies
	}
	if result.SavedSeconds < 0 || result.DiscardedSeconds < 0 {
		return OfflineTimerResult{}, errInvalidDependencies
	}
	if result.Timer != nil && !validTimerFor(*result.Timer, principal.UserID, pathID) {
		return OfflineTimerResult{}, errInvalidDependencies
	}
	if result.Activity != nil && (result.Activity.ParticipantID != principal.UserID || result.Activity.PathID != pathID || result.Activity.DurationSeconds() != result.SavedSeconds) {
		return OfflineTimerResult{}, errors.Join(ports.ErrInvalidArgument, errInvalidDependencies)
	}
	if result.Terminal && input.Kind != "start" {
		s.reconcileStoppedGoalReminders(ctx, principal.UserID)
	}
	return result, nil
}

// OfflineTimerIdentityHash binds immutable occurrence data. Client and canonical
// IDs are separately scoped aliases in the persistence identity registry.
func OfflineTimerIdentityHash(timer domain.RunningTimer) []byte {
	return requestHash("activity.offline.identity", timer.ParticipantID, timer.PathID, durableInstant(timer.StartedAt).Format(time.RFC3339Nano), timer.OccurrenceTimeZone)
}
