package path

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

var errInvalidOwnershipTransferDependencies = errors.New("path ownership transfer service dependencies are invalid")

type ownershipTransferPathReader interface {
	Get(context.Context, string, domain.ID) (domain.Entity, error)
}

type ownershipTransferAuthorizationReconciler interface {
	ReconcileAuthorizationBatch(context.Context, string, string) error
}

type OwnershipTransferDependencies struct {
	Auth                    ports.Authenticator
	Paths                   ownershipTransferPathReader
	Profiles                ProfileReader
	Authorizer              ports.Authorizer
	AuthorizationReconciler ownershipTransferAuthorizationReconciler
	AuthorizationWorker     string
	Transfers               OwnershipTransferRepository
	Audits                  ports.Audits
	AuditRateLimiter        ports.AuditRateLimiter
	Clock                   ports.Clock
	NewID                   func() string
	Lifetime                time.Duration
	CursorSigningKey        []byte
}

type OwnershipTransferService struct{ OwnershipTransferDependencies }

func NewOwnershipTransferService(dependencies OwnershipTransferDependencies) *OwnershipTransferService {
	return &OwnershipTransferService{OwnershipTransferDependencies: dependencies}
}

func (service *OwnershipTransferService) authenticate(ctx context.Context, authorization string) (ports.Principal, error) {
	if service.Auth == nil {
		return ports.Principal{}, errInvalidOwnershipTransferDependencies
	}
	principal, err := service.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return ports.Principal{}, err
	}
	if strings.TrimSpace(principal.UserID) == "" || len(principal.Scopes) != 1 || principal.Scopes[0] != "api:user" {
		return ports.Principal{}, platformapp.ErrUnauthenticated
	}
	return principal, nil
}

func (service *OwnershipTransferService) ListCandidates(ctx context.Context, authorization string, pathID domain.ID, cursor string, limit int) ([]OwnershipTransferCandidate, string, error) {
	principal, now, err := service.authorizedCreator(ctx, authorization, pathID, false)
	if err != nil {
		return nil, "", err
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if len(service.CursorSigningKey) < 32 {
		return nil, "", errInvalidOwnershipTransferDependencies
	}
	request := OwnershipTransferCandidatePageRequest{Limit: limit, Snapshot: now}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != ownershipTransferCandidateCursorDomain || payload.Snapshot.After(now) {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterUserID = payload.AfterID
		request.AfterCreated = payload.AfterCreated
		request.Snapshot = payload.Snapshot
	}
	page, err := service.Transfers.ListCandidates(ctx, OwnershipTransferQuery{ActorUserID: principal.UserID, PathID: pathID}, request)
	if err != nil {
		return nil, "", err
	}
	if len(page.Items) > limit || (page.HasMore && len(page.Items) == 0) {
		return nil, "", errInvalidOwnershipTransferDependencies
	}
	seen := make(map[string]struct{}, len(page.Items))
	for _, candidate := range page.Items {
		if !validOwnershipTransferCandidate(candidate, principal.UserID) || candidate.CreatedAt.IsZero() || candidate.CreatedAt.After(request.Snapshot) {
			return nil, "", errInvalidOwnershipTransferDependencies
		}
		if _, exists := seen[candidate.UserID]; exists {
			return nil, "", errInvalidOwnershipTransferDependencies
		}
		seen[candidate.UserID] = struct{}{}
	}
	nextCursor := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		nextCursor, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{
			Version: 1, Owner: principal.UserID, Domain: ownershipTransferCandidateCursorDomain,
			AfterID: last.UserID, AfterCreated: last.CreatedAt, Snapshot: request.Snapshot,
		})
		if err != nil {
			return nil, "", err
		}
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.PathOwnershipTransferListed, "path", string(pathID), audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	return page.Items, nextCursor, nil
}

func (service *OwnershipTransferService) GetPending(ctx context.Context, authorization string, pathID domain.ID) (OwnershipTransferDecision, error) {
	principal, now, err := service.authorizedActor(ctx, authorization)
	if err != nil {
		return OwnershipTransferDecision{}, err
	}
	if pathID == "" {
		return OwnershipTransferDecision{}, ports.ErrInvalidArgument
	}
	decision, err := service.Transfers.GetPending(ctx, OwnershipTransferQuery{ActorUserID: principal.UserID, PathID: pathID})
	if err != nil {
		return OwnershipTransferDecision{}, opaqueOwnershipTransferError(err)
	}
	if !validPendingOwnershipTransferDecision(decision, principal.UserID, now) {
		return OwnershipTransferDecision{}, domain.ErrOwnershipTransferUnavailable
	}
	decision.ViewerTimeZone, err = service.viewerTimeZone(ctx, principal.UserID)
	if err != nil {
		return OwnershipTransferDecision{}, err
	}
	event := shared.NewAuditEvent(ctx, service.Clock, decision.Transfer.InitiatorUserID, principal.UserID, audit.PathOwnershipTransferListed, "path_ownership_transfer", string(decision.Transfer.ID), audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return OwnershipTransferDecision{}, err
	}
	return decision, nil
}

func (service *OwnershipTransferService) Review(ctx context.Context, authorization string, pathID domain.ID, recipientUserID string) (OwnershipTransferReview, error) {
	principal, now, err := service.authorizedCreator(ctx, authorization, pathID, true)
	if err != nil {
		return OwnershipTransferReview{}, err
	}
	recipientUserID = strings.TrimSpace(recipientUserID)
	if recipientUserID == "" || recipientUserID == principal.UserID || len(service.CursorSigningKey) < 32 {
		return OwnershipTransferReview{}, ports.ErrInvalidArgument
	}
	candidate, err := service.Transfers.Candidate(ctx, OwnershipTransferQuery{ActorUserID: principal.UserID, PathID: pathID}, recipientUserID)
	if err != nil {
		return OwnershipTransferReview{}, opaqueOwnershipTransferError(err)
	}
	if !validOwnershipTransferCandidate(candidate, principal.UserID) || candidate.UserID != recipientUserID {
		return OwnershipTransferReview{}, errInvalidOwnershipTransferDependencies
	}
	expiresAt := now.Add(service.Lifetime)
	token, err := encodeOwnershipTransferReservation(service.CursorSigningKey, ownershipTransferReservation{
		Version: 1, Domain: "path-ownership-transfer-review", CreatorUserID: principal.UserID, PathID: pathID, RecipientUserID: candidate.UserID, ReviewedAt: now, ExpiresAt: expiresAt,
	})
	if err != nil {
		return OwnershipTransferReview{}, err
	}
	viewerTimeZone, err := service.viewerTimeZone(ctx, principal.UserID)
	if err != nil {
		return OwnershipTransferReview{}, err
	}
	return OwnershipTransferReview{
		Recipient:  OwnershipTransferPublicIdentity{UserID: candidate.UserID, Username: candidate.Username, DisplayName: candidate.DisplayName},
		ReviewedAt: now, ExpiresAt: expiresAt, ReservationToken: token, ViewerTimeZone: viewerTimeZone,
	}, nil
}

func (service *OwnershipTransferService) Initiate(
	ctx context.Context,
	authorization string,
	pathID domain.ID,
	reservationToken, idempotencyKey string,
	confirmed bool,
) (OwnershipTransferResult, error) {
	principal, now, err := service.authorizedCreator(ctx, authorization, pathID, true)
	if err != nil {
		return OwnershipTransferResult{}, err
	}
	reservationToken = strings.TrimSpace(reservationToken)
	if !confirmed || reservationToken == "" || !validIdempotencyKey(idempotencyKey) {
		return OwnershipTransferResult{}, ports.ErrInvalidArgument
	}
	reservation, decodeErr := decodeOwnershipTransferReservation(service.CursorSigningKey, reservationToken)
	if decodeErr != nil || reservation.CreatorUserID != principal.UserID || reservation.PathID != pathID || !now.Before(reservation.ExpiresAt) {
		return OwnershipTransferResult{}, ports.ErrInvalidArgument
	}
	viewerTimeZone, err := service.viewerTimeZone(ctx, principal.UserID)
	if err != nil {
		return OwnershipTransferResult{}, err
	}
	recipientUserID := reservation.RecipientUserID
	candidate, err := service.Transfers.Candidate(ctx, OwnershipTransferQuery{ActorUserID: principal.UserID, PathID: pathID}, recipientUserID)
	if err != nil {
		return OwnershipTransferResult{}, opaqueOwnershipTransferError(err)
	}
	if !validOwnershipTransferCandidate(candidate, principal.UserID) || candidate.UserID != recipientUserID {
		return OwnershipTransferResult{}, errInvalidOwnershipTransferDependencies
	}
	transfer, err := domain.NewReviewedOwnershipTransfer(
		domain.OwnershipTransferID(service.NewID()), pathID, principal.UserID, recipientUserID, reservation.ReviewedAt, now, reservation.ExpiresAt,
	)
	if err != nil {
		return OwnershipTransferResult{}, errInvalidOwnershipTransferDependencies
	}
	notification := newOwnershipTransferNotification(service.NewID(), transfer, NotificationPathOwnershipTransferReceived, principal.UserID, recipientUserID, NotificationActionable, true, now)
	if !validOwnershipTransferNotification(notification, transfer) {
		return OwnershipTransferResult{}, errInvalidOwnershipTransferDependencies
	}
	digest := sha256.Sum256([]byte(reservationToken))
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.PathOwnershipTransferCreated, "path_ownership_transfer", string(transfer.ID), audit.Succeeded)
	event.OccurredAt = now
	result, err := service.Transfers.Initiate(ctx, InitiateOwnershipTransferCommand{
		Transfer: transfer, ExpectedCreatorUserID: principal.UserID,
		RequireActivePath: true, RequireRecipientParticipant: true, RequireNoPendingForPath: true,
		Notification: notification,
		Idempotency:  ports.Idempotency{PrincipalID: principal.UserID, Operation: InitiateOwnershipTransferOperation, Key: idempotencyKey, RequestHash: digest[:]},
		Audit:        event,
	})
	if err != nil {
		return OwnershipTransferResult{}, opaqueOwnershipTransferError(err)
	}
	if !validInitiatedOwnershipTransferResult(result, transfer) {
		return OwnershipTransferResult{}, errInvalidOwnershipTransferDependencies
	}
	result.Counterpart = OwnershipTransferPublicIdentity{UserID: candidate.UserID, Username: candidate.Username, DisplayName: candidate.DisplayName}
	result.CounterpartRole = "recipient"
	result.ViewerTimeZone = viewerTimeZone
	return result, nil
}

func (service *OwnershipTransferService) Accept(ctx context.Context, authorization string, transferID domain.OwnershipTransferID, idempotencyKey string) (OwnershipTransferResult, error) {
	return service.mutate(ctx, authorization, transferID, idempotencyKey, AcceptOwnershipTransferOperation)
}

func (service *OwnershipTransferService) Decline(ctx context.Context, authorization string, transferID domain.OwnershipTransferID, idempotencyKey string) (OwnershipTransferResult, error) {
	return service.mutate(ctx, authorization, transferID, idempotencyKey, DeclineOwnershipTransferOperation)
}

func (service *OwnershipTransferService) Cancel(ctx context.Context, authorization string, transferID domain.OwnershipTransferID, idempotencyKey string) (OwnershipTransferResult, error) {
	return service.mutate(ctx, authorization, transferID, idempotencyKey, CancelOwnershipTransferOperation)
}

func (service *OwnershipTransferService) mutate(ctx context.Context, authorization string, transferID domain.OwnershipTransferID, idempotencyKey, operation string) (OwnershipTransferResult, error) {
	principal, now, err := service.authorizedActor(ctx, authorization)
	if err != nil {
		return OwnershipTransferResult{}, err
	}
	if transferID == "" || !validIdempotencyKey(idempotencyKey) {
		return OwnershipTransferResult{}, ports.ErrInvalidArgument
	}
	viewerTimeZone, err := service.viewerTimeZone(ctx, principal.UserID)
	if err != nil {
		return OwnershipTransferResult{}, err
	}
	digest := canonicalOwnershipTransferMutationHash(transferID)
	idempotency := ports.Idempotency{PrincipalID: principal.UserID, Operation: operation, Key: idempotencyKey, RequestHash: digest[:]}
	decision, err := service.Transfers.GetPending(ctx, OwnershipTransferQuery{ActorUserID: principal.UserID, TransferID: transferID, Idempotency: idempotency})
	if err != nil {
		return OwnershipTransferResult{}, opaqueOwnershipTransferError(err)
	}
	if decision.Replay != nil {
		if !validOwnershipTransferReplay(*decision.Replay, transferID, principal.UserID, operation) {
			return OwnershipTransferResult{}, errInvalidOwnershipTransferDependencies
		}
		if operation == AcceptOwnershipTransferOperation {
			if err := service.applyOwnershipTransferRelationships(ctx, *decision.Replay); err != nil {
				return OwnershipTransferResult{}, err
			}
		}
		decision.Replay.ViewerTimeZone = viewerTimeZone
		return *decision.Replay, nil
	}
	if !validPendingOwnershipTransferDecision(decision, principal.UserID, now) || decision.Transfer.ID != transferID {
		return OwnershipTransferResult{}, domain.ErrOwnershipTransferUnavailable
	}
	if service.NewID == nil {
		return OwnershipTransferResult{}, errInvalidOwnershipTransferDependencies
	}

	transfer := decision.Transfer
	owner := transfer.InitiatorUserID
	action := audit.Action("")
	switch operation {
	case AcceptOwnershipTransferOperation:
		action = audit.PathOwnershipTransferAccepted
	case DeclineOwnershipTransferOperation:
		action = audit.PathOwnershipTransferDeclined
	case CancelOwnershipTransferOperation:
		action = audit.PathOwnershipTransferCanceled
	default:
		return OwnershipTransferResult{}, errInvalidOwnershipTransferDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, owner, principal.UserID, action, "path_ownership_transfer", string(transferID), audit.Succeeded)
	event.OccurredAt = now
	var result OwnershipTransferResult
	switch operation {
	case AcceptOwnershipTransferOperation:
		if decision.Path.Archived() || decision.Path.OwnerUserID != owner || !decision.RecipientIsParticipant {
			return OwnershipTransferResult{}, domain.ErrOwnershipTransferUnavailable
		}
		accepted, transitionErr := transfer.Accept(principal.UserID, now)
		if transitionErr != nil {
			return OwnershipTransferResult{}, domain.ErrOwnershipTransferUnavailable
		}
		result, err = service.Transfers.Accept(ctx, AcceptOwnershipTransferCommand{
			Transfer: accepted, ExpectedCreatorUserID: owner, RequireActivePath: true, RequireRecipientParticipant: true,
			PromoteRecipientToCreator: true, RetainFormerCreatorAsAdministrator: true, PreserveMembershipAndActivity: true,
			Notification: newOwnershipTransferNotification(service.NewID(), accepted, NotificationPathOwnershipTransferAccepted, principal.UserID, owner, NotificationInformational, true, now),
			Idempotency:  idempotency, Audit: event,
		})
	case DeclineOwnershipTransferOperation:
		declined, transitionErr := transfer.Decline(principal.UserID, now)
		if transitionErr != nil {
			return OwnershipTransferResult{}, domain.ErrOwnershipTransferUnavailable
		}
		result, err = service.Transfers.Decline(ctx, DeclineOwnershipTransferCommand{
			Transfer:     declined,
			Notification: newOwnershipTransferNotification(service.NewID(), declined, NotificationPathOwnershipTransferDeclined, principal.UserID, owner, NotificationInformational, false, now),
			Idempotency:  idempotency, Audit: event,
		})
	case CancelOwnershipTransferOperation:
		if decision.Path.OwnerUserID != transfer.InitiatorUserID {
			return OwnershipTransferResult{}, domain.ErrOwnershipTransferUnavailable
		}
		canceled, transitionErr := transfer.Cancel(principal.UserID, now)
		if transitionErr != nil {
			return OwnershipTransferResult{}, domain.ErrOwnershipTransferUnavailable
		}
		result, err = service.Transfers.Cancel(ctx, CancelOwnershipTransferCommand{
			Transfer:     canceled,
			Notification: newOwnershipTransferNotification(service.NewID(), canceled, NotificationPathOwnershipTransferCanceled, principal.UserID, canceled.RecipientUserID, NotificationInformational, false, now),
			Idempotency:  idempotency, Audit: event,
		})
	}
	if err != nil {
		return OwnershipTransferResult{}, opaqueOwnershipTransferError(err)
	}
	if !validCompletedOwnershipTransferResult(result, transferID, principal.UserID, operation) {
		return OwnershipTransferResult{}, errInvalidOwnershipTransferDependencies
	}
	if operation == AcceptOwnershipTransferOperation {
		if err := service.applyOwnershipTransferRelationships(ctx, result); err != nil {
			return OwnershipTransferResult{}, err
		}
	}
	result.Counterpart = decision.Counterpart
	result.CounterpartRole = decision.CounterpartRole
	result.ViewerTimeZone = viewerTimeZone
	return result, nil
}

func (service *OwnershipTransferService) viewerTimeZone(ctx context.Context, userID string) (string, error) {
	if service.Profiles == nil {
		return "", errInvalidOwnershipTransferDependencies
	}
	zone, err := service.Profiles.TimeZone(ctx, userID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(zone) != zone || zone == "" || zone == "Local" {
		return "", errInvalidOwnershipTransferDependencies
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return "", errInvalidOwnershipTransferDependencies
	}
	return zone, nil
}

func (service *OwnershipTransferService) applyOwnershipTransferRelationships(ctx context.Context, result OwnershipTransferResult) error {
	if service.AuthorizationReconciler == nil || strings.TrimSpace(service.AuthorizationWorker) == "" ||
		!validOwnershipTransferRelationshipUpdates(result.RelationshipUpdates, result.Transfer) ||
		!validOwnershipTransferAuthorizationBatch(result.AuthorizationBatch, result) {
		return errInvalidOwnershipTransferDependencies
	}
	return service.AuthorizationReconciler.ReconcileAuthorizationBatch(ctx, result.AuthorizationBatch.ID, service.AuthorizationWorker)
}

func validOwnershipTransferAuthorizationBatch(batch ports.AuthorizationBatch, result OwnershipTransferResult) bool {
	if batch.ID == "" || batch.TransferID != string(result.Transfer.ID) || batch.ResourceType != "path" ||
		batch.ResourceID != string(result.Transfer.PathID) || batch.OwnerUserID != result.Transfer.RecipientUserID ||
		batch.ActorUserID != result.Transfer.RecipientUserID || len(batch.Updates) != len(result.RelationshipUpdates) {
		return false
	}
	for index := range result.RelationshipUpdates {
		if batch.Updates[index] != result.RelationshipUpdates[index] {
			return false
		}
	}
	return true
}

func validOwnershipTransferRelationshipUpdates(actual []ports.RelationshipUpdate, transfer domain.OwnershipTransfer) bool {
	expected := expectedOwnershipTransferRelationshipUpdates(transfer)
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func expectedOwnershipTransferRelationshipUpdates(transfer domain.OwnershipTransfer) []ports.RelationshipUpdate {
	resourceID := string(transfer.PathID)
	return []ports.RelationshipUpdate{
		{Operation: ports.AuthorizationDelete, ResourceType: "path", ResourceID: resourceID, Relation: "creator", SubjectType: "user", SubjectID: transfer.InitiatorUserID},
		{Operation: ports.AuthorizationTouch, ResourceType: "path", ResourceID: resourceID, Relation: "creator", SubjectType: "user", SubjectID: transfer.RecipientUserID},
		{Operation: ports.AuthorizationTouch, ResourceType: "path", ResourceID: resourceID, Relation: "administrator", SubjectType: "user", SubjectID: transfer.InitiatorUserID},
		{Operation: ports.AuthorizationDelete, ResourceType: "path", ResourceID: resourceID, Relation: "administrator", SubjectType: "user", SubjectID: transfer.RecipientUserID},
	}
}

func (service *OwnershipTransferService) authorizedActor(ctx context.Context, authorization string) (ports.Principal, time.Time, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return ports.Principal{}, time.Time{}, err
	}
	if service.Transfers == nil || service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil {
		return ports.Principal{}, time.Time{}, errInvalidOwnershipTransferDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return ports.Principal{}, time.Time{}, errInvalidOwnershipTransferDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return ports.Principal{}, time.Time{}, platformapp.ErrRateLimited
	}
	return principal, now, nil
}

func (service *OwnershipTransferService) authorizedCreator(ctx context.Context, authorization string, pathID domain.ID, requireCreateDependencies bool) (ports.Principal, time.Time, error) {
	principal, now, err := service.authorizedActor(ctx, authorization)
	if err != nil {
		return ports.Principal{}, time.Time{}, err
	}
	if pathID == "" {
		return ports.Principal{}, time.Time{}, ports.ErrInvalidArgument
	}
	if service.Paths == nil || service.Authorizer == nil || (requireCreateDependencies && (service.NewID == nil || service.Lifetime <= 0)) {
		return ports.Principal{}, time.Time{}, errInvalidOwnershipTransferDependencies
	}
	allowed, err := service.Authorizer.Check(ctx, "path", string(pathID), "transfer_ownership", principal.UserID)
	if err != nil {
		return ports.Principal{}, time.Time{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(pathID), audit.Denied)
		event.OccurredAt = now
		if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
			return ports.Principal{}, time.Time{}, err
		}
		return ports.Principal{}, time.Time{}, platformapp.ErrForbidden
	}
	entity, err := service.Paths.Get(ctx, principal.UserID, pathID)
	if err != nil {
		return ports.Principal{}, time.Time{}, err
	}
	if entity.ID != pathID || entity.OwnerUserID != principal.UserID || !validCreatedPath(entity, entity.OwnerUserID) {
		return ports.Principal{}, time.Time{}, domain.ErrOwnershipTransferUnavailable
	}
	if entity.Archived() {
		return ports.Principal{}, time.Time{}, domain.ErrOwnershipTransferUnavailable
	}
	return principal, now, nil
}

func validOwnershipTransferCandidate(candidate OwnershipTransferCandidate, creator string) bool {
	return strings.TrimSpace(candidate.UserID) == candidate.UserID && candidate.UserID != "" && candidate.UserID != creator &&
		strings.TrimSpace(candidate.Username) == candidate.Username && candidate.Username != "" &&
		strings.TrimSpace(candidate.DisplayName) == candidate.DisplayName && candidate.DisplayName != ""
}

func validPendingOwnershipTransferDecision(decision OwnershipTransferDecision, actor string, now time.Time) bool {
	transfer := decision.Transfer
	return transfer.ID != "" && transfer.PathID != "" && transfer.Pending(now) &&
		decision.Path.ID == transfer.PathID && decision.Path.OwnerUserID != "" &&
		(actor == transfer.InitiatorUserID || actor == transfer.RecipientUserID) &&
		validOwnershipTransferCounterpart(decision.Counterpart, decision.CounterpartRole, transfer, actor)
}

func validOwnershipTransferCounterpart(identity OwnershipTransferPublicIdentity, role string, transfer domain.OwnershipTransfer, actor string) bool {
	if strings.TrimSpace(identity.UserID) != identity.UserID || identity.UserID == "" ||
		strings.TrimSpace(identity.Username) != identity.Username || identity.Username == "" ||
		strings.TrimSpace(identity.DisplayName) != identity.DisplayName || identity.DisplayName == "" {
		return false
	}
	return (actor == transfer.InitiatorUserID && role == "recipient" && identity.UserID == transfer.RecipientUserID) ||
		(actor == transfer.RecipientUserID && role == "creator" && identity.UserID == transfer.InitiatorUserID)
}

func validInitiatedOwnershipTransferResult(result OwnershipTransferResult, expected domain.OwnershipTransfer) bool {
	if !result.Replayed {
		return result.Transfer == expected && result.Path.ID == ""
	}
	return result.Transfer.ID != "" && result.Transfer.PathID == expected.PathID &&
		result.Transfer.InitiatorUserID == expected.InitiatorUserID &&
		result.Transfer.RecipientUserID == expected.RecipientUserID
}

func validCompletedOwnershipTransferResult(result OwnershipTransferResult, id domain.OwnershipTransferID, actor, operation string) bool {
	if result.Transfer.ID != id || result.Transfer.PathID == "" || result.Transfer.InitiatorUserID == "" || result.Transfer.RecipientUserID == "" {
		return false
	}
	switch operation {
	case AcceptOwnershipTransferOperation:
		return actor == result.Transfer.RecipientUserID && !result.Transfer.AcceptedAt.IsZero() && result.Path.ID == result.Transfer.PathID && result.Path.OwnerUserID == actor
	case DeclineOwnershipTransferOperation:
		return actor == result.Transfer.RecipientUserID && !result.Transfer.DeclinedAt.IsZero()
	case CancelOwnershipTransferOperation:
		return actor == result.Transfer.InitiatorUserID && !result.Transfer.CanceledAt.IsZero()
	}
	return false
}

func validOwnershipTransferReplay(result OwnershipTransferResult, id domain.OwnershipTransferID, actor, operation string) bool {
	return result.Replayed && validCompletedOwnershipTransferResult(result, id, actor, operation) &&
		validOwnershipTransferCounterpart(result.Counterpart, result.CounterpartRole, result.Transfer, actor)
}

func opaqueOwnershipTransferError(err error) error {
	if errors.Is(err, ports.ErrNotFound) || errors.Is(err, domain.ErrOwnershipTransferUnavailable) || errors.Is(err, ports.ErrConflict) {
		return domain.ErrOwnershipTransferUnavailable
	}
	return err
}

func canonicalOwnershipTransferMutationHash(transferID domain.OwnershipTransferID) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(transferID))
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}
