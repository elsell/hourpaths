package path

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
)

func validGoalReminderPaths(bundle *GoalReminderBundle) bool {
	if bundle == nil || len(bundle.Paths) == 0 {
		return false
	}
	seen := make(map[string]bool, len(bundle.Paths))
	for _, p := range bundle.Paths {
		id := string(p.ID)
		if id == "" || strings.TrimSpace(id) != id || p.Name == "" || strings.TrimSpace(p.Name) != p.Name || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func (s *InvitationService) authorizeGoalReminderNotification(ctx context.Context, recipient string, notice InvitationNotificationProjection) error {
	if notice.Actor.UserID != recipient || !validGoalReminderPaths(notice.Reminder) {
		return ports.ErrNotFound
	}
	if s.Authorizer == nil {
		return errInvalidInvitationDependencies
	}
	for _, path := range notice.Reminder.Paths {
		allowed, err := s.Authorizer.Check(ctx, "path", string(path.ID), "view", recipient)
		if err != nil {
			return err
		}
		if !allowed {
			event := shared.NewAuditEvent(ctx, s.Clock, recipient, recipient, audit.ResourceAccessDenied, "notification", notice.ID, audit.Denied)
			if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
				return err
			}
			return ports.ErrNotFound
		}
	}
	return nil
}
