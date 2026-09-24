package path

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (repository controlledInvitationRepository) GetNotification(
	_ context.Context,
	recipientUserID, notificationID string,
) (InvitationNotificationProjection, error) {
	if repository.notificationUsers != nil {
		*repository.notificationUsers = append(*repository.notificationUsers, recipientUserID+"|"+notificationID)
	}
	if repository.err != nil {
		return InvitationNotificationProjection{}, repository.err
	}
	for _, item := range repository.notificationPage.Items {
		if item.ID == notificationID {
			return item, nil
		}
	}
	return InvitationNotificationProjection{}, ports.ErrNotFound
}

func TestGetNotificationResolvesOneRecipientScopedProjectionAndAuditsRead(t *testing.T) {
	now := time.Date(2026, 7, 24, 1, 0, 0, 0, time.UTC)
	item := InvitationNotificationProjection{
		ID: "notification-1", Kind: NotificationPathInvitationReceived,
		Presentation: NotificationActionable, CreatedAt: now.Add(-time.Minute),
		Actor: InvitationPublicIdentity{
			UserID: "creator", Username: "practice.owner", DisplayName: "Practice Owner",
		},
		PathID: "path-1", PathName: "Violin",
		InvitationID: "invitation-1", OfferedRole: domain.RoleParticipant,
	}
	var reads []string
	var events []audit.Event
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{
		principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}},
	}
	dependencies.Invitations = controlledInvitationRepository{
		notificationPage:  NotificationPage{Items: []InvitationNotificationProjection{item}},
		notificationUsers: &reads,
	}
	dependencies.Audits = controlledAudits{events: &events}

	got, err := NewInvitationService(dependencies).GetNotification(
		context.Background(), "Bearer valid", "notification-1",
	)
	if err != nil {
		t.Fatalf("GetNotification() error = %v", err)
	}
	if got != item || len(reads) != 1 || reads[0] != "recipient|notification-1" {
		t.Fatalf("GetNotification() = %+v, reads=%v", got, reads)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceViewed ||
		events[0].TargetType != "notification" || events[0].TargetID != "notification-1" ||
		events[0].OwnerUserID != "recipient" || events[0].ActorUserID != "recipient" {
		t.Fatalf("read audit = %+v", events)
	}
}

func TestGetNotificationAuditsOpaqueMissingOrForeignResult(t *testing.T) {
	now := time.Date(2026, 7, 24, 1, 1, 0, 0, time.UTC)
	var events []audit.Event
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{
		principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}},
	}
	dependencies.Invitations = controlledInvitationRepository{err: ports.ErrNotFound}
	dependencies.Audits = controlledAudits{events: &events}

	_, err := NewInvitationService(dependencies).GetNotification(
		context.Background(), "Bearer valid", "notification-foreign",
	)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetNotification() error = %v, want opaque not found", err)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceAccessDenied ||
		events[0].Outcome != audit.Denied ||
		events[0].TargetType != "notification" ||
		events[0].TargetID != "notification-foreign" {
		t.Fatalf("denied audit = %+v", events)
	}
}

func TestGetNotificationAuditsPreEligibilityResultAsOpaqueNotFound(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	item := commentHeartNotification(now.Add(time.Second))
	var events []audit.Event
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{
		principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}},
	}
	dependencies.Invitations = controlledInvitationRepository{
		notificationPage: NotificationPage{Items: []InvitationNotificationProjection{item}},
	}
	dependencies.Audits = controlledAudits{events: &events}

	_, err := NewInvitationService(dependencies).GetNotification(
		context.Background(), "Bearer valid", item.ID,
	)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetNotification() error = %v, want opaque not found", err)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceAccessDenied ||
		events[0].Outcome != audit.Denied || events[0].TargetType != "notification" ||
		events[0].TargetID != item.ID {
		t.Fatalf("denied audit = %+v", events)
	}
}

func TestGetNotificationAcceptsOnlyStrictEligiblePracticeReactionSubjects(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	valid := practiceReactionNotification(now.Add(-time.Second))
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.Invitations = controlledInvitationRepository{notificationPage: NotificationPage{Items: []InvitationNotificationProjection{valid}}}

	got, err := NewInvitationService(dependencies).GetNotification(context.Background(), "Bearer valid", valid.ID)
	if err != nil || got != valid {
		t.Fatalf("GetNotification()=%+v err=%v", got, err)
	}

	for name, mutate := range map[string]func(*InvitationNotificationProjection){
		"missing event":    func(item *InvitationNotificationProjection) { item.SocialFeedEventID = "" },
		"invalid reaction": func(item *InvitationNotificationProjection) { item.Reaction = "discourage" },
		"foreign subject":  func(item *InvitationNotificationProjection) { item.InvitationID = "invitation-1" },
	} {
		t.Run(name, func(t *testing.T) {
			item := valid
			mutate(&item)
			invalid := dependencies
			invalid.Invitations = controlledInvitationRepository{notificationPage: NotificationPage{Items: []InvitationNotificationProjection{item}}}
			if _, err := NewInvitationService(invalid).GetNotification(context.Background(), "Bearer valid", item.ID); !errors.Is(err, errInvalidInvitationDependencies) {
				t.Fatalf("invalid reaction notification error=%v", err)
			}
		})
	}
}

func TestGetNotificationAcceptsPrivacySafeDisabledInteractionContext(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	item := practiceCommentNotification(now.Add(-time.Second))
	item.Actor = InvitationPublicIdentity{UserID: "owner", Username: "owner", DisplayName: "Owner"}
	item.CommentID = ""
	item.InteractionDisabled = InteractionDisabledComments
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.Invitations = controlledInvitationRepository{notificationPage: NotificationPage{Items: []InvitationNotificationProjection{item}}}

	got, err := NewInvitationService(dependencies).GetNotification(context.Background(), "Bearer valid", item.ID)
	if err != nil || got != item {
		t.Fatalf("GetNotification()=%+v err=%v", got, err)
	}
}
