package path

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestListInvitationNotificationsIsRecipientScopedNewestFirstRateLimitedAndAudited(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	var users []string
	var requests []NotificationPageRequest
	var events []audit.Event
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.Invitations = controlledInvitationRepository{
		notificationPage: NotificationPage{Items: []InvitationNotificationProjection{{
			ID: "notification-2", Kind: NotificationPathInvitationAccepted,
			Presentation: NotificationInformational, Read: true, CreatedAt: now.Add(-time.Minute),
			Actor:  InvitationPublicIdentity{UserID: "actor", Username: "Reader.One", DisplayName: "Reader One"},
			PathID: "path-1", PathName: "Reading", InvitationID: "invitation-1",
			OfferedRole: domain.RoleParticipant,
		}}, HasMore: true, UnreadCount: 7},
		notificationUsers: &users, notificationRequests: &requests,
	}
	dependencies.Audits = controlledAudits{events: &events}

	items, cursor, unreadCount, err := NewInvitationService(dependencies).ListNotifications(
		context.Background(), "Bearer valid", "", 25,
	)
	if err != nil {
		t.Fatalf("ListNotifications() error = %v", err)
	}
	if len(items) != 1 || len(users) != 1 || users[0] != "recipient" {
		t.Fatalf("ListNotifications() = %+v, users = %v", items, users)
	}
	if unreadCount != 7 {
		t.Fatalf("ListNotifications() unread count = %d, want 7", unreadCount)
	}
	if len(requests) != 1 || requests[0].Limit != 25 || !requests[0].Snapshot.Equal(now) {
		t.Fatalf("notification page requests = %+v", requests)
	}
	payload, err := shared.DecodeCursor(dependencies.CursorSigningKey, cursor)
	if err != nil || payload.Owner != "recipient" || payload.Domain != "path-notification" ||
		payload.AfterID != "notification-2" || !payload.AfterCreated.Equal(now.Add(-time.Minute)) ||
		!payload.Snapshot.Equal(now) {
		t.Fatalf("notification cursor = %+v, %v", payload, err)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceListed ||
		events[0].OwnerUserID != "recipient" || events[0].ActorUserID != "recipient" ||
		events[0].TargetType != "notification" || events[0].TargetID != "history" {
		t.Fatalf("notification list audit = %+v", events)
	}
}

func TestListInvitationNotificationsRejectsForeignMalformedAndCrossDomainCursors(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	for _, cursor := range []string{
		"malformed",
		mustCursor(t, dependencies.CursorSigningKey, "other", "path-notification"),
		mustCursor(t, dependencies.CursorSigningKey, "recipient", "path-invitation"),
	} {
		if _, _, _, err := NewInvitationService(dependencies).ListNotifications(
			context.Background(), "Bearer valid", cursor, 25,
		); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("ListNotifications(cursor=%q) error = %v, want invalid argument", cursor, err)
		}
	}

	valid, err := shared.EncodeCursor(dependencies.CursorSigningKey, shared.CursorPayload{
		Version: 1, Owner: "recipient", Domain: "path-notification",
		AfterID: "notification-2", AfterCreated: now.Add(-time.Hour), Snapshot: now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	var requests []NotificationPageRequest
	dependencies.Invitations = controlledInvitationRepository{notificationRequests: &requests}
	if _, _, _, err := NewInvitationService(dependencies).ListNotifications(
		context.Background(), "Bearer valid", valid, 50,
	); err != nil {
		t.Fatalf("valid cursor error = %v", err)
	}
	if len(requests) != 1 || requests[0].AfterID != "notification-2" ||
		!requests[0].AfterCreated.Equal(now.Add(-time.Hour)) ||
		!requests[0].Snapshot.Equal(now.Add(-time.Minute)) ||
		requests[0].Limit != 50 {
		t.Fatalf("forwarded notification cursor = %+v", requests)
	}
}

func TestListInvitationNotificationsFailsClosedForInvalidProjectionAndDependencies(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	valid := InvitationNotificationProjection{
		ID: "notification-1", Kind: NotificationPathInvitationReceived,
		Presentation: NotificationActionable, CreatedAt: now,
		Actor:  InvitationPublicIdentity{UserID: "actor", Username: "Reader.One", DisplayName: "Reader One"},
		PathID: "path-1", PathName: "Reading", InvitationID: "invitation-1",
		OfferedRole: domain.RoleSupporter,
	}
	negativeCountDependencies := invitationDependencies(now)
	negativeCountDependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	negativeCountDependencies.Invitations = controlledInvitationRepository{
		notificationPage: NotificationPage{Items: []InvitationNotificationProjection{valid}, UnreadCount: -1},
	}
	if _, _, _, err := NewInvitationService(negativeCountDependencies).ListNotifications(
		context.Background(), "Bearer valid", "", 25,
	); !errors.Is(err, errInvalidInvitationDependencies) {
		t.Fatalf("negative unread count error = %v, want invalid dependencies", err)
	}
	for name, mutate := range map[string]func(*InvitationNotificationProjection){
		"unknown kind":       func(item *InvitationNotificationProjection) { item.Kind = "unknown" },
		"wrong presentation": func(item *InvitationNotificationProjection) { item.Presentation = NotificationInformational },
		"missing actor":      func(item *InvitationNotificationProjection) { item.Actor.UserID = "" },
		"missing path name":  func(item *InvitationNotificationProjection) { item.PathName = "" },
		"unsupported role":   func(item *InvitationNotificationProjection) { item.OfferedRole = "administrator" },
		"zero creation time": func(item *InvitationNotificationProjection) { item.CreatedAt = time.Time{} },
	} {
		t.Run(name, func(t *testing.T) {
			item := valid
			mutate(&item)
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.Invitations = controlledInvitationRepository{
				notificationPage: NotificationPage{Items: []InvitationNotificationProjection{item}},
			}
			if _, _, _, err := NewInvitationService(dependencies).ListNotifications(
				context.Background(), "Bearer valid", "", 25,
			); !errors.Is(err, errInvalidInvitationDependencies) {
				t.Fatalf("invalid notification error = %v, want invalid dependencies", err)
			}
		})
	}

	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.AuditRateLimiter = controlledLimiter{denied: true}
	if _, _, _, err := NewInvitationService(dependencies).ListNotifications(
		context.Background(), "Bearer valid", "", 25,
	); !errors.Is(err, platformapp.ErrRateLimited) {
		t.Fatalf("rate-limited notification list error = %v", err)
	}

	auditFailure := errors.New("audit unavailable")
	dependencies = invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.Audits = controlledAudits{err: auditFailure}
	if _, _, _, err := NewInvitationService(dependencies).ListNotifications(
		context.Background(), "Bearer valid", "", 25,
	); !errors.Is(err, auditFailure) {
		t.Fatalf("unaudited notification list error = %v", err)
	}

	dependencies = invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	older := valid
	older.ID, older.CreatedAt = "notification-older", now.Add(-time.Minute)
	dependencies.Invitations = controlledInvitationRepository{
		notificationPage: NotificationPage{Items: []InvitationNotificationProjection{older, valid}},
	}
	if _, _, _, err := NewInvitationService(dependencies).ListNotifications(
		context.Background(), "Bearer valid", "", 25,
	); !errors.Is(err, errInvalidInvitationDependencies) {
		t.Fatalf("out-of-order notification page error = %v", err)
	}
}

func TestNotificationPageAcceptsAdministratorRoleChangesButNotAdministratorInvitationsOrRemovals(t *testing.T) {
	now := time.Date(2026, 8, 3, 20, 0, 0, 0, time.UTC)
	base := InvitationNotificationProjection{ID: "administrator-notice", Kind: NotificationPathMemberRoleChanged, Presentation: NotificationInformational, CreatedAt: now, Actor: InvitationPublicIdentity{UserID: "creator", Username: "Creator.One", DisplayName: "Creator One"}, PathID: "path-1", PathName: "Reading", OfferedRole: domain.RoleAdministrator}
	if !validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{base}}, 25, now) {
		t.Fatal("administrator grant notification was rejected")
	}
	for _, kind := range []InvitationNotificationKind{NotificationPathInvitationReceived, NotificationPathMemberRemoved} {
		invalid := base
		invalid.Kind = kind
		if kind == NotificationPathInvitationReceived {
			invalid.Presentation = NotificationActionable
			invalid.InvitationID = "invitation-1"
		}
		if validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{invalid}}, 25, now) {
			t.Fatalf("administrator role accepted for %s", kind)
		}
	}
}

func TestNotificationPageAcceptsOnlyStrictVisibilityPayload(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	item := InvitationNotificationProjection{ID: "visibility-notice", Kind: NotificationPathVisibilityChanged, Presentation: NotificationInformational, CreatedAt: now, Actor: InvitationPublicIdentity{UserID: "creator", Username: "creator", DisplayName: "Creator"}, PathID: "path-1", PathName: "Read", PathVisibility: "followers"}
	if !validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{item}}, 25, now) {
		t.Fatal("valid visibility notice rejected")
	}
	item.PathVisibility = "friends"
	if validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{item}}, 25, now) {
		t.Fatal("unknown visibility accepted")
	}
	item.PathVisibility, item.Kind = "followers", NotificationPathMemberLeft
	if validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{item}}, 25, now) {
		t.Fatal("visibility payload crossed kind")
	}
}

func TestNotificationPageAcceptsMemberLeftNotice(t *testing.T) {
	now := time.Date(2026, 8, 9, 15, 35, 0, 0, time.UTC)
	item := InvitationNotificationProjection{
		ID: "member-left-notice", Kind: NotificationPathMemberLeft,
		Presentation: NotificationInformational, CreatedAt: now,
		Actor:  InvitationPublicIdentity{UserID: "participant", Username: "participant", DisplayName: "Participant"},
		PathID: "path-1", PathName: "Reading",
	}
	if !validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{item}, UnreadCount: 1}, 25, now) {
		t.Fatal("member-left notification was rejected")
	}
	item.OfferedRole = domain.RoleParticipant
	if validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{item}, UnreadCount: 1}, 25, now) {
		t.Fatal("member-left notification accepted an unrelated role payload")
	}
}
