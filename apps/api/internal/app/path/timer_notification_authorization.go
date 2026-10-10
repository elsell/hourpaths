package path

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (s *InvitationService) authorizeTimerNotification(ctx context.Context, recipient string, notice InvitationNotificationProjection) error {
	if notice.Kind == NotificationGoalPracticeReminder {
		return s.authorizeGoalReminderNotification(ctx, recipient, notice)
	}
	if notice.Kind != NotificationTimerStarted && notice.Kind != NotificationLongTimerRunning && notice.Kind != NotificationGoalNoLongerAchievable && !notice.Kind.IsAchievement() {
		return nil
	}
	if (notice.Kind == NotificationLongTimerRunning || notice.Kind == NotificationGoalNoLongerAchievable) && notice.Actor.UserID != recipient {
		return ports.ErrNotFound
	}
	if s.Authorizer == nil {
		return errInvalidInvitationDependencies
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(notice.PathID), "view", recipient)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	denial := shared.NewAuditEvent(ctx, s.Clock, recipient, recipient, audit.ResourceAccessDenied, "notification", notice.ID, audit.Denied)
	if err := s.Audits.AppendAuditEvent(ctx, denial); err != nil {
		return err
	}
	return ports.ErrNotFound
}
