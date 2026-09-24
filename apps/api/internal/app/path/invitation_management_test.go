package path

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledInvitationManagement struct {
	page     ManagedInvitationPage
	decision CancellationDecision
	cancel   CancelInvitationResult
	commands *[]CancelInvitationCommand
	actor    *string
	pathID   *domain.ID
	pageReq  *InvitationPageRequest
	err      error
}

func TestManagerInvitationAccessDeniesOrdinaryMembersAndArchivedCancellation(t *testing.T) {
	now := time.Date(2026, 8, 8, 14, 0, 0, 0, time.UTC)
	for _, operation := range []string{"list", "cancel"} {
		t.Run("denied "+operation, func(t *testing.T) {
			var events []audit.Event
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "participant", Scopes: []string{"api:user"}}}
			dependencies.Authorizer = controlledAuthorizer{allowed: false}
			dependencies.InvitationManagement = controlledInvitationManagement{}
			dependencies.Audits = controlledAudits{events: &events}
			service := NewInvitationService(dependencies)
			var err error
			if operation == "list" {
				_, _, err = service.ListManagedPending(context.Background(), "Bearer valid", "path-1", "", 25)
			} else {
				_, err = service.Cancel(context.Background(), "Bearer valid", "cancel-key-000001", "path-1", "invitation-1")
			}
			if !errors.Is(err, platformapp.ErrForbidden) || len(events) != 1 || events[0].Action != audit.ResourceAccessDenied {
				t.Fatalf("error=%v events=%+v", err, events)
			}
		})
	}
	archived, err := invitationTestPath(now).Archive(now)
	if err != nil {
		t.Fatal(err)
	}
	var listedActor string
	dependencies := invitationDependencies(now.Add(time.Minute))
	dependencies.Paths = controlledRepository{entity: archived}
	dependencies.InvitationManagement = controlledInvitationManagement{actor: &listedActor}
	service := NewInvitationService(dependencies)
	if _, _, err := service.ListManagedPending(context.Background(), "Bearer valid", archived.ID, "", 25); err != nil || listedActor != "creator" {
		t.Fatalf("archived list actor=%q error=%v", listedActor, err)
	}
	if _, err := service.Cancel(context.Background(), "Bearer valid", "cancel-key-000001", archived.ID, "invitation-1"); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("archived Cancel() error=%v", err)
	}
}

func (repository controlledInvitationManagement) ListManagedPending(_ context.Context, actor string, pathID domain.ID, request InvitationPageRequest) (ManagedInvitationPage, error) {
	if repository.actor != nil {
		*repository.actor = actor
	}
	if repository.pathID != nil {
		*repository.pathID = pathID
	}
	if repository.pageReq != nil {
		*repository.pageReq = request
	}
	return repository.page, repository.err
}
func (repository controlledInvitationManagement) CancellationDecision(_ context.Context, actor string, pathID domain.ID, invitationID domain.InvitationID, _ ports.Idempotency) (CancellationDecision, error) {
	return repository.decision, repository.err
}
func (repository controlledInvitationManagement) Cancel(_ context.Context, command CancelInvitationCommand) (CancelInvitationResult, error) {
	if repository.commands != nil {
		*repository.commands = append(*repository.commands, command)
	}
	return repository.cancel, repository.err
}

func TestManagerListsPendingInvitationsAndCancelsAcrossSenders(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 123456000, time.UTC)
	path := invitationTestPath(now)
	pending := domain.Invitation{ID: "invitation-1", PathID: path.ID, InviterUserID: "other-admin", RecipientUserID: "recipient", OfferedRole: domain.RoleSupporter, CreatedAt: now.Add(-time.Hour)}
	managed := ManagedInvitation{Invitation: pending, Inviter: InvitationPublicIdentity{UserID: "other-admin", Username: "sender", DisplayName: "Sender"}, Recipient: InvitationPublicIdentity{UserID: "recipient", Username: "reader", DisplayName: "Reader"}}
	var actor string
	var listedPath domain.ID
	var pageRequest InvitationPageRequest
	var commands []CancelInvitationCommand
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "administrator", Scopes: []string{"api:user"}}}
	dependencies.Paths = controlledRepository{entity: path}
	dependencies.InvitationManagement = controlledInvitationManagement{page: ManagedInvitationPage{Items: []ManagedInvitation{managed}}, decision: CancellationDecision{Invitation: pending, Path: path}, cancel: CancelInvitationResult{Invitation: domain.Invitation{ID: pending.ID, PathID: pending.PathID, InviterUserID: pending.InviterUserID, RecipientUserID: pending.RecipientUserID, OfferedRole: pending.OfferedRole, CreatedAt: pending.CreatedAt, CanceledAt: now}}, commands: &commands, actor: &actor, pathID: &listedPath, pageReq: &pageRequest}
	service := NewInvitationService(dependencies)
	items, next, err := service.ListManagedPending(context.Background(), "Bearer valid", path.ID, "", 25)
	if err != nil || next != "" || len(items) != 1 || items[0] != managed || actor != "administrator" || listedPath != path.ID || pageRequest.Limit != 25 {
		t.Fatalf("ListManagedPending() = %+v %q %v actor=%q path=%q page=%+v", items, next, err, actor, listedPath, pageRequest)
	}
	result, err := service.Cancel(context.Background(), "Bearer valid", "cancel-key-000001", path.ID, pending.ID)
	if err != nil || result.Invitation.CanceledAt != now || len(commands) != 1 {
		t.Fatalf("Cancel() = %+v %v commands=%+v", result, err, commands)
	}
	command := commands[0]
	if command.ActorUserID != "administrator" || command.Idempotency.Operation != CancelInvitationOperation || command.Audit.Action != audit.PathInvitationCanceled || command.Audit.OwnerUserID != "creator" || command.Audit.ActorUserID != "administrator" {
		t.Fatalf("cancel command=%+v", command)
	}
}
