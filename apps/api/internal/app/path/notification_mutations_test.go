package path

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (repository controlledInvitationRepository) MarkNotificationRead(_ context.Context, command NotificationMutationCommand) (NotificationMutationResult, error) {
	if repository.notificationMutations != nil {
		*repository.notificationMutations = append(*repository.notificationMutations, command)
	}
	return repository.notificationResult, repository.err
}

func (repository controlledInvitationRepository) DeleteNotification(_ context.Context, command NotificationMutationCommand) (NotificationMutationResult, error) {
	if repository.notificationMutations != nil {
		*repository.notificationMutations = append(*repository.notificationMutations, command)
	}
	return repository.notificationResult, repository.err
}

func (repository controlledInvitationRepository) MarkAllNotificationsRead(_ context.Context, command NotificationMutationCommand) (NotificationMutationResult, error) {
	if repository.notificationMutations != nil {
		*repository.notificationMutations = append(*repository.notificationMutations, command)
	}
	return repository.notificationResult, repository.err
}

func TestNotificationMutationsAreRecipientScopedRateLimitedAndAtomicallyAudited(t *testing.T) {
	now := time.Date(2026, 7, 23, 21, 0, 0, 0, time.UTC)
	for name, run := range map[string]func(*InvitationService) (NotificationMutationResult, error){
		"read": func(service *InvitationService) (NotificationMutationResult, error) {
			return service.MarkNotificationRead(context.Background(), "Bearer valid", "notification-1")
		},
		"delete": func(service *InvitationService) (NotificationMutationResult, error) {
			return service.DeleteNotification(context.Background(), "Bearer valid", "notification-1")
		},
		"mark all": func(service *InvitationService) (NotificationMutationResult, error) {
			return service.MarkAllNotificationsRead(context.Background(), "Bearer valid")
		},
	} {
		t.Run(name, func(t *testing.T) {
			var commands []NotificationMutationCommand
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.Invitations = controlledInvitationRepository{
				notificationMutations: &commands,
				notificationResult:    NotificationMutationResult{UnreadCount: 2},
			}
			result, err := run(NewInvitationService(dependencies))
			if err != nil || result.UnreadCount != 2 || len(commands) != 1 {
				t.Fatalf("mutation result=%+v commands=%+v error=%v", result, commands, err)
			}
			command := commands[0]
			wantTarget := "notification-1"
			if name == "mark all" {
				wantTarget = "history"
			}
			wantAction := audit.ResourceUpdated
			if name == "delete" {
				wantAction = audit.ResourceDeleted
			}
			if command.RecipientUserID != "recipient" || command.NotificationID != wantTarget ||
				!command.ChangedAt.Equal(now) || command.Audit.Action != wantAction ||
				command.Audit.OwnerUserID != "recipient" || command.Audit.ActorUserID != "recipient" ||
				command.Audit.TargetType != "notification" || command.Audit.TargetID != wantTarget {
				t.Fatalf("notification mutation command = %+v", command)
			}
		})
	}

	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.AuditRateLimiter = controlledLimiter{denied: true}
	if _, err := NewInvitationService(dependencies).MarkNotificationRead(
		context.Background(), "Bearer valid", "notification-1",
	); !errors.Is(err, platformapp.ErrRateLimited) {
		t.Fatalf("rate limited mutation error = %v", err)
	}
}

func TestNotificationMutationAuditsOpaqueUnavailableResult(t *testing.T) {
	now := time.Date(2026, 7, 23, 21, 0, 0, 0, time.UTC)
	var events []audit.Event
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.Invitations = controlledInvitationRepository{err: ports.ErrNotFound}
	dependencies.Audits = controlledAudits{events: &events}
	if _, err := NewInvitationService(dependencies).DeleteNotification(
		context.Background(), "Bearer valid", "foreign-or-missing",
	); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("opaque unavailable error = %v", err)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceAccessDenied ||
		events[0].Outcome != audit.Denied || events[0].OwnerUserID != "recipient" ||
		events[0].TargetType != "notification" || events[0].TargetID != "foreign-or-missing" {
		t.Fatalf("denied mutation audit = %+v", events)
	}
}
