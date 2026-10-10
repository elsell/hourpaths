package path

import (
	"context"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (service *InvitationService) ListNotifications(
	ctx context.Context,
	authorization string,
	cursor string,
	limit int,
) ([]InvitationNotificationProjection, string, int64, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", 0, err
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, "", 0, ports.ErrInvalidArgument
	}
	if service.Invitations == nil || service.Audits == nil ||
		service.AuditRateLimiter == nil || service.Clock == nil ||
		len(service.CursorSigningKey) < 32 {
		return nil, "", 0, errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC()
	if now.IsZero() {
		return nil, "", 0, errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", 0, platformapp.ErrRateLimited
	}
	request := NotificationPageRequest{GoalReminders: notificationGoalReminderRepresentation(ctx), GoalDeadlines: notificationGoalDeadlineRepresentation(ctx), LongTimers: notificationLongTimerRepresentation(ctx), Achievements: notificationAchievementRepresentation(ctx), Limit: limit, Snapshot: now, TimerStarts: notificationTimerRepresentation(ctx), EmojiReactions: notificationEmojiRepresentation(ctx)}
	cursorDomain := "path-notification"
	if request.EmojiReactions {
		cursorDomain = "path-notification-emoji"
	}
	if request.TimerStarts {
		cursorDomain += "-timers"
	}
	if request.Achievements {
		cursorDomain += "-achievements"
	}
	if request.LongTimers {
		cursorDomain += "-long-timers"
	}
	if request.GoalDeadlines {
		cursorDomain += "-goal-deadlines"
	}
	if request.GoalReminders {
		cursorDomain += "-goal-reminders"
	}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID ||
			payload.Domain != cursorDomain || payload.Snapshot.After(now) {
			return nil, "", 0, ports.ErrInvalidArgument
		}
		request.AfterID = payload.AfterID
		request.AfterCreated = payload.AfterCreated
		request.Snapshot = payload.Snapshot
	}
	page, err := service.Invitations.ListNotifications(ctx, principal.UserID, request)
	if err != nil {
		return nil, "", 0, err
	}
	if !validNotificationPage(page, limit, request.Snapshot) {
		return nil, "", 0, errInvalidInvitationDependencies
	}
	for _, item := range page.Items {
		if item.Kind == NotificationGoalPracticeReminder && !request.GoalReminders {
			return nil, "", 0, ports.ErrUnavailable
		}
		if err := service.authorizeTimerNotification(ctx, principal.UserID, item); err != nil {
			// Never expose a partial page or unread count from a stale permission
			// projection. Its next refresh must reconcile with authoritative access.
			return nil, "", 0, ports.ErrUnavailable
		}
	}
	nextCursor := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		nextCursor, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{
			Version: 1, Owner: principal.UserID, Domain: cursorDomain,
			AfterID: last.ID, AfterCreated: last.CreatedAt, Snapshot: request.Snapshot,
		})
		if err != nil {
			return nil, "", 0, err
		}
	}
	event := shared.NewAuditEvent(
		ctx, service.Clock, principal.UserID, principal.UserID,
		audit.ResourceListed, "notification", "history", audit.Succeeded,
	)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", 0, err
	}
	return page.Items, nextCursor, page.UnreadCount, nil
}
