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

func (s *Service) Delete(ctx context.Context, authorization, idempotencyKey string, id domain.ID, confirmed bool, expectedName string) (DeletePathResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return DeletePathResult{}, err
	}
	if !confirmed || id == "" || !validIdempotencyKey(idempotencyKey) || strings.TrimSpace(expectedName) != expectedName || !domainNameValid(expectedName) {
		return DeletePathResult{}, ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.AuthorizationOutbox == nil || s.AuthorizationSerializer == nil || s.Repository == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil || s.AuthorizationWorker == "" || s.AuthorizationLease <= 0 {
		return DeletePathResult{}, errInvalidPathDependencies
	}
	digest := canonicalDeletePathRequestHash(id, confirmed, expectedName)
	idempotency := ports.Idempotency{PrincipalID: principal.UserID, Operation: DeletePathOperation, Key: idempotencyKey, RequestHash: digest[:]}
	replayed, err := s.Repository.DeletionReplay(ctx, principal.UserID, id, idempotency)
	if err != nil {
		return DeletePathResult{}, err
	}
	if replayed {
		return DeletePathResult{PathID: id, Deleted: true, Replayed: true}, nil
	}
	now := s.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return DeletePathResult{}, errInvalidPathDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return DeletePathResult{}, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(id), "manage_lifecycle", principal.UserID)
	if err != nil {
		return DeletePathResult{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(id), audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
			return DeletePathResult{}, err
		}
		return DeletePathResult{}, platformapp.ErrForbidden
	}
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceDeleted, "path", string(id), audit.Succeeded)
	event.OccurredAt = now
	result, err := s.Repository.DeletePath(ctx, DeletePathCommand{
		ActorUserID: principal.UserID, PathID: id, ExpectedName: expectedName, DeletedAt: now,
		Idempotency: idempotency, Audit: event, NewID: s.NewID,
		AuthorizationWorker: s.AuthorizationWorker, AuthorizationLease: s.AuthorizationLease,
	})
	if err != nil {
		return DeletePathResult{}, err
	}
	if result.Replayed {
		return DeletePathResult{PathID: id, Deleted: true, Replayed: true}, nil
	}
	if result.PathID != id || !result.Deleted {
		return DeletePathResult{}, errInvalidPathDependencies
	}
	cleanupChanges := make(map[string]struct{}, len(result.AuthorizationChanges))
	for _, change := range result.AuthorizationChanges {
		if !validDeleteAuthorizationChange(change, id, principal.UserID) {
			return DeletePathResult{}, errInvalidPathDependencies
		}
		if _, duplicate := cleanupChanges[change.ID]; duplicate {
			return DeletePathResult{}, errInvalidPathDependencies
		}
		cleanupChanges[change.ID] = struct{}{}
	}
	seen := make(map[string]struct{})
	for {
		change, claimErr := s.AuthorizationOutbox.ClaimAuthorizationChangeForResource(ctx, "path", string(id), s.AuthorizationWorker, s.AuthorizationLease)
		if errors.Is(claimErr, ports.ErrNotFound) {
			break
		}
		if errors.Is(claimErr, ports.ErrAuthorizationPending) {
			// An earlier atomic relationship batch owns this resource. The shared
			// workers will apply it before the durable deletion changes.
			break
		}
		if claimErr != nil {
			return DeletePathResult{}, claimErr
		}
		if !validPendingPathAuthorizationChange(change, id) {
			return DeletePathResult{}, errInvalidPathDependencies
		}
		if _, duplicate := seen[change.ID]; duplicate {
			return DeletePathResult{}, errInvalidPathDependencies
		}
		seen[change.ID] = struct{}{}
		if _, cleanup := cleanupChanges[change.ID]; cleanup && !validDeleteAuthorizationChange(change, id, principal.UserID) {
			return DeletePathResult{}, errInvalidPathDependencies
		}
		if err := s.reconcileRelationship(ctx, change); err != nil {
			return DeletePathResult{}, err
		}
	}
	return result, nil
}

func validPendingPathAuthorizationChange(change ports.AuthorizationChange, id domain.ID) bool {
	validOperation := change.Operation == ports.AuthorizationTouch || change.Operation == ports.AuthorizationDelete
	validRelation := change.Relation == "creator" || change.Relation == "administrator" || change.Relation == "participant" || change.Relation == "supporter" ||
		change.Relation == "followers_owner" || change.Relation == "public_viewer"
	validSubject := strings.TrimSpace(change.SubjectID) == change.SubjectID && change.SubjectID != ""
	if change.Relation == "followers_owner" {
		validSubject = change.SubjectID == change.OwnerUserID
	} else if change.Relation == "public_viewer" {
		validSubject = change.SubjectID == "*"
	}
	return change.ID != "" && change.ResourceType == "path" && change.ResourceID == string(id) && validRelation &&
		change.SubjectType == "user" && validSubject &&
		strings.TrimSpace(change.OwnerUserID) == change.OwnerUserID && change.OwnerUserID != "" &&
		strings.TrimSpace(change.ActorUserID) == change.ActorUserID && change.ActorUserID != "" && validOperation &&
		strings.TrimSpace(change.LockedBy) == change.LockedBy && change.LockedBy != "" && change.Lease > 0
}

func canonicalDeletePathRequestHash(id domain.ID, confirmed bool, expectedName string) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(id))
	writeHashBool(digest, confirmed)
	writeHashString(digest, expectedName)
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func domainNameValid(name string) bool {
	_, err := domain.New("validation", "validation", domain.Attributes{Name: name, Visibility: "private"})
	return err == nil
}

func validDeleteAuthorizationChange(change ports.AuthorizationChange, id domain.ID, owner string) bool {
	validRelationAndSubject := change.Relation == "creator" || change.Relation == "administrator" || change.Relation == "participant" || change.Relation == "supporter"
	if change.Relation == "followers_owner" {
		validRelationAndSubject = change.SubjectID == owner
	} else if change.Relation == "public_viewer" {
		validRelationAndSubject = change.SubjectID == "*"
	}
	return change.ID != "" && change.ResourceType == "path" && change.ResourceID == string(id) &&
		change.SubjectType == "user" && change.SubjectID != "" && change.OwnerUserID == owner &&
		change.ActorUserID == owner && change.Operation == ports.AuthorizationDelete &&
		change.LockedBy != "" && change.Lease > 0 && validRelationAndSubject
}
