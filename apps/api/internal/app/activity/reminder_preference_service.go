package activity

import (
	"context"
	"errors"
	"math"
	"strconv"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (s *Service) GetGoalReminderPreference(ctx context.Context, authorization, pathID string) (GoalReminderPreference, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return GoalReminderPreference{}, err
	}
	if !validPathID(pathID) {
		return GoalReminderPreference{}, ports.ErrInvalidArgument
	}
	if s.ReminderPreferences == nil {
		return GoalReminderPreference{}, errInvalidDependencies
	}
	if _, err = s.authorizeTracking(ctx, principal.UserID, pathID); err != nil {
		return GoalReminderPreference{}, err
	}
	result, err := s.ReminderPreferences.GetGoalReminderPreference(ctx, principal.UserID, pathID)
	if err != nil {
		return GoalReminderPreference{}, s.reminderPreferenceFailure(ctx, principal.UserID, pathID, err)
	}
	if result.Revision < 0 {
		return GoalReminderPreference{}, errInvalidDependencies
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceViewed, pathID)
	event.TargetType = "goal_reminder_preference"
	if err = s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return GoalReminderPreference{}, err
	}
	return result, nil
}

func (s *Service) UpdateGoalReminderPreference(ctx context.Context, authorization, pathID, key string, revision int64, enabled bool) (GoalReminderPreference, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return GoalReminderPreference{}, err
	}
	if !validPathID(pathID) || !validIdempotencyKey(key) || revision < 0 || revision == math.MaxInt64 {
		return GoalReminderPreference{}, ports.ErrInvalidArgument
	}
	if s.ReminderPreferences == nil {
		return GoalReminderPreference{}, errInvalidDependencies
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return GoalReminderPreference{}, err
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceUpdated, pathID)
	event.TargetType = "goal_reminder_preference"
	event.OccurredAt = now
	result, err := s.ReminderPreferences.UpdateGoalReminderPreference(ctx, GoalReminderPreferenceCommand{
		ParticipantID: principal.UserID, PathID: pathID, Enabled: enabled, ExpectedRevision: revision, At: now, Audit: event,
		Idempotency: ports.Idempotency{PrincipalID: principal.UserID, Operation: UpdateGoalReminderPreferenceOperation, Key: key, RequestHash: requestHash(UpdateGoalReminderPreferenceOperation, pathID, strconv.FormatInt(revision, 10), strconv.FormatBool(enabled))},
	})
	if err != nil {
		return GoalReminderPreference{}, s.reminderPreferenceFailure(ctx, principal.UserID, pathID, err)
	}
	if result.Preference.Enabled != enabled || result.Preference.Revision != revision+1 {
		return GoalReminderPreference{}, errInvalidDependencies
	}
	return result.Preference, nil
}

func (s *Service) reminderPreferenceFailure(ctx context.Context, actor, pathID string, err error) error {
	if errors.Is(err, ports.ErrNotFound) {
		if auditErr := s.Audits.AppendAuditEvent(ctx, s.auditEvent(ctx, actor, audit.ResourceAccessDenied, pathID)); auditErr != nil {
			return auditErr
		}
	}
	return err
}
