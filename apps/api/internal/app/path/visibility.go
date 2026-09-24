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
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (s *Service) SetVisibility(ctx context.Context, authorization, key string, id domain.ID, confirmed bool, expectedVisibility, visibility string) (SetVisibilityResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return SetVisibilityResult{}, err
	}
	expectedVisibility, visibility = strings.TrimSpace(expectedVisibility), strings.TrimSpace(visibility)
	if !confirmed || id == "" || expectedVisibility == visibility || !validVisibility(expectedVisibility) || !validVisibility(visibility) || !validIdempotencyKey(key) {
		return SetVisibilityResult{}, ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.AuthorizationOutbox == nil || s.AuthorizationSerializer == nil || s.Repository == nil || s.Profiles == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil || s.AuthorizationWorker == "" || s.AuthorizationLease <= 0 {
		return SetVisibilityResult{}, errInvalidPathDependencies
	}
	now := s.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return SetVisibilityResult{}, errInvalidPathDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return SetVisibilityResult{}, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(id), "manage_visibility", principal.UserID)
	if err != nil {
		return SetVisibilityResult{}, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(id), audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
			return SetVisibilityResult{}, err
		}
		return SetVisibilityResult{}, platformapp.ErrForbidden
	}
	digest := canonicalSetVisibilityRequestHash(id, expectedVisibility, visibility)
	idempotency := ports.Idempotency{PrincipalID: principal.UserID, Operation: SetVisibilityOperation, Key: key, RequestHash: digest[:]}
	replay, err := s.Repository.SetVisibilityReplay(ctx, principal.UserID, id, idempotency)
	if err != nil {
		return SetVisibilityResult{}, err
	}
	if replay != nil {
		if !replay.Replayed || replay.Path.ID != id || replay.Path.OwnerUserID != principal.UserID || replay.Path.Visibility != visibility || !validCreatedPath(replay.Path, principal.UserID) {
			return SetVisibilityResult{}, errInvalidPathDependencies
		}
		if err := s.reconcileVisibilityAuthorization(ctx, id); err != nil {
			return SetVisibilityResult{}, err
		}
		return *replay, nil
	}
	profile, err := s.Profiles.PathCreationProfile(ctx, principal.UserID)
	if err != nil {
		return SetVisibilityResult{}, err
	}
	if identity.ValidateProfileVisibility(profile.ProfileVisibility) != nil {
		return SetVisibilityResult{}, errInvalidPathDependencies
	}
	if _, err := boundedVisibility(profile.ProfileVisibility, visibility); err != nil {
		return SetVisibilityResult{}, ports.ErrInvalidArgument
	}
	existing, err := s.Repository.Get(ctx, principal.UserID, id)
	if err != nil {
		return SetVisibilityResult{}, err
	}
	if existing.ID != id || existing.OwnerUserID != principal.UserID || !validCreatedPath(existing, existing.OwnerUserID) {
		return SetVisibilityResult{}, errInvalidPathDependencies
	}
	if existing.Archived() || existing.Visibility != expectedVisibility {
		return SetVisibilityResult{}, ports.ErrConflict
	}
	changedAt := now
	if !changedAt.After(existing.UpdatedAt) {
		changedAt = existing.UpdatedAt.Add(time.Microsecond)
	}
	updated, err := existing.SetVisibility(visibility, changedAt)
	if err != nil {
		return SetVisibilityResult{}, ports.ErrInvalidArgument
	}
	event := shared.NewAuditEvent(ctx, s.Clock, existing.OwnerUserID, principal.UserID, audit.PathVisibilityChanged, "path", string(id), audit.Succeeded)
	event.OccurredAt = changedAt
	result, err := s.Repository.SetVisibility(ctx, SetVisibilityCommand{
		ActorUserID: principal.UserID, ExpectedVisibility: expectedVisibility, Path: updated, ChangedAt: changedAt,
		Idempotency: idempotency, Audit: event,
		NewID: s.NewID, AuthorizationWorker: s.AuthorizationWorker, AuthorizationLease: s.AuthorizationLease,
	})
	if err != nil {
		return SetVisibilityResult{}, err
	}
	if result.Path.ID != existing.ID || result.Path.OwnerUserID != existing.OwnerUserID || result.Path.Name != existing.Name || result.Path.Visibility != visibility || result.Path.IntervalGoal != existing.IntervalGoal || result.Path.OverallTarget != existing.OverallTarget || result.Path.CreatedAt != existing.CreatedAt || result.Path.Archived() || (!result.Replayed && result.Path != updated) {
		return SetVisibilityResult{}, errInvalidPathDependencies
	}
	if !result.Replayed && !validVisibilityAuthorizationChanges(result.AuthorizationChanges, existing, result.Path, principal.UserID) {
		return SetVisibilityResult{}, errInvalidPathDependencies
	}
	if err := s.reconcileVisibilityAuthorization(ctx, id); err != nil {
		return SetVisibilityResult{}, err
	}
	return result, nil
}

func (s *Service) reconcileVisibilityAuthorization(ctx context.Context, id domain.ID) error {
	for {
		change, err := s.AuthorizationOutbox.ClaimAuthorizationChangeForResource(ctx, "path", string(id), s.AuthorizationWorker, s.AuthorizationLease)
		if errors.Is(err, ports.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if !validPendingPathAuthorizationChange(change, id) {
			return errInvalidPathDependencies
		}
		if err := s.reconcileRelationship(ctx, change); err != nil {
			return err
		}
	}
}

func validVisibilityAuthorizationChanges(changes []ports.AuthorizationChange, before, after domain.Entity, actor string) bool {
	expected := make([]struct {
		relation, subject string
		operation         ports.AuthorizationOperation
	}, 0, 2)
	appendExpected := func(visibility string, operation ports.AuthorizationOperation) {
		relation, subject := visibilityAuthorizationRelation(visibility, actor)
		if relation != "" {
			expected = append(expected, struct {
				relation, subject string
				operation         ports.AuthorizationOperation
			}{relation, subject, operation})
		}
	}
	appendExpected(before.Visibility, ports.AuthorizationDelete)
	appendExpected(after.Visibility, ports.AuthorizationTouch)
	if len(changes) != len(expected) {
		return false
	}
	for i, change := range changes {
		want := expected[i]
		if !validPendingPathAuthorizationChange(change, before.ID) || change.OwnerUserID != actor || change.ActorUserID != actor || change.Relation != want.relation || change.SubjectID != want.subject || change.Operation != want.operation {
			return false
		}
	}
	return true
}

func visibilityAuthorizationRelation(visibility, owner string) (string, string) {
	switch visibility {
	case "followers":
		return "followers_owner", owner
	case "public":
		return "public_viewer", "*"
	default:
		return "", ""
	}
}

func validVisibility(value string) bool {
	return value == "private" || value == "followers" || value == "public"
}

func canonicalSetVisibilityRequestHash(id domain.ID, expected, visibility string) [sha256.Size]byte {
	digest := sha256.New()
	writeHashString(digest, string(id))
	writeHashString(digest, expected)
	writeHashString(digest, visibility)
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}
