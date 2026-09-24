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

func TestRejectInvitationAtomicallyConsumesWithoutGrantingAccess(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	invitation := domain.Invitation{
		ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
		RecipientUserID: "recipient", OfferedRole: domain.RoleParticipant, CreatedAt: now.Add(-time.Hour),
	}
	rejected := invitation
	rejected.RejectedAt = now
	var commands []RejectInvitationCommand
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.Invitations = controlledInvitationRepository{
		rejectionDecision: RejectionDecision{Invitation: invitation, OwnerUserID: "creator"},
		rejectCommands:    &commands,
		rejectResult:      RejectInvitationResult{Invitation: rejected},
	}

	result, err := NewInvitationService(dependencies).Reject(
		context.Background(), "Bearer valid", "reject-key-000001", invitation.ID,
	)
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if result.Replayed || result.Invitation != rejected {
		t.Fatalf("Reject() = %+v, want rejected invitation", result)
	}
	if len(commands) != 1 {
		t.Fatalf("reject commands = %d, want 1", len(commands))
	}
	command := commands[0]
	if command.Invitation != rejected || command.Idempotency.PrincipalID != "recipient" ||
		command.Idempotency.Operation != RejectInvitationOperation ||
		command.Idempotency.Key != "reject-key-000001" || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("reject command = %+v", command)
	}
	if command.Audit.Action != audit.PathInvitationRejected || command.Audit.Outcome != audit.Succeeded ||
		command.Audit.OwnerUserID != "creator" || command.Audit.ActorUserID != "recipient" ||
		command.Audit.TargetType != "path_invitation" || command.Audit.TargetID != "invitation-1" ||
		command.Audit.OccurredAt != now {
		t.Fatalf("rejection audit = %+v", command.Audit)
	}
}

func TestRejectInvitationReplaysAndHidesUnavailableOrRaceLoss(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	rejected := domain.Invitation{
		ID: "invitation-1", PathID: "path-1", InviterUserID: "creator", RecipientUserID: "recipient",
		OfferedRole: domain.RoleSupporter, CreatedAt: now.Add(-time.Hour), RejectedAt: now,
	}
	for _, test := range []struct {
		name       string
		repository controlledInvitationRepository
		wantReplay bool
		wantErr    error
		wantWrites int
	}{
		{
			name: "same-key replay",
			repository: controlledInvitationRepository{rejectionDecision: RejectionDecision{
				Replay: &RejectInvitationResult{Invitation: rejected, Replayed: true},
			}},
			wantReplay: true,
		},
		{
			name:       "unavailable is opaque",
			repository: controlledInvitationRepository{err: domain.ErrInvitationUnavailable},
			wantErr:    domain.ErrInvitationUnavailable,
		},
		{
			name: "accept race loss is opaque",
			repository: controlledInvitationRepository{
				rejectionDecision: RejectionDecision{Invitation: domain.Invitation{
					ID: "invitation-1", PathID: "path-1", InviterUserID: "creator", RecipientUserID: "recipient",
					OfferedRole: domain.RoleSupporter, CreatedAt: now.Add(-time.Hour),
				}, OwnerUserID: "creator"},
				rejectErr: domain.ErrInvitationUnavailable,
			},
			wantErr: domain.ErrInvitationUnavailable, wantWrites: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var commands []RejectInvitationCommand
			var events []audit.Event
			test.repository.rejectCommands = &commands
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.Invitations = test.repository
			dependencies.Audits = controlledAudits{events: &events}
			result, err := NewInvitationService(dependencies).Reject(
				context.Background(), "Bearer valid", "reject-key-000001", "invitation-1",
			)
			if !errors.Is(err, test.wantErr) || result.Replayed != test.wantReplay {
				t.Fatalf("Reject() = %+v, %v; want replay=%t error=%v", result, err, test.wantReplay, test.wantErr)
			}
			if len(commands) != test.wantWrites {
				t.Fatalf("Reject() writes = %d, want %d", len(commands), test.wantWrites)
			}
			if test.wantErr != nil && len(events) != 1 {
				t.Fatalf("denial audits = %d, want 1", len(events))
			}
		})
	}
}
