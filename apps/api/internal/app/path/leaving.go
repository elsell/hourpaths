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

func (s *Service) Leave(ctx context.Context, authorization, idempotencyKey string, pathID domain.ID, confirmed, retainActivity bool) (LeavePathResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return LeavePathResult{}, err
	}
	if pathID == "" || !confirmed || !validIdempotencyKey(idempotencyKey) {
		return LeavePathResult{}, ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.AuthorizationOutbox == nil || s.AuthorizationSerializer == nil || s.Repository == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil || s.AuthorizationWorker == "" || s.AuthorizationLease <= 0 {
		return LeavePathResult{}, errInvalidPathDependencies
	}

	digest := canonicalLeavePathRequestHash(pathID, confirmed, retainActivity)
	idempotency := ports.Idempotency{PrincipalID: principal.UserID, Operation: LeavePathOperation, Key: idempotencyKey, RequestHash: digest[:]}
	replayed, err := s.Repository.LeavePathReplay(ctx, principal.UserID, pathID, idempotency)
	if err != nil {
		return LeavePathResult{}, err
	}
	if replayed {
		if err := s.reconcilePathLeaveAuthorization(ctx, pathID); err != nil {
			return LeavePathResult{}, err
		}
		return LeavePathResult{PathID: pathID, Left: true, ActivityRetained: retainActivity, Replayed: true}, nil
	}

	now := s.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return LeavePathResult{}, errInvalidPathDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return LeavePathResult{}, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(pathID), "leave", principal.UserID)
	if err != nil {
		return LeavePathResult{}, err
	}
	if !allowed {
		denied := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(pathID), audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, denied); err != nil {
			return LeavePathResult{}, err
		}
		return LeavePathResult{}, platformapp.ErrForbidden
	}
	entity, err := s.Repository.Get(ctx, principal.UserID, pathID)
	if err != nil {
		return LeavePathResult{}, err
	}
	if entity.Archived() || entity.OwnerUserID == principal.UserID {
		denied := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(pathID), audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, denied); err != nil {
			return LeavePathResult{}, err
		}
		return LeavePathResult{}, platformapp.ErrForbidden
	}
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "path", string(pathID), audit.Succeeded)
	event.OccurredAt = now
	result, err := s.Repository.LeavePath(ctx, LeavePathCommand{ActorUserID: principal.UserID, PathID: pathID, LeftAt: now, RetainActivity: retainActivity, Idempotency: idempotency, Audit: event, NewID: s.NewID, AuthorizationWorker: s.AuthorizationWorker, AuthorizationLease: s.AuthorizationLease})
	if err != nil {
		return LeavePathResult{}, err
	}
	if result.Replayed {
		if err := s.reconcilePathLeaveAuthorization(ctx, pathID); err != nil {
			return LeavePathResult{}, err
		}
		return LeavePathResult{PathID: pathID, Left: true, ActivityRetained: retainActivity, Replayed: true}, nil
	}
	if result.PathID != pathID || !result.Left || result.ActivityRetained != retainActivity || len(result.AuthorizationChanges) != 1 || !validLeaveAuthorizationChange(result.AuthorizationChanges[0], pathID, principal.UserID) {
		return LeavePathResult{}, errInvalidPathDependencies
	}

	if err := s.reconcilePathLeaveAuthorization(ctx, pathID); err != nil {
		return LeavePathResult{}, err
	}
	return result, nil
}

func (s *Service) reconcilePathLeaveAuthorization(ctx context.Context, pathID domain.ID) error {
	for {
		change, claimErr := s.AuthorizationOutbox.ClaimAuthorizationChangeForResource(ctx, "path", string(pathID), s.AuthorizationWorker, s.AuthorizationLease)
		if errors.Is(claimErr, ports.ErrNotFound) {
			break
		}
		if errors.Is(claimErr, ports.ErrAuthorizationPending) {
			return ports.ErrAuthorizationPending
		}
		if claimErr != nil {
			return claimErr
		}
		if !validPendingPathAuthorizationChange(change, pathID) {
			return errInvalidPathDependencies
		}
		if err := s.reconcileRelationship(ctx, change); err != nil {
			return err
		}
	}
	return nil
}

func canonicalLeavePathRequestHash(pathID domain.ID, confirmed, retainActivity bool) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(pathID))
	writeHashBool(digest, confirmed)
	writeHashBool(digest, retainActivity)
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func validLeaveAuthorizationChange(change ports.AuthorizationChange, pathID domain.ID, actor string) bool {
	validRole := change.Relation == "administrator" || change.Relation == "participant" || change.Relation == "supporter"
	return change.ID != "" && change.ResourceType == "path" && change.ResourceID == string(pathID) && validRole &&
		change.SubjectType == "user" && change.SubjectID == actor && strings.TrimSpace(change.OwnerUserID) == change.OwnerUserID && change.OwnerUserID != "" &&
		change.ActorUserID == actor && change.Operation == ports.AuthorizationDelete && change.LockedBy != "" && change.Lease > 0
}
