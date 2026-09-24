package path

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const memberListCursorDomain = "path-member-list"

func (s *Service) ListMembers(ctx context.Context, authorization string, pathID domain.ID, cursor string, limit int) ([]Member, string, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if pathID == "" || s.Authorizer == nil || s.MemberRemoval == nil || s.Clock == nil || len(s.CursorSigningKey) < 32 {
		return nil, "", errInvalidPathDependencies
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(pathID), "view", principal.UserID)
	if err != nil {
		return nil, "", err
	}
	if !allowed {
		return nil, "", platformapp.ErrForbidden
	}
	now := s.Clock.Now().UTC()
	request := MemberPageRequest{Limit: limit, Snapshot: now}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(s.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != memberListCursorDomain+":"+string(pathID) || payload.Snapshot.After(now) || !validProjectionFingerprint(payload.Projection) {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterUserID, request.AfterCreated, request.Snapshot, request.ExpectedProjectionFingerprint = payload.AfterID, payload.AfterCreated, payload.Snapshot, payload.Projection
	}
	page, err := s.MemberRemoval.ListMembers(ctx, MemberListQuery{ActorUserID: principal.UserID, PathID: pathID}, request)
	if err != nil {
		return nil, "", err
	}
	if len(page.Items) > limit || (page.HasMore && len(page.Items) == 0) || !validProjectionFingerprint(page.ProjectionFingerprint) {
		return nil, "", errInvalidPathDependencies
	}
	if request.ExpectedProjectionFingerprint != "" && page.ProjectionFingerprint != request.ExpectedProjectionFingerprint {
		return nil, "", ports.ErrInvalidArgument
	}
	var progressShapeSet bool
	var intervalTarget, overallTarget int64
	var intervalPresent, overallPresent bool
	for _, member := range page.Items {
		if member.UserID == "" || strings.TrimSpace(member.Username) == "" || strings.TrimSpace(member.DisplayName) == "" || (member.Role != "creator" && member.Role != "administrator" && member.Role != "participant" && member.Role != "supporter") || member.SessionCount < 0 || member.TotalTrackedSeconds < 0 || !validGoalProgress(member.IntervalProgress) || !validGoalProgress(member.OverallProgress) || (member.IntervalProgress != nil && member.IntervalProgress.AccumulatedSeconds > member.TotalTrackedSeconds) || (member.OverallProgress != nil && member.OverallProgress.AccumulatedSeconds != member.TotalTrackedSeconds) || (member.Role == "supporter" && (member.IntervalProgress != nil || member.OverallProgress != nil)) || member.CreatedAt.IsZero() || member.CreatedAt.After(request.Snapshot) || ((member.CanRemove || member.CanChangeRole) && (member.Role != "participant" && member.Role != "supporter" || member.UserID == principal.UserID)) || (member.CanGrantAdministrator && (member.Role != "participant" || member.UserID == principal.UserID)) || (member.CanRevokeAdministrator && (member.Role != "administrator" || member.UserID == principal.UserID)) || (member.CanStepDownAdministrator && (member.Role != "administrator" || member.UserID != principal.UserID)) || (member.CanLeave && member.UserID != principal.UserID) {
			return nil, "", errInvalidPathDependencies
		}
		if member.Role != "supporter" {
			if !progressShapeSet {
				intervalPresent, overallPresent = member.IntervalProgress != nil, member.OverallProgress != nil
				if intervalPresent {
					intervalTarget = member.IntervalProgress.TargetSeconds
				}
				if overallPresent {
					overallTarget = member.OverallProgress.TargetSeconds
				}
				progressShapeSet = true
			} else if (member.IntervalProgress != nil) != intervalPresent || (member.OverallProgress != nil) != overallPresent || member.IntervalProgress != nil && member.IntervalProgress.TargetSeconds != intervalTarget || member.OverallProgress != nil && member.OverallProgress.TargetSeconds != overallTarget {
				return nil, "", errInvalidPathDependencies
			}
		}
	}
	next := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		next, err = shared.EncodeCursor(s.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: memberListCursorDomain + ":" + string(pathID), AfterID: last.UserID, AfterCreated: last.CreatedAt, Snapshot: request.Snapshot, Projection: page.ProjectionFingerprint})
		if err != nil {
			return nil, "", errInvalidPathDependencies
		}
	}
	return page.Items, next, nil
}

func validProjectionFingerprint(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func validGoalProgress(progress *GoalProgress) bool {
	return progress == nil || progress.AccumulatedSeconds >= 0 && progress.TargetSeconds > 0
}

func (s *Service) ReviewMemberRemoval(ctx context.Context, authorization string, pathID domain.ID, target string) (MemberRemovalReview, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return MemberRemovalReview{}, err
	}
	if pathID == "" || strings.TrimSpace(target) != target || target == "" {
		return MemberRemovalReview{}, ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.MemberRemoval == nil {
		return MemberRemovalReview{}, errInvalidPathDependencies
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(pathID), "manage_members", principal.UserID)
	if err != nil {
		return MemberRemovalReview{}, err
	}
	if !allowed {
		return MemberRemovalReview{}, platformapp.ErrForbidden
	}
	review, err := s.MemberRemoval.ReviewMemberRemoval(ctx, MemberRemovalQuery{ActorUserID: principal.UserID, TargetUserID: target, PathID: pathID})
	if err != nil {
		return MemberRemovalReview{}, err
	}
	if review.UserID != target || strings.TrimSpace(review.Username) == "" || strings.TrimSpace(review.DisplayName) == "" || !review.Role.Valid() || review.SessionCount < 0 || review.TotalTrackedSeconds < 0 {
		return MemberRemovalReview{}, errInvalidPathDependencies
	}
	return review, nil
}

func (s *Service) RemoveMember(ctx context.Context, authorization, key string, pathID domain.ID, target string, confirmed bool, expectedRole domain.MembershipRole) (RemoveMemberResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return RemoveMemberResult{}, err
	}
	if pathID == "" || strings.TrimSpace(target) != target || target == "" || target == principal.UserID || !confirmed || !expectedRole.Valid() || !validIdempotencyKey(key) {
		return RemoveMemberResult{}, ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.AuthorizationOutbox == nil || s.AuthorizationSerializer == nil || s.MemberRemoval == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil || s.AuthorizationWorker == "" || s.AuthorizationLease <= 0 {
		return RemoveMemberResult{}, errInvalidPathDependencies
	}
	digest := canonicalRemoveMemberRequestHash(pathID, target, confirmed, expectedRole)
	idempotency := ports.Idempotency{PrincipalID: principal.UserID, Operation: RemoveMemberOperation, Key: key, RequestHash: digest[:]}
	replayed, err := s.MemberRemoval.RemoveMemberReplay(ctx, principal.UserID, pathID, target, idempotency)
	if err != nil {
		return RemoveMemberResult{}, err
	}
	if replayed {
		if err := s.reconcilePathLeaveAuthorization(ctx, pathID); err != nil {
			return RemoveMemberResult{}, err
		}
		return RemoveMemberResult{PathID: pathID, UserID: target, Removed: true, ActivityDeleted: expectedRole == domain.RoleParticipant, Replayed: true}, nil
	}
	now := s.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return RemoveMemberResult{}, errInvalidPathDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return RemoveMemberResult{}, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(pathID), "manage_members", principal.UserID)
	if err != nil {
		return RemoveMemberResult{}, err
	}
	if !allowed {
		denied := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(pathID), audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, denied); err != nil {
			return RemoveMemberResult{}, err
		}
		return RemoveMemberResult{}, platformapp.ErrForbidden
	}
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "path_member", string(pathID)+":"+target, audit.Succeeded)
	event.OccurredAt = now
	notification := MemberAccessNotification{ID: s.NewID(), RecipientUserID: target, ActorUserID: principal.UserID, PathID: pathID, Kind: NotificationPathMemberRemoved, Role: expectedRole, CreatedAt: now}
	result, err := s.MemberRemoval.RemoveMember(ctx, RemoveMemberCommand{ActorUserID: principal.UserID, TargetUserID: target, PathID: pathID, ExpectedRole: expectedRole, RemovedAt: now, Idempotency: idempotency, Audit: event, Notification: notification, NewID: s.NewID, AuthorizationWorker: s.AuthorizationWorker, AuthorizationLease: s.AuthorizationLease})
	if err != nil {
		return RemoveMemberResult{}, err
	}
	if result.Replayed {
		if err := s.reconcilePathLeaveAuthorization(ctx, pathID); err != nil {
			return RemoveMemberResult{}, err
		}
		return RemoveMemberResult{PathID: pathID, UserID: target, Removed: true, ActivityDeleted: expectedRole == domain.RoleParticipant, Replayed: true}, nil
	}
	if result.PathID != pathID || result.UserID != target || !result.Removed || result.ActivityDeleted != (expectedRole == domain.RoleParticipant) || len(result.AuthorizationChanges) != 1 || !validRemovedMemberAuthorizationChange(result.AuthorizationChanges[0], pathID, principal.UserID, target, string(expectedRole)) {
		return RemoveMemberResult{}, errInvalidPathDependencies
	}
	if err := s.reconcilePathLeaveAuthorization(ctx, pathID); err != nil {
		return RemoveMemberResult{}, err
	}
	return result, nil
}

func (s *Service) ChangeMemberRole(ctx context.Context, authorization, key string, pathID domain.ID, target string, confirmed bool, expectedRole, role domain.MembershipRole) (ChangeMemberRoleResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return ChangeMemberRoleResult{}, err
	}
	if pathID == "" || strings.TrimSpace(target) != target || target == "" || !confirmed || !validMemberRoleTransition(principal.UserID, target, expectedRole, role) || !validIdempotencyKey(key) {
		return ChangeMemberRoleResult{}, ports.ErrInvalidArgument
	}
	if s.Authorizer == nil || s.AuthorizationOutbox == nil || s.AuthorizationSerializer == nil || s.MemberRemoval == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil || s.AuthorizationWorker == "" || s.AuthorizationLease <= 0 {
		return ChangeMemberRoleResult{}, errInvalidPathDependencies
	}
	digest := canonicalChangeMemberRoleRequestHash(pathID, target, confirmed, expectedRole, role)
	idempotency := ports.Idempotency{PrincipalID: principal.UserID, Operation: ChangeMemberRoleOperation, Key: key, RequestHash: digest[:]}
	replayed, err := s.MemberRemoval.ChangeMemberRoleReplay(ctx, principal.UserID, pathID, target, idempotency)
	if err != nil {
		return ChangeMemberRoleResult{}, err
	}
	if replayed {
		if err := s.reconcilePathLeaveAuthorization(ctx, pathID); err != nil {
			return ChangeMemberRoleResult{}, err
		}
		return ChangeMemberRoleResult{PathID: pathID, UserID: target, Role: role, ActivityDeleted: deletesActivity(expectedRole, role), Replayed: true}, nil
	}
	now := s.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return ChangeMemberRoleResult{}, errInvalidPathDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return ChangeMemberRoleResult{}, platformapp.ErrRateLimited
	}
	permission := "manage_members"
	if expectedRole == domain.RoleAdministrator || role == domain.RoleAdministrator {
		permission = "manage_administrators"
		if principal.UserID == target {
			permission = "step_down_administrator"
		}
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(pathID), permission, principal.UserID)
	if err != nil {
		return ChangeMemberRoleResult{}, err
	}
	if !allowed {
		denied := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path", string(pathID), audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, denied); err != nil {
			return ChangeMemberRoleResult{}, err
		}
		return ChangeMemberRoleResult{}, platformapp.ErrForbidden
	}
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "path_member", string(pathID)+":"+target, audit.Succeeded)
	event.OccurredAt = now
	notification := MemberAccessNotification{ID: s.NewID(), RecipientUserID: target, ActorUserID: principal.UserID, PathID: pathID, Kind: NotificationPathMemberRoleChanged, Role: role, CreatedAt: now}
	result, err := s.MemberRemoval.ChangeMemberRole(ctx, ChangeMemberRoleCommand{ActorUserID: principal.UserID, TargetUserID: target, PathID: pathID, ExpectedRole: expectedRole, Role: role, ChangedAt: now, Idempotency: idempotency, Audit: event, Notification: notification, NewID: s.NewID, AuthorizationWorker: s.AuthorizationWorker, AuthorizationLease: s.AuthorizationLease})
	if err != nil {
		return ChangeMemberRoleResult{}, err
	}
	if result.Replayed {
		if err := s.reconcilePathLeaveAuthorization(ctx, pathID); err != nil {
			return ChangeMemberRoleResult{}, err
		}
		return ChangeMemberRoleResult{PathID: pathID, UserID: target, Role: role, ActivityDeleted: deletesActivity(expectedRole, role), Replayed: true}, nil
	}
	if result.PathID != pathID || result.UserID != target || result.Role != role || result.ActivityDeleted != deletesActivity(expectedRole, role) || len(result.AuthorizationChanges) != 2 || !validChangedRoleAuthorizationChanges(result.AuthorizationChanges, pathID, principal.UserID, target, expectedRole, role) {
		return ChangeMemberRoleResult{}, errInvalidPathDependencies
	}
	if err := s.reconcilePathLeaveAuthorization(ctx, pathID); err != nil {
		return ChangeMemberRoleResult{}, err
	}
	return result, nil
}

func validMemberRoleTransition(actor, target string, from, to domain.MembershipRole) bool {
	if !from.ValidPathMemberRole() || !to.ValidPathMemberRole() || from == to {
		return false
	}
	if actor == target {
		return from == domain.RoleAdministrator && to == domain.RoleParticipant
	}
	return from.Valid() && to.Valid() ||
		from == domain.RoleParticipant && to == domain.RoleAdministrator ||
		from == domain.RoleAdministrator && to == domain.RoleParticipant
}

func deletesActivity(from, to domain.MembershipRole) bool {
	return from == domain.RoleParticipant && to == domain.RoleSupporter
}

func canonicalChangeMemberRoleRequestHash(pathID domain.ID, target string, confirmed bool, expectedRole, role domain.MembershipRole) [sha256.Size]byte {
	h := sha256.New()
	writeHashString(h, string(pathID))
	writeHashString(h, target)
	writeHashBool(h, confirmed)
	writeHashString(h, string(expectedRole))
	writeHashString(h, string(role))
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

func validChangedRoleAuthorizationChanges(changes []ports.AuthorizationChange, pathID domain.ID, actor, target string, expectedRole, role domain.MembershipRole) bool {
	if len(changes) != 2 {
		return false
	}
	seenDelete, seenTouch := false, false
	for _, change := range changes {
		if change.ID == "" || change.ResourceType != "path" || change.ResourceID != string(pathID) || change.SubjectType != "user" || change.SubjectID != target || change.OwnerUserID == "" || change.ActorUserID != actor || change.LockedBy == "" || change.Lease <= 0 {
			return false
		}
		if change.Relation == string(expectedRole) && change.Operation == ports.AuthorizationDelete {
			seenDelete = true
		} else if change.Relation == string(role) && change.Operation == ports.AuthorizationTouch {
			seenTouch = true
		} else {
			return false
		}
	}
	return seenDelete && seenTouch
}

func canonicalRemoveMemberRequestHash(pathID domain.ID, target string, confirmed bool, role domain.MembershipRole) [sha256.Size]byte {
	h := sha256.New()
	writeHashString(h, string(pathID))
	writeHashString(h, target)
	writeHashBool(h, confirmed)
	writeHashString(h, string(role))
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}
func validRemovedMemberAuthorizationChange(change ports.AuthorizationChange, pathID domain.ID, actor, target, role string) bool {
	return change.ID != "" && change.ResourceType == "path" && change.ResourceID == string(pathID) && change.Relation == role && change.SubjectType == "user" && change.SubjectID == target && change.OwnerUserID != "" && change.ActorUserID == actor && change.Operation == ports.AuthorizationDelete && change.LockedBy != "" && change.Lease > 0
}
