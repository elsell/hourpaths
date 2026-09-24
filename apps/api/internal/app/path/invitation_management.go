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

const managedInvitationCursorDomain = "path-managed-invitation"

func (service *InvitationService) ListManagedPending(ctx context.Context, authorization string, pathID domain.ID, cursor string, limit int) ([]ManagedInvitation, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if pathID == "" {
		return nil, "", ports.ErrInvalidArgument
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if service.Paths == nil || service.Authorizer == nil || service.InvitationManagement == nil || service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil || len(service.CursorSigningKey) < 32 {
		return nil, "", errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC()
	if now.IsZero() {
		return nil, "", errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", platformapp.ErrRateLimited
	}
	allowed, err := service.Authorizer.Check(ctx, "path", string(pathID), "manage_members", principal.UserID)
	if err != nil {
		return nil, "", err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(pathID), audit.Denied)
		event.OccurredAt = now
		if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
			return nil, "", err
		}
		return nil, "", platformapp.ErrForbidden
	}
	path, err := service.Paths.Get(ctx, principal.UserID, pathID)
	if err != nil {
		return nil, "", err
	}
	if path.ID != pathID || !validCreatedPath(path, path.OwnerUserID) {
		return nil, "", errInvalidInvitationDependencies
	}
	request := InvitationPageRequest{Limit: limit, Snapshot: now}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != managedInvitationCursorDomain || payload.Projection != string(pathID) || payload.Snapshot.After(now) {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterID, request.AfterCreated, request.Snapshot = domain.InvitationID(payload.AfterID), payload.AfterCreated, payload.Snapshot
	}
	page, err := service.InvitationManagement.ListManagedPending(ctx, principal.UserID, pathID, request)
	if err != nil {
		return nil, "", err
	}
	if !validManagedInvitationPage(page, pathID, request) {
		return nil, "", errInvalidInvitationDependencies
	}
	next := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1].Invitation
		next, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: managedInvitationCursorDomain, Projection: string(pathID), AfterID: string(last.ID), AfterCreated: last.CreatedAt, Snapshot: request.Snapshot})
		if err != nil {
			return nil, "", err
		}
	}
	event := shared.NewAuditEvent(ctx, service.Clock, path.OwnerUserID, principal.UserID, audit.PathInvitationListed, "path", string(pathID), audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	return page.Items, next, nil
}

func validManagedInvitationPage(page ManagedInvitationPage, pathID domain.ID, request InvitationPageRequest) bool {
	if len(page.Items) > request.Limit || (page.HasMore && len(page.Items) == 0) {
		return false
	}
	for index, item := range page.Items {
		invitation := item.Invitation
		if invitation.PathID != pathID || !invitation.Pending() || invitation.CreatedAt.After(request.Snapshot) || item.Inviter.UserID != invitation.InviterUserID || item.Recipient.UserID != invitation.RecipientUserID || !validPublicIdentity(item.Inviter) || !validPublicIdentity(item.Recipient) {
			return false
		}
		if index > 0 {
			previous := page.Items[index-1].Invitation
			if invitation.CreatedAt.After(previous.CreatedAt) || (invitation.CreatedAt.Equal(previous.CreatedAt) && invitation.ID >= previous.ID) {
				return false
			}
		}
	}
	return true
}

func validPublicIdentity(identity InvitationPublicIdentity) bool {
	return strings.TrimSpace(identity.UserID) != "" && identity.UserID == strings.TrimSpace(identity.UserID) && strings.TrimSpace(identity.Username) != "" && identity.Username == strings.TrimSpace(identity.Username) && strings.TrimSpace(identity.DisplayName) != "" && identity.DisplayName == strings.TrimSpace(identity.DisplayName)
}

func (service *InvitationService) Cancel(ctx context.Context, authorization, idempotencyKey string, pathID domain.ID, invitationID domain.InvitationID) (CancelInvitationResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return CancelInvitationResult{}, err
	}
	if pathID == "" || invitationID == "" || !validIdempotencyKey(idempotencyKey) {
		return CancelInvitationResult{}, ports.ErrInvalidArgument
	}
	if service.Paths == nil || service.Authorizer == nil || service.InvitationManagement == nil || service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil {
		return CancelInvitationResult{}, errInvalidInvitationDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return CancelInvitationResult{}, errInvalidInvitationDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return CancelInvitationResult{}, platformapp.ErrRateLimited
	}
	allowed, err := service.Authorizer.Check(ctx, "path", string(pathID), "manage_members", principal.UserID)
	if err != nil {
		return CancelInvitationResult{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(pathID), audit.Denied)
		event.OccurredAt = now
		if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
			return CancelInvitationResult{}, err
		}
		return CancelInvitationResult{}, platformapp.ErrForbidden
	}
	path, err := service.Paths.Get(ctx, principal.UserID, pathID)
	if err != nil {
		return CancelInvitationResult{}, err
	}
	if path.ID != pathID || !validCreatedPath(path, path.OwnerUserID) {
		return CancelInvitationResult{}, errInvalidInvitationDependencies
	}
	digest := canonicalCancelInvitationRequestHash(pathID, invitationID)
	idempotency := ports.Idempotency{PrincipalID: principal.UserID, Operation: CancelInvitationOperation, Key: idempotencyKey, RequestHash: digest[:]}
	decision, err := service.InvitationManagement.CancellationDecision(ctx, principal.UserID, pathID, invitationID, idempotency)
	if err != nil {
		if errors.Is(err, domain.ErrInvitationUnavailable) {
			return CancelInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
		}
		return CancelInvitationResult{}, err
	}
	if decision.Replay != nil {
		result := *decision.Replay
		if !result.Replayed || !validCanceledInvitationResult(result, pathID, invitationID) {
			return CancelInvitationResult{}, errInvalidInvitationDependencies
		}
		return result, nil
	}
	if path.Archived() {
		return CancelInvitationResult{}, ports.ErrConflict
	}
	if decision.Path.ID != pathID || decision.Path.OwnerUserID != path.OwnerUserID || decision.Invitation.ID != invitationID || decision.Invitation.PathID != pathID || !decision.Invitation.Pending() {
		return CancelInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
	}
	canceled, err := decision.Invitation.Cancel(now)
	if err != nil {
		return CancelInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
	}
	event := shared.NewAuditEvent(ctx, service.Clock, path.OwnerUserID, principal.UserID, audit.PathInvitationCanceled, "path_invitation", string(invitationID), audit.Succeeded)
	event.OccurredAt = now
	result, err := service.InvitationManagement.Cancel(ctx, CancelInvitationCommand{ActorUserID: principal.UserID, Invitation: canceled, Idempotency: idempotency, Audit: event})
	if err != nil {
		if errors.Is(err, domain.ErrInvitationUnavailable) {
			return CancelInvitationResult{}, service.denyUnavailableInvitation(ctx, principal.UserID, invitationID, now)
		}
		return CancelInvitationResult{}, err
	}
	if !validCanceledInvitationResult(result, pathID, invitationID) {
		return CancelInvitationResult{}, errInvalidInvitationDependencies
	}
	return result, nil
}

func canonicalCancelInvitationRequestHash(pathID domain.ID, invitationID domain.InvitationID) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(pathID))
	writeHashString(digest, string(invitationID))
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func validCanceledInvitationResult(result CancelInvitationResult, pathID domain.ID, invitationID domain.InvitationID) bool {
	invitation := result.Invitation
	return invitation.ID == invitationID && invitation.PathID == pathID && !invitation.CanceledAt.IsZero() && invitation.AcceptedAt.IsZero() && invitation.RejectedAt.IsZero() && !invitation.CanceledAt.Before(invitation.CreatedAt)
}
