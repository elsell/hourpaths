package path

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestGoalReminderHistoryChecksEveryPathAndFailsClosed(t *testing.T) {
	now := time.Date(2026, 10, 9, 22, 0, 0, 0, time.UTC)
	d := invitationDependencies(now)
	d.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	notice := InvitationNotificationProjection{ID: "reminder", Kind: NotificationGoalPracticeReminder, Presentation: NotificationInformational, CreatedAt: now, Actor: InvitationPublicIdentity{UserID: "recipient", Username: "reader", DisplayName: "Reader"}, Reminder: &GoalReminderBundle{Paths: []GoalReminderPath{{ID: "path-a", Name: "Reading"}, {ID: "path-b", Name: "Piano"}}}}
	d.Invitations = controlledInvitationRepository{notificationPage: NotificationPage{Items: []InvitationNotificationProjection{notice}, UnreadCount: 1}}
	var calls []authorizationCall
	d.Authorizer = controlledAuthorizer{allowed: true, calls: &calls}
	ctx := WithNotificationGoalReminderRepresentation(context.Background(), true)
	items, _, count, err := NewInvitationService(d).ListNotifications(ctx, "Bearer valid", "", 25)
	if err != nil || len(items) != 1 || count != 1 || len(calls) != 2 {
		t.Fatalf("bundle=%+v count=%d calls=%+v err=%v", items, count, calls, err)
	}
	for i, id := range []string{"path-a", "path-b"} {
		if calls[i] != (authorizationCall{"path", id, "view", "recipient"}) {
			t.Fatalf("wrong permission %+v", calls[i])
		}
	}
	for _, a := range []controlledAuthorizer{{allowed: false}, {err: errors.New("permission service unavailable")}} {
		d.Authorizer = a
		if items, _, count, err := NewInvitationService(d).ListNotifications(ctx, "Bearer valid", "", 25); err == nil || len(items) != 0 || count != 0 {
			t.Fatalf("failed open %+v %d %v", items, count, err)
		}
	}
	d.Authorizer = controlledAuthorizer{allowed: true}
	notice.Actor.UserID = "foreign"
	d.Invitations = controlledInvitationRepository{notificationPage: NotificationPage{Items: []InvitationNotificationProjection{notice}}}
	if items, _, _, err := NewInvitationService(d).ListNotifications(ctx, "Bearer valid", "", 25); err == nil || len(items) != 0 {
		t.Fatal("foreign bundle exposed")
	}
}
