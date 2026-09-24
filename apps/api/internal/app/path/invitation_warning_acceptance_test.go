package path

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestAcceptInvitationWarningAndOpaqueFailuresLeaveInvitationPending(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	invitation := domain.Invitation{
		ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
		RecipientUserID: "recipient", OfferedRole: domain.RoleParticipant, CreatedAt: now.Add(-time.Hour),
	}
	tests := []struct {
		name    string
		mutate  func(*InvitationDependencies)
		wantErr error
	}{
		{"warning required", func(d *InvitationDependencies) {
			d.WarningPolicy = controlledInvitationWarningPolicy{decision: InvitationWarningDecision{Required: true, Context: InvitationWarningContext{PathVisibility: "followers"}}}
		}, ErrInvitationWarningRequired},
		{"wrong recipient opaque", func(d *InvitationDependencies) {
			d.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "intruder", Scopes: []string{"api:user"}}}
		}, domain.ErrInvitationUnavailable},
		{"rate limited", func(d *InvitationDependencies) {
			d.AuditRateLimiter = controlledLimiter{denied: true}
		}, platformapp.ErrRateLimited},
		{"missing repository", func(d *InvitationDependencies) {
			d.Invitations = nil
		}, errInvalidInvitationDependencies},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var commands []AcceptInvitationCommand
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.Invitations = controlledInvitationRepository{
				decision: InvitationDecision{
					Invitation: invitation, Path: invitationTestPath(now),
					RecipientVisibility: identity.ProfileVisibilityPrivate,
				},
				acceptCommands: &commands,
			}
			test.mutate(&dependencies)
			_, err := NewInvitationService(dependencies).Accept(context.Background(), "Bearer valid", "accept-key-000001", invitation.ID)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Accept() error = %v, want %v", err, test.wantErr)
			}
			if len(commands) != 0 {
				t.Fatalf("Accept() wrote %d commands", len(commands))
			}
		})
	}
}

func TestAcceptInvitationRequiresCurrentVisibilityAcknowledgementBeforeWriting(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	invitation := domain.Invitation{
		ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
		RecipientUserID: "recipient", OfferedRole: domain.RoleParticipant, CreatedAt: now.Add(-time.Hour),
	}
	tests := []struct {
		name            string
		acknowledgement InvitationWarningAcknowledgement
		wantErr         error
	}{
		{name: "missing acknowledgement", wantErr: ErrInvitationWarningRequired},
		{
			name:            "stale followers acknowledgement after Path becomes public",
			acknowledgement: InvitationWarningAcknowledgement{PathVisibility: "followers"},
			wantErr:         ErrInvitationWarningRequired,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var commands []AcceptInvitationCommand
			var warningCalls []InvitationWarningInput
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.Invitations = controlledInvitationRepository{
				decision: InvitationDecision{
					Invitation: invitation,
					Path: func() domain.Entity {
						path := invitationTestPath(now)
						path.Visibility = "public"
						return path
					}(),
					RecipientVisibility: identity.ProfileVisibilityPrivate,
					HasRetainedActivity: true,
				},
				acceptCommands: &commands,
			}
			dependencies.WarningPolicy = controlledInvitationWarningPolicy{
				decision: InvitationWarningDecision{
					Required: true,
					Context: InvitationWarningContext{
						PathVisibility:      "public",
						HasRetainedActivity: true,
					},
				},
				calls: &warningCalls,
			}

			_, err := NewInvitationService(dependencies).AcceptConfirmed(
				context.Background(), "Bearer valid", "accept-key-000001", invitation.ID, test.acknowledgement,
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Accept() error = %v, want %v", err, test.wantErr)
			}
			var warningErr *InvitationWarningRequiredError
			if !errors.As(err, &warningErr) || warningErr.Context != (InvitationWarningContext{
				PathVisibility: "public", HasRetainedActivity: true,
			}) {
				t.Fatalf("Accept() warning = %#v, want current public retained-activity context", err)
			}
			if len(warningCalls) != 1 || !warningCalls[0].HasRetainedActivity ||
				warningCalls[0].PathVisibility != "public" {
				t.Fatalf("warning policy calls = %+v", warningCalls)
			}
			if len(commands) != 0 {
				t.Fatalf("Accept() wrote %d commands before a current acknowledgement", len(commands))
			}
		})
	}
}

func TestAcceptInvitationRequestHashBindsWarningAcknowledgement(t *testing.T) {
	absent := canonicalAcceptInvitationRequestHash("invitation-1", InvitationWarningAcknowledgement{})
	followers := canonicalAcceptInvitationRequestHash(
		"invitation-1",
		InvitationWarningAcknowledgement{PathVisibility: "followers"},
	)
	public := canonicalAcceptInvitationRequestHash(
		"invitation-1",
		InvitationWarningAcknowledgement{PathVisibility: "public"},
	)

	if absent == followers || absent == public || followers == public {
		t.Fatalf(
			"acknowledgement hashes must be distinct: absent=%x followers=%x public=%x",
			absent, followers, public,
		)
	}
}

func TestAcceptInvitationWarningBypassesAndCurrentAcknowledgementProceed(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name                string
		role                domain.MembershipRole
		pathVisibility      string
		recipientVisibility identity.ProfileVisibility
		acknowledgement     InvitationWarningAcknowledgement
		warningRequired     bool
	}{
		{
			name: "current public acknowledgement",
			role: domain.RoleParticipant, pathVisibility: "public",
			recipientVisibility: identity.ProfileVisibilityPrivate,
			acknowledgement:     InvitationWarningAcknowledgement{PathVisibility: "public"},
			warningRequired:     true,
		},
		{
			name: "supporter bypass",
			role: domain.RoleSupporter, pathVisibility: "public",
			recipientVisibility: identity.ProfileVisibilityPrivate,
		},
		{
			name: "private Path bypass",
			role: domain.RoleParticipant, pathVisibility: "private",
			recipientVisibility: identity.ProfileVisibilityPrivate,
		},
		{
			name: "public profile bypass",
			role: domain.RoleParticipant, pathVisibility: "public",
			recipientVisibility: identity.ProfileVisibilityPublic,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invitation := domain.Invitation{
				ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
				RecipientUserID: "recipient", OfferedRole: test.role, CreatedAt: now.Add(-time.Hour),
			}
			accepted := invitation
			accepted.AcceptedAt = now
			path := invitationTestPath(now)
			path.Visibility = test.pathVisibility
			change := ports.AuthorizationChange{
				ID: "authorization-change-1", ResourceType: "path", ResourceID: "path-1",
				Relation: string(test.role), SubjectType: "user", SubjectID: "recipient",
				OwnerUserID: "creator", ActorUserID: "recipient", Operation: ports.AuthorizationTouch,
				LockedBy: "invitation-worker", Lease: time.Minute,
			}
			var commands []AcceptInvitationCommand
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.NewID = sequentialIDs("authorization-change-1", "acceptance-notification-1")
			dependencies.Invitations = controlledInvitationRepository{
				decision: InvitationDecision{
					Invitation: invitation, Path: path, RecipientVisibility: test.recipientVisibility,
				},
				acceptCommands: &commands,
				acceptResult:   AcceptInvitationResult{Invitation: accepted, AuthorizationChange: change},
			}
			dependencies.WarningPolicy = controlledInvitationWarningPolicy{
				decision: InvitationWarningDecision{
					Required: test.warningRequired,
					Context:  InvitationWarningContext{PathVisibility: test.pathVisibility},
				},
			}
			dependencies.AuthorizationOutbox = normalAcceptanceOutbox{
				renewed: new(string), completedChange: new(string), completed: new(audit.Event),
			}

			if _, err := NewInvitationService(dependencies).AcceptConfirmed(
				context.Background(), "Bearer valid", "accept-key-000001", invitation.ID, test.acknowledgement,
			); err != nil {
				t.Fatalf("Accept() error = %v", err)
			}
			if len(commands) != 1 {
				t.Fatalf("Accept() writes = %d, want 1", len(commands))
			}
		})
	}
}
