package path

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledInvitationDirectory struct {
	recipient InvitationRecipient
	err       error
	usernames *[]string
}

func (directory controlledInvitationDirectory) ActiveByExactUsername(_ context.Context, username string) (InvitationRecipient, error) {
	if directory.usernames != nil {
		*directory.usernames = append(*directory.usernames, username)
	}
	return directory.recipient, directory.err
}

type controlledInvitationRepository struct {
	sendCommands          *[]SendInvitationCommand
	sendResult            SendInvitationResult
	pendingPage           InvitationPage
	pendingUsers          *[]string
	pendingRequests       *[]InvitationPageRequest
	notificationPage      NotificationPage
	notificationUsers     *[]string
	notificationRequests  *[]NotificationPageRequest
	notificationMutations *[]NotificationMutationCommand
	notificationResult    NotificationMutationResult
	decision              InvitationDecision
	acceptCommands        *[]AcceptInvitationCommand
	acceptResult          AcceptInvitationResult
	acceptErr             error
	rejectionDecision     RejectionDecision
	rejectCommands        *[]RejectInvitationCommand
	rejectResult          RejectInvitationResult
	rejectErr             error
	err                   error
}

func (repository controlledInvitationRepository) Send(_ context.Context, command SendInvitationCommand) (SendInvitationResult, error) {
	if repository.sendCommands != nil {
		*repository.sendCommands = append(*repository.sendCommands, command)
	}
	return repository.sendResult, repository.err
}

func (repository controlledInvitationRepository) ListPending(_ context.Context, recipientUserID string, request InvitationPageRequest) (InvitationPage, error) {
	if repository.pendingUsers != nil {
		*repository.pendingUsers = append(*repository.pendingUsers, recipientUserID)
	}
	if repository.pendingRequests != nil {
		*repository.pendingRequests = append(*repository.pendingRequests, request)
	}
	return repository.pendingPage, repository.err
}

func (repository controlledInvitationRepository) ListNotifications(_ context.Context, recipientUserID string, request NotificationPageRequest) (NotificationPage, error) {
	if repository.notificationUsers != nil {
		*repository.notificationUsers = append(*repository.notificationUsers, recipientUserID)
	}
	if repository.notificationRequests != nil {
		*repository.notificationRequests = append(*repository.notificationRequests, request)
	}
	return repository.notificationPage, repository.err
}

func (repository controlledInvitationRepository) AcceptanceDecision(_ context.Context, recipientUserID string, invitationID domain.InvitationID, _ ports.Idempotency) (InvitationDecision, error) {
	if repository.err != nil {
		return InvitationDecision{}, repository.err
	}
	if repository.decision.Invitation.RecipientUserID != recipientUserID || repository.decision.Invitation.ID != invitationID {
		return InvitationDecision{}, domain.ErrInvitationUnavailable
	}
	return repository.decision, nil
}

func (repository controlledInvitationRepository) Accept(_ context.Context, command AcceptInvitationCommand) (AcceptInvitationResult, error) {
	if repository.acceptCommands != nil {
		*repository.acceptCommands = append(*repository.acceptCommands, command)
	}
	if repository.acceptErr != nil {
		return AcceptInvitationResult{}, repository.acceptErr
	}
	return repository.acceptResult, repository.err
}

func (repository controlledInvitationRepository) RejectionDecision(_ context.Context, recipientUserID string, invitationID domain.InvitationID, _ ports.Idempotency) (RejectionDecision, error) {
	if repository.err != nil {
		return RejectionDecision{}, repository.err
	}
	if repository.rejectionDecision.Replay == nil &&
		(repository.rejectionDecision.Invitation.RecipientUserID != recipientUserID || repository.rejectionDecision.Invitation.ID != invitationID) {
		return RejectionDecision{}, domain.ErrInvitationUnavailable
	}
	return repository.rejectionDecision, nil
}

func (repository controlledInvitationRepository) Reject(_ context.Context, command RejectInvitationCommand) (RejectInvitationResult, error) {
	if repository.rejectCommands != nil {
		*repository.rejectCommands = append(*repository.rejectCommands, command)
	}
	if repository.rejectErr != nil {
		return RejectInvitationResult{}, repository.rejectErr
	}
	return repository.rejectResult, repository.err
}

type controlledInvitationWarningPolicy struct {
	decision InvitationWarningDecision
	err      error
	calls    *[]InvitationWarningInput
}

func (policy controlledInvitationWarningPolicy) Evaluate(_ context.Context, input InvitationWarningInput) (InvitationWarningDecision, error) {
	if policy.calls != nil {
		*policy.calls = append(*policy.calls, input)
	}
	return policy.decision, policy.err
}

type controlledAuthorizationStatusReader struct {
	state AuthorizationChangeState
	err   error
	ids   *[]string
}

func (reader controlledAuthorizationStatusReader) AuthorizationChangeState(_ context.Context, id string) (AuthorizationChangeState, error) {
	if reader.ids != nil {
		*reader.ids = append(*reader.ids, id)
	}
	return reader.state, reader.err
}

func invitationTestPath(now time.Time) domain.Entity {
	entity, err := domain.New("path-1", "creator", domain.Attributes{Name: "Practice", Visibility: "followers"})
	if err != nil {
		panic(err)
	}
	entity.CreatedAt, entity.UpdatedAt = now.Add(-time.Hour), now.Add(-time.Hour)
	return entity
}

func invitationDependencies(now time.Time) InvitationDependencies {
	return InvitationDependencies{
		Auth:  controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}},
		Paths: controlledRepository{entity: invitationTestPath(now)},
		Directory: controlledInvitationDirectory{recipient: InvitationRecipient{
			UserID: "recipient", Username: "reader", ProfileVisibility: identity.ProfileVisibilityPublic,
		}},
		Authorizer:              controlledAuthorizer{allowed: true},
		Invitations:             controlledInvitationRepository{},
		WarningPolicy:           controlledInvitationWarningPolicy{},
		Audits:                  controlledAudits{},
		AuditRateLimiter:        controlledLimiter{},
		Clock:                   controlledClock{now: now},
		NewID:                   func() string { return "generated-id" },
		AuthorizationOutbox:     controlledOutbox{},
		AuthorizationStatus:     controlledAuthorizationStatusReader{state: AuthorizationChangeCompleted},
		AuthorizationSerializer: controlledSerializer{},
		AuthorizationWorker:     "invitation-worker",
		AuthorizationLease:      time.Minute,
		CursorSigningKey:        []byte("0123456789abcdef0123456789abcdef"),
	}
}

func TestSendInvitationOrchestratesAuthorizedExactUsernameAtomicWrite(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	var authorizationChecks []authorizationCall
	var usernames []string
	var commands []SendInvitationCommand
	dependencies := invitationDependencies(now)
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &authorizationChecks}
	dependencies.Directory = controlledInvitationDirectory{
		recipient: InvitationRecipient{UserID: "recipient", Username: "reader", ProfileVisibility: identity.ProfileVisibilityPrivate},
		usernames: &usernames,
	}
	dependencies.Invitations = controlledInvitationRepository{
		sendCommands: &commands,
		sendResult: SendInvitationResult{Invitation: domain.Invitation{
			ID: "generated-id", PathID: "path-1", InviterUserID: "creator", RecipientUserID: "recipient",
			OfferedRole: domain.RoleParticipant, CreatedAt: now,
		}},
	}

	result, err := NewInvitationService(dependencies).Send(
		context.Background(), "Bearer valid", "send-key-00000001", "path-1", "Reader", "recipient", domain.RoleParticipant,
	)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result.Invitation.RecipientUserID != "recipient" || len(usernames) != 1 || usernames[0] != "Reader" {
		t.Fatalf("Send() = %+v, exact username lookups = %v", result, usernames)
	}
	if len(authorizationChecks) != 1 || authorizationChecks[0] != (authorizationCall{"path", "path-1", "manage_members", "creator"}) {
		t.Fatalf("authorization checks = %+v", authorizationChecks)
	}
	if len(commands) != 1 {
		t.Fatalf("send commands = %d, want 1", len(commands))
	}
	command := commands[0]
	if command.Invitation.OfferedRole != domain.RoleParticipant || command.Invitation.RecipientUserID != "recipient" ||
		command.ExpectedRecipientUsername != "Reader" ||
		command.Idempotency.PrincipalID != "creator" || command.Idempotency.Operation != SendInvitationOperation ||
		command.Idempotency.Key != "send-key-00000001" || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("send command = %+v", command)
	}
	if command.Notification.RecipientUserID != "recipient" || !command.Notification.Actionable ||
		command.Notification.InvitationID != command.Invitation.ID || command.Notification.OfferedRole != domain.RoleParticipant {
		t.Fatalf("recipient notification = %+v", command.Notification)
	}
	if command.Audit.Action != audit.PathInvitationCreated || command.Audit.Outcome != audit.Succeeded ||
		command.Audit.ActorUserID != "creator" || command.Audit.OwnerUserID != "creator" ||
		command.Audit.TargetID != string(command.Invitation.ID) {
		t.Fatalf("send audit = %+v", command.Audit)
	}
}

func TestSendInvitationFailsClosedForDenialArchivedPathRateLimitAndDependencies(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		mutate  func(*InvitationDependencies)
		wantErr error
	}{
		{"denied", func(d *InvitationDependencies) { d.Authorizer = controlledAuthorizer{} }, platformapp.ErrForbidden},
		{"archived", func(d *InvitationDependencies) {
			entity := invitationTestPath(now)
			entity.ArchivedAt = now
			d.Paths = controlledRepository{entity: entity}
		}, ports.ErrConflict},
		{"rate limited", func(d *InvitationDependencies) { d.AuditRateLimiter = controlledLimiter{denied: true} }, platformapp.ErrRateLimited},
		{"missing directory", func(d *InvitationDependencies) { d.Directory = nil }, errInvalidInvitationDependencies},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var commands []SendInvitationCommand
			var events []audit.Event
			dependencies := invitationDependencies(now)
			dependencies.Invitations = controlledInvitationRepository{sendCommands: &commands}
			dependencies.Audits = controlledAudits{events: &events}
			test.mutate(&dependencies)
			_, err := NewInvitationService(dependencies).Send(context.Background(), "Bearer valid", "send-key-00000001", "path-1", "reader", "recipient", domain.RoleParticipant)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Send() error = %v, want %v", err, test.wantErr)
			}
			if len(commands) != 0 {
				t.Fatalf("Send() wrote %d commands", len(commands))
			}
			if test.name == "denied" && (len(events) != 1 || events[0].Action != audit.ResourceAccessDenied) {
				t.Fatalf("denial audits = %+v", events)
			}
		})
	}
}

func TestSendInvitationRejectsAUsernameNowOwnedBySomeoneOtherThanReviewedRecipient(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	var commands []SendInvitationCommand
	dependencies := invitationDependencies(now)
	dependencies.Directory = controlledInvitationDirectory{recipient: InvitationRecipient{
		UserID: "replacement-user", Username: "Reader", DisplayName: "Replacement",
		ProfileVisibility: identity.ProfileVisibilityPublic,
	}}
	dependencies.Invitations = controlledInvitationRepository{sendCommands: &commands}

	_, err := NewInvitationService(dependencies).Send(
		context.Background(), "Bearer valid", "send-key-00000001",
		"path-1", "Reader", "reviewed-user", domain.RoleParticipant,
	)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("reassigned username error = %v, want opaque ErrNotFound", err)
	}
	if len(commands) != 0 {
		t.Fatalf("reassigned username wrote %d commands", len(commands))
	}
	if canonicalSendInvitationRequestHash("path-1", "Reader", "reviewed-user", domain.RoleParticipant) ==
		canonicalSendInvitationRequestHash("path-1", "Reader", "replacement-user", domain.RoleParticipant) {
		t.Fatal("send idempotency hash does not bind the reviewed recipient user ID")
	}
}

func TestListPendingInvitationsIsRecipientScopedRateLimitedAndAudited(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	invitation := domain.Invitation{ID: "invitation-1", PathID: "path-1", InviterUserID: "creator", RecipientUserID: "recipient", OfferedRole: domain.RoleParticipant, CreatedAt: now}
	var users []string
	var requests []InvitationPageRequest
	var events []audit.Event
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.Invitations = controlledInvitationRepository{
		pendingPage: InvitationPage{Items: []PendingInvitationCandidate{{
			PendingInvitation: PendingInvitation{
				Invitation: invitation, PathName: "Reading",
				Inviter: InvitationPublicIdentity{
					UserID: "creator", Username: "Creator", DisplayName: "Path Creator",
				},
			},
			WarningInput: InvitationWarningInput{
				RecipientVisibility: identity.ProfileVisibilityPrivate,
				PathVisibility:      "followers",
				OfferedRole:         domain.RoleParticipant,
				HasRetainedActivity: true,
			},
		}}, HasMore: true},
		pendingUsers: &users, pendingRequests: &requests,
	}
	dependencies.WarningPolicy = VisibilityInvitationWarningPolicy{}
	dependencies.Audits = controlledAudits{events: &events}

	items, cursor, err := NewInvitationService(dependencies).ListPending(context.Background(), "Bearer valid", "", 25)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(items) != 1 || len(users) != 1 || users[0] != "recipient" {
		t.Fatalf("ListPending() = %+v, users = %v", items, users)
	}
	if items[0].PathName != "Reading" || items[0].Inviter.Username != "Creator" ||
		items[0].Inviter.DisplayName != "Path Creator" || items[0].Warning == nil ||
		*items[0].Warning != (InvitationWarningContext{
			PathVisibility: "followers", HasRetainedActivity: true,
		}) {
		t.Fatalf("ListPending() context = %+v", items[0])
	}
	if len(requests) != 1 || !requests[0].Snapshot.Equal(now) {
		t.Fatalf("ListPending() requests = %+v, want current stable snapshot", requests)
	}
	payload, err := shared.DecodeCursor(dependencies.CursorSigningKey, cursor)
	if err != nil || payload.Owner != "recipient" || payload.Domain != "path-invitation" ||
		payload.AfterID != "invitation-1" || payload.AfterCreated != now ||
		payload.Snapshot != now {
		t.Fatalf("ListPending() cursor = %+v, %v", payload, err)
	}
	if len(events) != 1 || events[0].Action != audit.PathInvitationListed || events[0].OwnerUserID != "recipient" {
		t.Fatalf("list audit = %+v", events)
	}
}

func TestListPendingInvitationsRejectsForeignOrMalformedCursorAndForwardsValidCursor(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	for _, cursor := range []string{
		"malformed",
		mustCursor(t, dependencies.CursorSigningKey, "other", "path-invitation"),
		mustCursor(t, dependencies.CursorSigningKey, "recipient", "path"),
	} {
		if _, _, err := NewInvitationService(dependencies).ListPending(context.Background(), "Bearer valid", cursor, 25); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("ListPending(cursor=%q) error = %v, want invalid argument", cursor, err)
		}
	}

	valid, err := shared.EncodeCursor(dependencies.CursorSigningKey, shared.CursorPayload{
		Version: 1, Owner: "recipient", Domain: "path-invitation",
		AfterID: "invitation-1", AfterCreated: now.Add(-time.Hour), Snapshot: now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	var requests []InvitationPageRequest
	dependencies.Invitations = controlledInvitationRepository{pendingRequests: &requests}
	if _, cursor, err := NewInvitationService(dependencies).ListPending(context.Background(), "Bearer valid", valid, 100); err != nil || cursor != "" {
		t.Fatalf("ListPending(valid cursor) next=%q error=%v", cursor, err)
	}
	if len(requests) != 1 || requests[0].AfterID != "invitation-1" ||
		requests[0].AfterCreated != now.Add(-time.Hour) ||
		requests[0].Snapshot != now.Add(-time.Minute) || requests[0].Limit != 100 {
		t.Fatalf("repository page request = %+v", requests)
	}
}

func TestAcceptInvitationAtomicallyGrantsExactOfferedRoleAndReconcilesAuthorization(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	for _, role := range []domain.MembershipRole{domain.RoleParticipant, domain.RoleSupporter} {
		t.Run(string(role), func(t *testing.T) {
			invitation := domain.Invitation{
				ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
				RecipientUserID: "recipient", OfferedRole: role, CreatedAt: now.Add(-time.Hour),
			}
			accepted := invitation
			accepted.AcceptedAt = now
			change := ports.AuthorizationChange{
				ID: "authorization-change-1", ResourceType: "path", ResourceID: "path-1",
				Relation: string(role), SubjectType: "user", SubjectID: "recipient",
				OwnerUserID: "creator", ActorUserID: "recipient", Operation: ports.AuthorizationTouch,
				LockedBy: "invitation-worker", Lease: time.Minute,
			}
			var commands []AcceptInvitationCommand
			var writes []relationshipWrite
			var renewed, completedChange string
			var completed audit.Event
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.NewID = sequentialIDs("authorization-change-1", "acceptance-notification-1")
			dependencies.Invitations = controlledInvitationRepository{
				decision: InvitationDecision{
					Invitation: invitation, Path: invitationTestPath(now),
					RecipientVisibility: identity.ProfileVisibilityPublic,
				},
				acceptCommands: &commands,
				acceptResult: AcceptInvitationResult{
					Invitation: accepted, AuthorizationChange: change,
				},
			}
			dependencies.Authorizer = controlledAuthorizer{writes: &writes}
			dependencies.AuthorizationOutbox = normalAcceptanceOutbox{
				renewed: &renewed, completedChange: &completedChange, completed: &completed,
			}

			result, err := NewInvitationService(dependencies).Accept(
				context.Background(), "Bearer valid", "accept-key-000001", invitation.ID,
			)
			if err != nil {
				t.Fatalf("Accept() error = %v", err)
			}
			if result.Replayed || result.Invitation != accepted || result.AuthorizationChange != change {
				t.Fatalf("Accept() = %+v, want accepted invitation and exact authorization change", result)
			}
			if len(commands) != 1 {
				t.Fatalf("accept commands = %d, want 1", len(commands))
			}
			command := commands[0]
			if command.Invitation != accepted || command.AuthorizationChange != change {
				t.Fatalf("accept command invitation/change = %+v / %+v", command.Invitation, command.AuthorizationChange)
			}
			if command.Idempotency.PrincipalID != "recipient" ||
				command.Idempotency.Operation != AcceptInvitationOperation ||
				command.Idempotency.Key != "accept-key-000001" ||
				len(command.Idempotency.RequestHash) != 32 {
				t.Fatalf("accept idempotency = %+v", command.Idempotency)
			}
			if command.Notification.ID != "acceptance-notification-1" ||
				command.Notification.RecipientUserID != "creator" ||
				command.Notification.ActorUserID != "recipient" ||
				command.Notification.PathID != invitation.PathID ||
				command.Notification.InvitationID != invitation.ID ||
				command.Notification.OfferedRole != role ||
				command.Notification.Actionable ||
				command.Notification.CreatedAt != now {
				t.Fatalf("acceptance notification = %+v", command.Notification)
			}
			if command.Audit.Action != audit.PathInvitationAccepted ||
				command.Audit.Outcome != audit.Succeeded ||
				command.Audit.OwnerUserID != "creator" ||
				command.Audit.ActorUserID != "recipient" ||
				command.Audit.TargetType != "path_invitation" ||
				command.Audit.TargetID != string(invitation.ID) ||
				command.Audit.OccurredAt != now {
				t.Fatalf("acceptance audit = %+v", command.Audit)
			}
			if len(writes) != 1 || writes[0] != (relationshipWrite{
				resourceType: "path", resourceID: "path-1", relation: string(role),
				subjectType: "user", subjectID: "recipient",
			}) || renewed != "authorization-change-1/invitation-worker" ||
				completedChange != "authorization-change-1/invitation-worker" {
				t.Fatalf("relationship writes = %+v, renewed = %q, completed = %q", writes, renewed, completedChange)
			}
			if completed.Action != audit.AuthorizationApplied ||
				completed.OwnerUserID != "creator" ||
				completed.ActorUserID != "recipient" ||
				completed.TargetType != "path" ||
				completed.TargetID != "path-1" ||
				completed.Outcome != audit.Succeeded {
				t.Fatalf("authorization completion audit = %+v", completed)
			}
		})
	}
}

type normalAcceptanceOutbox struct {
	renewed, completedChange *string
	completed                *audit.Event
}

func (normalAcceptanceOutbox) ClaimAuthorizationChanges(context.Context, string, time.Duration, int) ([]ports.AuthorizationChange, error) {
	return nil, nil
}
func (normalAcceptanceOutbox) ClaimAuthorizationChange(context.Context, string, string, time.Duration) (ports.AuthorizationChange, error) {
	return ports.AuthorizationChange{}, ports.ErrNotFound
}
func (normalAcceptanceOutbox) ClaimAuthorizationChangeForResource(context.Context, string, string, string, time.Duration) (ports.AuthorizationChange, error) {
	return ports.AuthorizationChange{}, errors.New("resource claim must not be used")
}
func (outbox normalAcceptanceOutbox) RenewAuthorizationChange(_ context.Context, id, worker string, _ time.Duration) error {
	*outbox.renewed = id + "/" + worker
	return nil
}
func (outbox normalAcceptanceOutbox) CompleteAuthorizationChangeWithAudit(_ context.Context, id, worker string, event audit.Event) error {
	*outbox.completedChange = id + "/" + worker
	*outbox.completed = event
	return nil
}
func (normalAcceptanceOutbox) FailAuthorizationChange(context.Context, string, string, int, string) (bool, error) {
	return false, nil
}
func (normalAcceptanceOutbox) ListAuthorizationDeadLetters(context.Context, string, ports.PageRequest) (ports.AuthorizationDeadLetterPage, error) {
	return ports.AuthorizationDeadLetterPage{}, nil
}
func (normalAcceptanceOutbox) RequeueAuthorizationDeadLetter(context.Context, string, string, string, time.Duration, audit.Event) (ports.AuthorizationChange, error) {
	return ports.AuthorizationChange{}, nil
}

func TestAcceptInvitationGrantsExactOfferedRoleAndHealsReplayByExactChangeID(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	for _, role := range []domain.MembershipRole{domain.RoleParticipant, domain.RoleSupporter} {
		t.Run(string(role), func(t *testing.T) {
			invitation := domain.Invitation{
				ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
				RecipientUserID: "recipient", OfferedRole: role, CreatedAt: now.Add(-time.Hour),
			}
			accepted := invitation
			accepted.AcceptedAt = now
			change := ports.AuthorizationChange{
				ID: "authorization-change-1", ResourceType: "path", ResourceID: "path-1",
				Relation: string(role), SubjectType: "user", SubjectID: "recipient",
				OwnerUserID: "creator", ActorUserID: "recipient", Operation: ports.AuthorizationTouch,
				LockedBy: "invitation-worker", Lease: time.Minute,
			}
			var commands []AcceptInvitationCommand
			var claims []string
			var writes []relationshipWrite
			var completed audit.Event
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.Invitations = controlledInvitationRepository{
				decision: InvitationDecision{
					Invitation: invitation, Path: invitationTestPath(now), RecipientVisibility: identity.ProfileVisibilityPublic,
					Replay: &AcceptInvitationResult{Invitation: accepted, AuthorizationChange: change, Replayed: true},
				},
				acceptCommands: &commands,
			}
			dependencies.Authorizer = controlledAuthorizer{writes: &writes}
			dependencies.AuthorizationOutbox = exactClaimOutbox{
				change: change, claims: &claims, completed: &completed,
			}

			result, err := NewInvitationService(dependencies).Accept(context.Background(), "Bearer valid", "accept-key-000001", invitation.ID)
			if err != nil {
				t.Fatalf("Accept() error = %v", err)
			}
			if result.Invitation.AcceptedAt != now || len(commands) != 0 {
				t.Fatalf("Accept() = %+v, commands = %d", result, len(commands))
			}
			if len(claims) != 1 || claims[0] != "authorization-change-1" {
				t.Fatalf("exact replay claims = %v", claims)
			}
			if len(writes) != 1 || writes[0].relation != string(role) || writes[0].subjectID != "recipient" ||
				completed.Action != audit.AuthorizationApplied {
				t.Fatalf("writes = %+v, completion = %+v", writes, completed)
			}
		})
	}
}

func TestAcceptInvitationReplayOnlySucceedsWhenExactAuthorizationChangeIsCompleted(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	invitation := domain.Invitation{
		ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
		RecipientUserID: "recipient", OfferedRole: domain.RoleParticipant, CreatedAt: now.Add(-time.Hour),
		AcceptedAt: now,
	}
	change := ports.AuthorizationChange{
		ID: "authorization-change-1", ResourceType: "path", ResourceID: "path-1",
		Relation: string(domain.RoleParticipant), SubjectType: "user", SubjectID: "recipient",
		OwnerUserID: "creator", ActorUserID: "recipient", Operation: ports.AuthorizationTouch,
	}
	statusFailure := errors.New("authorization status unavailable")
	tests := []struct {
		name    string
		state   AuthorizationChangeState
		err     error
		wantErr error
	}{
		{name: "completed", state: AuthorizationChangeCompleted},
		{name: "pending", state: AuthorizationChangePending, wantErr: ports.ErrAuthorizationPending},
		{name: "locked", state: AuthorizationChangeLocked, wantErr: ports.ErrAuthorizationPending},
		{name: "dead lettered", state: AuthorizationChangeDeadLettered, wantErr: ports.ErrAuthorizationDeadLettered},
		{name: "missing", err: ports.ErrNotFound, wantErr: ports.ErrNotFound},
		{name: "status dependency failure", err: statusFailure, wantErr: statusFailure},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var statusIDs []string
			var commands []AcceptInvitationCommand
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.Invitations = controlledInvitationRepository{
				decision: InvitationDecision{
					Invitation: invitation, Path: invitationTestPath(now),
					RecipientVisibility: identity.ProfileVisibilityPublic,
					Replay: &AcceptInvitationResult{
						Invitation: invitation, AuthorizationChange: change, Replayed: true,
					},
				},
				acceptCommands: &commands,
			}
			dependencies.AuthorizationOutbox = normalAcceptanceOutbox{}
			dependencies.AuthorizationStatus = controlledAuthorizationStatusReader{
				state: test.state, err: test.err, ids: &statusIDs,
			}

			result, err := NewInvitationService(dependencies).Accept(
				context.Background(), "Bearer valid", "accept-key-000001", invitation.ID,
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Accept() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && (!result.Replayed || result.AuthorizationChange != change) {
				t.Fatalf("Accept() = %+v, want completed replay", result)
			}
			if len(statusIDs) != 1 || statusIDs[0] != change.ID {
				t.Fatalf("authorization status IDs = %v, want [%s]", statusIDs, change.ID)
			}
			if len(commands) != 0 {
				t.Fatalf("Accept() wrote %d commands during replay", len(commands))
			}
		})
	}
}

type exactClaimOutbox struct {
	change    ports.AuthorizationChange
	claims    *[]string
	completed *audit.Event
}

func (outbox exactClaimOutbox) ClaimAuthorizationChanges(context.Context, string, time.Duration, int) ([]ports.AuthorizationChange, error) {
	return nil, nil
}
func (outbox exactClaimOutbox) ClaimAuthorizationChange(_ context.Context, id, _ string, _ time.Duration) (ports.AuthorizationChange, error) {
	*outbox.claims = append(*outbox.claims, id)
	return outbox.change, nil
}
func (exactClaimOutbox) ClaimAuthorizationChangeForResource(context.Context, string, string, string, time.Duration) (ports.AuthorizationChange, error) {
	return ports.AuthorizationChange{}, errors.New("resource claim must not be used")
}
func (exactClaimOutbox) RenewAuthorizationChange(context.Context, string, string, time.Duration) error {
	return nil
}
func (outbox exactClaimOutbox) CompleteAuthorizationChangeWithAudit(_ context.Context, _, _ string, event audit.Event) error {
	*outbox.completed = event
	return nil
}
func (exactClaimOutbox) FailAuthorizationChange(context.Context, string, string, int, string) (bool, error) {
	return false, nil
}
func (exactClaimOutbox) ListAuthorizationDeadLetters(context.Context, string, ports.PageRequest) (ports.AuthorizationDeadLetterPage, error) {
	return ports.AuthorizationDeadLetterPage{}, nil
}
func (exactClaimOutbox) RequeueAuthorizationDeadLetter(context.Context, string, string, string, time.Duration, audit.Event) (ports.AuthorizationChange, error) {
	return ports.AuthorizationChange{}, nil
}

func TestAcceptInvitationOpaqueUnavailableIsDeniedAuditedAndAuditFailureFailsClosed(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	auditFailure := errors.New("denial audit unavailable")
	tests := []struct {
		name       string
		repository controlledInvitationRepository
		auditErr   error
		wantErr    error
	}{
		{
			name:       "repository hides missing or wrong recipient",
			repository: controlledInvitationRepository{err: domain.ErrInvitationUnavailable},
			wantErr:    domain.ErrInvitationUnavailable,
		},
		{
			name: "invalid decision is opaque",
			repository: controlledInvitationRepository{decision: InvitationDecision{
				Invitation: domain.Invitation{
					ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
					RecipientUserID: "somebody-else", OfferedRole: domain.RoleParticipant,
					CreatedAt: now.Add(-time.Hour),
				},
				Path: invitationTestPath(now), RecipientVisibility: identity.ProfileVisibilityPublic,
			}},
			wantErr: domain.ErrInvitationUnavailable,
		},
		{
			name:       "denial audit failure",
			repository: controlledInvitationRepository{err: domain.ErrInvitationUnavailable},
			auditErr:   auditFailure,
			wantErr:    auditFailure,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var events []audit.Event
			var commands []AcceptInvitationCommand
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			test.repository.acceptCommands = &commands
			dependencies.Invitations = test.repository
			dependencies.Audits = controlledAudits{events: &events, err: test.auditErr}

			_, err := NewInvitationService(dependencies).Accept(
				context.Background(), "Bearer valid", "accept-key-000001", "invitation-1",
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Accept() error = %v, want %v", err, test.wantErr)
			}
			if test.auditErr != nil && errors.Is(err, domain.ErrInvitationUnavailable) {
				t.Fatalf("Accept() leaked unavailable result when denial audit failed: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("denial audits = %+v, want exactly one", events)
			}
			event := events[0]
			if event.Action != audit.ResourceAccessDenied || event.Outcome != audit.Denied ||
				event.OwnerUserID != "recipient" || event.ActorUserID != "recipient" ||
				event.TargetType != "path_invitation" || event.TargetID != "invitation-1" ||
				event.OccurredAt != now {
				t.Fatalf("denial audit = %+v", event)
			}
			if len(commands) != 0 {
				t.Fatalf("Accept() wrote %d commands after opaque denial", len(commands))
			}
		})
	}
}

func TestAcceptInvitationRowLockRaceLossIsDeniedAuditedAndAuditFailureSupersedes(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	auditFailure := errors.New("denial audit unavailable")
	invitation := domain.Invitation{
		ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
		RecipientUserID: "recipient", OfferedRole: domain.RoleParticipant, CreatedAt: now.Add(-time.Hour),
	}
	for _, test := range []struct {
		name     string
		auditErr error
		wantErr  error
	}{
		{name: "race loser is opaque and audited", wantErr: domain.ErrInvitationUnavailable},
		{name: "denial audit failure supersedes opaque result", auditErr: auditFailure, wantErr: auditFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			var events []audit.Event
			var commands []AcceptInvitationCommand
			dependencies := invitationDependencies(now)
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
			dependencies.Invitations = controlledInvitationRepository{
				decision: InvitationDecision{
					Invitation: invitation, Path: invitationTestPath(now),
					RecipientVisibility: identity.ProfileVisibilityPrivate,
				},
				acceptCommands: &commands,
				acceptErr:      domain.ErrInvitationUnavailable,
			}
			dependencies.Audits = controlledAudits{events: &events, err: test.auditErr}

			_, err := NewInvitationService(dependencies).Accept(
				context.Background(), "Bearer valid", "accept-key-000001", invitation.ID,
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Accept() error = %v, want %v", err, test.wantErr)
			}
			if test.auditErr != nil && errors.Is(err, domain.ErrInvitationUnavailable) {
				t.Fatalf("Accept() leaked unavailable result when denial audit failed: %v", err)
			}
			if len(commands) != 1 {
				t.Fatalf("Accept() commands = %d, want one attempted atomic acceptance", len(commands))
			}
			if len(events) != 1 {
				t.Fatalf("denial audits = %+v, want exactly one", events)
			}
			event := events[0]
			if event.Action != audit.ResourceAccessDenied || event.Outcome != audit.Denied ||
				event.OwnerUserID != "recipient" || event.ActorUserID != "recipient" ||
				event.TargetType != "path_invitation" || event.TargetID != string(invitation.ID) ||
				event.OccurredAt != now {
				t.Fatalf("denial audit = %+v", event)
			}
		})
	}
}
