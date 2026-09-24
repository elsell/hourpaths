package path

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type transferRepositoryFake struct {
	candidates []OwnershipTransferCandidate
	page       OwnershipTransferCandidatePage
	pending    OwnershipTransferDecision
	result     OwnershipTransferResult
	err        error
	queries    *[]OwnershipTransferQuery
	pages      *[]OwnershipTransferCandidatePageRequest
	initiates  *[]InitiateOwnershipTransferCommand
	accepts    *[]AcceptOwnershipTransferCommand
	declines   *[]DeclineOwnershipTransferCommand
	cancels    *[]CancelOwnershipTransferCommand
}

type ownershipTransferReconcilerFake struct {
	calls *[]string
	err   error
}

func (fake ownershipTransferReconcilerFake) ReconcileAuthorizationBatch(_ context.Context, id, worker string) error {
	if fake.calls != nil {
		*fake.calls = append(*fake.calls, id+"|"+worker)
	}
	return fake.err
}

func (f transferRepositoryFake) ListCandidates(_ context.Context, query OwnershipTransferQuery, page OwnershipTransferCandidatePageRequest) (OwnershipTransferCandidatePage, error) {
	if f.queries != nil {
		*f.queries = append(*f.queries, query)
	}
	if f.pages != nil {
		*f.pages = append(*f.pages, page)
	}
	if f.page.Items != nil || f.page.HasMore {
		return f.page, f.err
	}
	return OwnershipTransferCandidatePage{Items: f.candidates}, f.err
}
func (f transferRepositoryFake) Candidate(_ context.Context, query OwnershipTransferQuery, userID string) (OwnershipTransferCandidate, error) {
	if f.queries != nil {
		*f.queries = append(*f.queries, query)
	}
	for _, candidate := range f.candidates {
		if candidate.UserID == userID {
			return candidate, f.err
		}
	}
	if f.err != nil {
		return OwnershipTransferCandidate{}, f.err
	}
	return OwnershipTransferCandidate{}, domain.ErrOwnershipTransferUnavailable
}
func (f transferRepositoryFake) GetPending(_ context.Context, query OwnershipTransferQuery) (OwnershipTransferDecision, error) {
	if f.queries != nil {
		*f.queries = append(*f.queries, query)
	}
	decision := f.pending
	if decision.Counterpart.UserID == "" && decision.Transfer.ID != "" {
		if query.ActorUserID == decision.Transfer.InitiatorUserID {
			decision.Counterpart = OwnershipTransferPublicIdentity{UserID: decision.Transfer.RecipientUserID, Username: "recipient", DisplayName: "Recipient"}
			decision.CounterpartRole = "recipient"
		} else if query.ActorUserID == decision.Transfer.RecipientUserID {
			decision.Counterpart = OwnershipTransferPublicIdentity{UserID: decision.Transfer.InitiatorUserID, Username: "creator", DisplayName: "Creator"}
			decision.CounterpartRole = "creator"
		}
	}
	if decision.Replay != nil && decision.Replay.Counterpart.UserID == "" {
		decision.Replay.Counterpart, decision.Replay.CounterpartRole = decision.Counterpart, decision.CounterpartRole
	}
	return decision, f.err
}
func (f transferRepositoryFake) Initiate(_ context.Context, command InitiateOwnershipTransferCommand) (OwnershipTransferResult, error) {
	if f.initiates != nil {
		*f.initiates = append(*f.initiates, command)
	}
	if f.result.Transfer.ID == "" {
		return OwnershipTransferResult{Transfer: command.Transfer}, f.err
	}
	return f.result, f.err
}
func (f transferRepositoryFake) Accept(_ context.Context, command AcceptOwnershipTransferCommand) (OwnershipTransferResult, error) {
	if f.accepts != nil {
		*f.accepts = append(*f.accepts, command)
	}
	if f.result.Transfer.ID == "" {
		path := activeTransferPath(command.Transfer.AcceptedAt)
		path.OwnerUserID = command.Transfer.RecipientUserID
		updates := expectedOwnershipTransferRelationshipUpdates(command.Transfer)
		return OwnershipTransferResult{Transfer: command.Transfer, Path: path, RelationshipUpdates: updates, AuthorizationBatch: ownershipTransferAuthorizationBatch(command.Transfer, updates)}, f.err
	}
	return f.result, f.err
}
func (f transferRepositoryFake) Decline(_ context.Context, command DeclineOwnershipTransferCommand) (OwnershipTransferResult, error) {
	if f.declines != nil {
		*f.declines = append(*f.declines, command)
	}
	if f.result.Transfer.ID == "" {
		return OwnershipTransferResult{Transfer: command.Transfer}, f.err
	}
	return f.result, f.err
}
func (f transferRepositoryFake) Cancel(_ context.Context, command CancelOwnershipTransferCommand) (OwnershipTransferResult, error) {
	if f.cancels != nil {
		*f.cancels = append(*f.cancels, command)
	}
	if f.result.Transfer.ID == "" {
		return OwnershipTransferResult{Transfer: command.Transfer}, f.err
	}
	return f.result, f.err
}

func activeTransferPath(now time.Time) domain.Entity {
	return domain.Entity{ID: "path-1", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Practice", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
}

func transferService(now time.Time, repository OwnershipTransferRepository) *OwnershipTransferService {
	return NewOwnershipTransferService(OwnershipTransferDependencies{
		Auth:       controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}},
		Paths:      controlledRepository{entity: activeTransferPath(now)},
		Profiles:   controlledProfiles{timeZone: "America/New_York"},
		Authorizer: controlledAuthorizer{allowed: true}, AuthorizationReconciler: ownershipTransferReconcilerFake{}, AuthorizationWorker: "transfer-worker", Transfers: repository,
		Audits: controlledAudits{}, AuditRateLimiter: controlledLimiter{}, Clock: controlledClock{now: now},
		NewID: sequentialIDs("transfer-1", "transfer-notification-1"), Lifetime: 72 * time.Hour,
		CursorSigningKey: []byte("0123456789abcdef0123456789abcdef"),
	})
}

func transferReservationToken(t *testing.T, service *OwnershipTransferService, now time.Time, recipient string) string {
	t.Helper()
	token, err := encodeOwnershipTransferReservation(service.CursorSigningKey, ownershipTransferReservation{
		Version: 1, Domain: "path-ownership-transfer-review", CreatorUserID: "creator", PathID: "path-1", RecipientUserID: recipient, ReviewedAt: now, ExpiresAt: now.Add(72 * time.Hour),
	})
	if err != nil {
		t.Fatalf("encode reservation: %v", err)
	}
	return token
}

func TestInitiateOwnershipTransferUsesCreatorPermissionEligibleCandidateAbsoluteExpiryAndAtomicCommand(t *testing.T) {
	now := time.Date(2026, 7, 27, 16, 30, 0, 123456000, time.UTC)
	var authz []authorizationCall
	var commands []InitiateOwnershipTransferCommand
	repository := transferRepositoryFake{
		candidates: []OwnershipTransferCandidate{{UserID: "recipient", Username: "recipient", DisplayName: "Recipient", Administrator: true, CreatedAt: now.Add(-time.Hour)}},
		initiates:  &commands,
	}
	service := transferService(now, repository)
	service.Authorizer = controlledAuthorizer{allowed: true, calls: &authz}

	result, err := service.Initiate(context.Background(), "Bearer valid", "path-1", transferReservationToken(t, service, now, "recipient"), "transfer-key-0001", true)
	if err != nil {
		t.Fatalf("Initiate() error = %v", err)
	}
	if len(authz) != 1 || authz[0] != (authorizationCall{"path", "path-1", "transfer_ownership", "creator"}) {
		t.Fatalf("authorization calls = %+v", authz)
	}
	if len(commands) != 1 {
		t.Fatalf("initiate commands = %d, want 1", len(commands))
	}
	command := commands[0]
	if command.Transfer.ID != "transfer-1" || command.Transfer.PathID != "path-1" ||
		command.Transfer.InitiatorUserID != "creator" || command.Transfer.RecipientUserID != "recipient" ||
		command.Transfer.ReviewedAt != now || command.Transfer.CreatedAt != now || command.Transfer.ExpiresAt != now.Add(72*time.Hour) {
		t.Fatalf("transfer = %+v", command.Transfer)
	}
	if command.ExpectedCreatorUserID != "creator" || !command.RequireActivePath ||
		!command.RequireRecipientParticipant || !command.RequireNoPendingForPath {
		t.Fatalf("atomic initiation invariants = %+v", command)
	}
	if command.Idempotency.PrincipalID != "creator" || command.Idempotency.Operation != InitiateOwnershipTransferOperation ||
		command.Idempotency.Key != "transfer-key-0001" || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("idempotency = %+v", command.Idempotency)
	}
	if command.Audit.Action != audit.PathOwnershipTransferCreated || command.Audit.Outcome != audit.Succeeded ||
		command.Audit.OwnerUserID != "creator" || command.Audit.ActorUserID != "creator" ||
		command.Audit.TargetType != "path_ownership_transfer" || command.Audit.TargetID != "transfer-1" || command.Audit.OccurredAt != now {
		t.Fatalf("audit = %+v", command.Audit)
	}
	assertTransferNotification(t, command.Notification, "transfer-notification-1", "recipient", "creator",
		NotificationPathOwnershipTransferReceived, NotificationActionable, true, command.Transfer, now)
	if result.Transfer != command.Transfer {
		t.Fatalf("result = %+v, want command transfer", result)
	}
}

func TestOwnershipTransferReviewIsAuthoritativeAndSideEffectFree(t *testing.T) {
	now := time.Date(2026, 7, 27, 16, 30, 0, 123456000, time.UTC)
	var initiates []InitiateOwnershipTransferCommand
	service := transferService(now, transferRepositoryFake{
		candidates: []OwnershipTransferCandidate{{UserID: "recipient", Username: "canonical", DisplayName: "Canonical Recipient", CreatedAt: now.Add(-time.Hour)}},
		initiates:  &initiates,
	})
	review, err := service.Review(context.Background(), "Bearer valid", "path-1", "recipient")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if review.Recipient.UserID != "recipient" || review.Recipient.Username != "canonical" || review.Recipient.DisplayName != "Canonical Recipient" ||
		review.ReviewedAt != now || review.ExpiresAt != now.Add(72*time.Hour) || review.ViewerTimeZone != "America/New_York" || review.ReservationToken == "" {
		t.Fatalf("review = %+v", review)
	}
	if len(initiates) != 0 {
		t.Fatalf("review persisted %d transfers", len(initiates))
	}
	reservation, err := decodeOwnershipTransferReservation(service.CursorSigningKey, review.ReservationToken)
	if err != nil || reservation.Version != 1 || reservation.Domain != "path-ownership-transfer-review" || reservation.CreatorUserID != "creator" || reservation.PathID != "path-1" || reservation.RecipientUserID != "recipient" || reservation.ReviewedAt != review.ReviewedAt || reservation.ExpiresAt != review.ExpiresAt {
		t.Fatalf("reservation = %+v, err = %v", reservation, err)
	}
}

func TestOwnershipTransferConfirmationRejectsMalformedForgedExpiredAndCrossDomainReservations(t *testing.T) {
	now := time.Date(2026, 7, 27, 16, 30, 0, 0, time.UTC)
	service := transferService(now, transferRepositoryFake{})
	valid := ownershipTransferReservation{Version: 1, Domain: "path-ownership-transfer-review", CreatorUserID: "creator", PathID: "path-1", RecipientUserID: "recipient", ReviewedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)}
	expired, err := encodeOwnershipTransferReservation(service.CursorSigningKey, valid)
	if err != nil {
		t.Fatal(err)
	}
	crossDomain := valid
	crossDomain.Domain = "pagination-cursor"
	forged := "X" + expired[1:]
	for name, token := range map[string]string{"malformed": "not-a-token", "forged": forged, "expired": expired} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.Initiate(context.Background(), "Bearer valid", "path-1", token, "transfer-key-0001", true); !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("Initiate() error = %v", err)
			}
		})
	}
	if _, err := encodeOwnershipTransferReservation(service.CursorSigningKey, crossDomain); !errors.Is(err, errInvalidOwnershipTransferDependencies) {
		t.Fatalf("cross-domain encode error = %v", err)
	}
}

func TestOwnershipTransferCandidateListingAndPendingReadAreActorScopedAndOpaque(t *testing.T) {
	now := time.Date(2026, 7, 27, 17, 0, 0, 0, time.UTC)
	transfer, _ := domain.NewOwnershipTransfer("transfer-1", "path-1", "creator", "recipient", now.Add(-time.Hour), now.Add(time.Hour))
	var queries []OwnershipTransferQuery
	repository := transferRepositoryFake{
		candidates: []OwnershipTransferCandidate{{UserID: "recipient", Username: "recipient", DisplayName: "Recipient", CreatedAt: now.Add(-time.Hour)}},
		pending:    OwnershipTransferDecision{Transfer: transfer, Path: activeTransferPath(now), RecipientIsParticipant: true}, queries: &queries,
	}
	service := transferService(now, repository)

	candidates, _, err := service.ListCandidates(context.Background(), "Bearer valid", "path-1", "", 25)
	if err != nil || len(candidates) != 1 || candidates[0].UserID != "recipient" {
		t.Fatalf("ListCandidates() = %+v, %v", candidates, err)
	}
	pending, err := service.GetPending(context.Background(), "Bearer valid", "path-1")
	if err != nil || pending.Transfer != transfer {
		t.Fatalf("GetPending() = %+v, %v", pending, err)
	}
	if len(queries) != 2 || queries[0].ActorUserID != "creator" || queries[0].PathID != "path-1" ||
		queries[1].ActorUserID != "creator" || queries[1].PathID != "path-1" {
		t.Fatalf("queries = %+v", queries)
	}

	service.Transfers = transferRepositoryFake{err: ports.ErrNotFound}
	if _, err := service.GetPending(context.Background(), "Bearer valid", "path-1"); !errors.Is(err, domain.ErrOwnershipTransferUnavailable) {
		t.Fatalf("GetPending() error = %v, want opaque unavailable", err)
	}
}

func TestOwnershipTransferCandidateListingUsesSignedActorScopedKeysetCursor(t *testing.T) {
	now := time.Date(2026, 7, 27, 17, 30, 0, 0, time.UTC)
	createdAt := now.Add(-time.Hour)
	var pages []OwnershipTransferCandidatePageRequest
	service := transferService(now, transferRepositoryFake{
		page:  OwnershipTransferCandidatePage{Items: []OwnershipTransferCandidate{{UserID: "recipient", Username: "recipient", DisplayName: "Recipient", CreatedAt: createdAt}}, HasMore: true},
		pages: &pages,
	})

	items, nextCursor, err := service.ListCandidates(context.Background(), "Bearer valid", "path-1", "", 1)
	if err != nil || len(items) != 1 || nextCursor == "" || len(pages) != 1 || pages[0].Limit != 1 || pages[0].Snapshot != now {
		t.Fatalf("first page items=%+v cursor=%q pages=%+v err=%v", items, nextCursor, pages, err)
	}
	payload, err := shared.DecodeCursor(service.CursorSigningKey, nextCursor)
	if err != nil || payload.Owner != "creator" || payload.Domain != ownershipTransferCandidateCursorDomain || payload.AfterID != "recipient" || payload.AfterCreated != createdAt || payload.Snapshot != now {
		t.Fatalf("cursor payload=%+v err=%v", payload, err)
	}
	service.Transfers = transferRepositoryFake{pages: &pages}
	if _, _, err := service.ListCandidates(context.Background(), "Bearer valid", "path-1", nextCursor, 100); err != nil {
		t.Fatalf("continuation error = %v", err)
	}
	if len(pages) != 2 || pages[1].AfterUserID != "recipient" || pages[1].AfterCreated != createdAt || pages[1].Snapshot != now || pages[1].Limit != 100 {
		t.Fatalf("continuation page = %+v", pages)
	}
	if _, _, err := service.ListCandidates(context.Background(), "Bearer valid", "path-1", nextCursor+"tampered", 25); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("tampered cursor error = %v", err)
	}
}

func TestOwnershipTransferMutationsEnforceRecipientAndCreatorRolesAndCarryIdempotentAuditCommands(t *testing.T) {
	now := time.Date(2026, 7, 27, 18, 0, 0, 0, time.UTC)
	base, _ := domain.NewOwnershipTransfer("transfer-1", "path-1", "creator", "recipient", now.Add(-time.Hour), now.Add(time.Hour))
	path := activeTransferPath(now)

	t.Run("accept recipient revalidates active path creator and participant", func(t *testing.T) {
		var commands []AcceptOwnershipTransferCommand
		var reconciliations []string
		repo := transferRepositoryFake{pending: OwnershipTransferDecision{Transfer: base, Path: path, RecipientIsParticipant: true}, accepts: &commands}
		service := transferService(now, repo)
		service.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
		service.AuthorizationReconciler = ownershipTransferReconcilerFake{calls: &reconciliations}
		result, err := service.Accept(context.Background(), "Bearer valid", "transfer-1", "accept-transfer-01")
		if err != nil {
			t.Fatalf("Accept() error = %v", err)
		}
		if len(commands) != 1 || commands[0].Transfer.AcceptedAt != now || commands[0].ExpectedCreatorUserID != "creator" ||
			!commands[0].RequireActivePath || !commands[0].RequireRecipientParticipant || !commands[0].PromoteRecipientToCreator ||
			!commands[0].RetainFormerCreatorAsAdministrator || !commands[0].PreserveMembershipAndActivity {
			t.Fatalf("accept commands = %+v", commands)
		}
		assertTransferMutationMetadata(t, commands[0].Idempotency, commands[0].Audit, "recipient", AcceptOwnershipTransferOperation, "accept-transfer-01", audit.PathOwnershipTransferAccepted, now)
		assertTransferNotification(t, commands[0].Notification, "transfer-1", "creator", "recipient",
			NotificationPathOwnershipTransferAccepted, NotificationInformational, true, commands[0].Transfer, now)
		if result.Transfer.AcceptedAt != now {
			t.Fatalf("accepted result = %+v", result)
		}
		if len(reconciliations) != 1 || reconciliations[0] != "ownership-transfer-transfer-1|transfer-worker" {
			t.Fatalf("authorization reconciliations = %+v", reconciliations)
		}
	})

	t.Run("decline recipient only", func(t *testing.T) {
		var commands []DeclineOwnershipTransferCommand
		repo := transferRepositoryFake{pending: OwnershipTransferDecision{Transfer: base, Path: path, RecipientIsParticipant: true}, declines: &commands}
		service := transferService(now, repo)
		service.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
		_, err := service.Decline(context.Background(), "Bearer valid", "transfer-1", "decline-transfer1")
		if err != nil || len(commands) != 1 || commands[0].Transfer.DeclinedAt != now {
			t.Fatalf("Decline() commands=%+v err=%v", commands, err)
		}
		assertTransferMutationMetadata(t, commands[0].Idempotency, commands[0].Audit, "recipient", DeclineOwnershipTransferOperation, "decline-transfer1", audit.PathOwnershipTransferDeclined, now)
		assertTransferNotification(t, commands[0].Notification, "transfer-1", "creator", "recipient",
			NotificationPathOwnershipTransferDeclined, NotificationInformational, false, commands[0].Transfer, now)
	})

	t.Run("cancel initiating creator only", func(t *testing.T) {
		var commands []CancelOwnershipTransferCommand
		repo := transferRepositoryFake{pending: OwnershipTransferDecision{Transfer: base, Path: path, RecipientIsParticipant: true}, cancels: &commands}
		service := transferService(now, repo)
		_, err := service.Cancel(context.Background(), "Bearer valid", "transfer-1", "cancel-transfer-01")
		if err != nil || len(commands) != 1 || commands[0].Transfer.CanceledAt != now {
			t.Fatalf("Cancel() commands=%+v err=%v", commands, err)
		}
		assertTransferMutationMetadata(t, commands[0].Idempotency, commands[0].Audit, "creator", CancelOwnershipTransferOperation, "cancel-transfer-01", audit.PathOwnershipTransferCanceled, now)
		assertTransferNotification(t, commands[0].Notification, "transfer-1", "recipient", "creator",
			NotificationPathOwnershipTransferCanceled, NotificationInformational, false, commands[0].Transfer, now)
	})
}

func ownershipTransferAuthorizationBatch(transfer domain.OwnershipTransfer, updates []ports.RelationshipUpdate) ports.AuthorizationBatch {
	return ports.AuthorizationBatch{
		ID: "ownership-transfer-" + string(transfer.ID), TransferID: string(transfer.ID), ResourceType: "path", ResourceID: string(transfer.PathID),
		OwnerUserID: transfer.RecipientUserID, ActorUserID: transfer.RecipientUserID, Updates: append([]ports.RelationshipUpdate(nil), updates...),
	}
}

func assertTransferNotification(t *testing.T, notification OwnershipTransferNotification, id, recipient, actor string,
	kind InvitationNotificationKind, presentation NotificationPresentation, push bool,
	transfer domain.OwnershipTransfer, now time.Time,
) {
	t.Helper()
	if notification.ID != id || notification.RecipientUserID != recipient || notification.ActorUserID != actor ||
		notification.PathID != transfer.PathID || notification.OwnershipTransferID != transfer.ID ||
		notification.Kind != kind || notification.Presentation != presentation ||
		notification.PushRequested != push || notification.CreatedAt != now {
		t.Fatalf("notification = %+v", notification)
	}
}

func assertRelationshipUpdateBatch(t *testing.T, actual, expected []ports.RelationshipUpdate) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("relationship update count = %d, want %d: %+v", len(actual), len(expected), actual)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("relationship update %d = %+v, want %+v", index, actual[index], expected[index])
		}
	}
}

func TestOwnershipTransferFailsClosedForIneligibleStaleArchivedOrWrongActorState(t *testing.T) {
	now := time.Date(2026, 7, 27, 19, 0, 0, 0, time.UTC)
	base, _ := domain.NewOwnershipTransfer("transfer-1", "path-1", "creator", "recipient", now.Add(-time.Hour), now.Add(time.Hour))
	path := activeTransferPath(now)

	for _, test := range []struct {
		name     string
		decision OwnershipTransferDecision
		actor    string
	}{
		{name: "recipient no longer participant", decision: OwnershipTransferDecision{Transfer: base, Path: path}, actor: "recipient"},
		{name: "creator changed", decision: OwnershipTransferDecision{Transfer: base, Path: func() domain.Entity { p := path; p.OwnerUserID = "other"; return p }(), RecipientIsParticipant: true}, actor: "recipient"},
		{name: "archived", decision: OwnershipTransferDecision{Transfer: base, Path: func() domain.Entity { p := path; p.ArchivedAt = now.Add(-time.Minute); return p }(), RecipientIsParticipant: true}, actor: "recipient"},
		{name: "wrong accept actor", decision: OwnershipTransferDecision{Transfer: base, Path: path, RecipientIsParticipant: true}, actor: "attacker"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var commands []AcceptOwnershipTransferCommand
			service := transferService(now, transferRepositoryFake{pending: test.decision, accepts: &commands})
			service.Auth = controlledAuthenticator{principal: ports.Principal{UserID: test.actor, Scopes: []string{"api:user"}}}
			_, err := service.Accept(context.Background(), "Bearer valid", "transfer-1", "accept-transfer-01")
			if !errors.Is(err, domain.ErrOwnershipTransferUnavailable) || len(commands) != 0 {
				t.Fatalf("Accept() commands=%d err=%v", len(commands), err)
			}
		})
	}

	var initiates []InitiateOwnershipTransferCommand
	service := transferService(now, transferRepositoryFake{initiates: &initiates})
	if _, err := service.Initiate(context.Background(), "Bearer valid", "path-1", transferReservationToken(t, service, now, "supporter-or-self"), "transfer-key-0001", true); !errors.Is(err, domain.ErrOwnershipTransferUnavailable) || len(initiates) != 0 {
		t.Fatalf("ineligible Initiate() commands=%d err=%v", len(initiates), err)
	}

	service.Authorizer = controlledAuthorizer{allowed: false}
	if _, _, err := service.ListCandidates(context.Background(), "Bearer valid", "path-1", "", 25); !errors.Is(err, platformapp.ErrForbidden) {
		t.Fatalf("denied ListCandidates() error = %v", err)
	}

	archived := path
	archived.ArchivedAt = now.Add(-time.Minute)
	service = transferService(now, transferRepositoryFake{candidates: []OwnershipTransferCandidate{{UserID: "recipient", Username: "recipient", DisplayName: "Recipient"}}, initiates: &initiates})
	service.Paths = controlledRepository{entity: archived}
	if _, err := service.Initiate(context.Background(), "Bearer valid", "path-1", transferReservationToken(t, service, now, "recipient"), "transfer-key-0001", true); !errors.Is(err, domain.ErrOwnershipTransferUnavailable) {
		t.Fatalf("archived Initiate() error = %v", err)
	}

	stalePath := path
	stalePath.OwnerUserID = "new-creator"
	var cancels []CancelOwnershipTransferCommand
	service = transferService(now, transferRepositoryFake{pending: OwnershipTransferDecision{Transfer: base, Path: stalePath, RecipientIsParticipant: true}, cancels: &cancels})
	if _, err := service.Cancel(context.Background(), "Bearer valid", "transfer-1", "cancel-transfer-01"); !errors.Is(err, domain.ErrOwnershipTransferUnavailable) || len(cancels) != 0 {
		t.Fatalf("stale creator Cancel() commands=%d err=%v", len(cancels), err)
	}
}

func assertTransferMutationMetadata(t *testing.T, idempotency ports.Idempotency, event audit.Event, actor, operation, key string, action audit.Action, now time.Time) {
	t.Helper()
	if idempotency.PrincipalID != actor || idempotency.Operation != operation || idempotency.Key != key || len(idempotency.RequestHash) != 32 {
		t.Fatalf("idempotency = %+v", idempotency)
	}
	if event.Action != action || event.Outcome != audit.Succeeded || event.OwnerUserID != "creator" || event.ActorUserID != actor ||
		event.TargetType != "path_ownership_transfer" || event.TargetID != "transfer-1" || event.OccurredAt != now {
		t.Fatalf("audit = %+v", event)
	}
}
