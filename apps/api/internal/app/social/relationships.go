package social

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	FollowOperation              = "social.follow"
	CancelFollowRequestOperation = "social.follow_request.cancel"
	UnfollowOperation            = "social.unfollow"
	AcceptFollowRequestOperation = "social.follow_request.accept"
	RejectFollowRequestOperation = "social.follow_request.reject"
)

type FollowRequestDecision string

const (
	FollowRequestAccepted FollowRequestDecision = "accepted"
	FollowRequestRejected FollowRequestDecision = "rejected"
)

type RelationshipCommand struct {
	ActorUserID, TargetUsername, RequestID string
	OccurredAt                             time.Time
	Idempotency                            ports.Idempotency
	Audit                                  audit.Event
}

type RelationshipResult struct {
	Target              domain.PublicProfile
	RequestID           string
	Changed             bool
	AuthorizationChange ports.AuthorizationChange
	Replayed            bool
}

type ReviewResult struct {
	Request             domain.FollowRequest
	Decision            FollowRequestDecision
	AuthorizationChange ports.AuthorizationChange
	Replayed            bool
}

type FollowRequestPage struct {
	Requests []domain.FollowRequest
	HasMore  bool
}

type RelationshipRepository interface {
	Follow(context.Context, RelationshipCommand) (RelationshipResult, error)
	CancelRequest(context.Context, RelationshipCommand) (RelationshipResult, error)
	Unfollow(context.Context, RelationshipCommand) (RelationshipResult, error)
	AcceptRequest(context.Context, RelationshipCommand) (ReviewResult, error)
	RejectRequest(context.Context, RelationshipCommand) (ReviewResult, error)
	ListIncoming(context.Context, string, ports.PageRequest) (FollowRequestPage, error)
}

type RelationshipRateLimiter interface {
	Allow(actorUserID, target string, now time.Time) bool
}

type AuthorizationChangeState string

const (
	AuthorizationChangePending      AuthorizationChangeState = "pending"
	AuthorizationChangeLocked       AuthorizationChangeState = "locked"
	AuthorizationChangeCompleted    AuthorizationChangeState = "completed"
	AuthorizationChangeDeadLettered AuthorizationChangeState = "dead_lettered"
)

type AuthorizationChangeStatusReader interface {
	AuthorizationChangeState(context.Context, string) (AuthorizationChangeState, error)
}

func (service *Service) Follow(ctx context.Context, authorization, targetUsername, idempotencyKey string) (RelationshipResult, error) {
	return service.mutateTarget(ctx, authorization, targetUsername, idempotencyKey, FollowOperation)
}

func (service *Service) CancelRequest(ctx context.Context, authorization, targetUsername, idempotencyKey string) (RelationshipResult, error) {
	return service.mutateTarget(ctx, authorization, targetUsername, idempotencyKey, CancelFollowRequestOperation)
}

func (service *Service) Unfollow(ctx context.Context, authorization, targetUsername, idempotencyKey string) (RelationshipResult, error) {
	return service.mutateTarget(ctx, authorization, targetUsername, idempotencyKey, UnfollowOperation)
}

func (service *Service) mutateTarget(ctx context.Context, authorization, rawTargetUsername, idempotencyKey, operation string) (RelationshipResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return RelationshipResult{}, err
	}
	targetUsername, err := domain.NormalizeUsername(rawTargetUsername)
	if err != nil || !validRelationshipIdempotencyKey(idempotencyKey) {
		return RelationshipResult{}, ports.ErrInvalidArgument
	}
	now, err := service.relationshipMutationReady(principal.UserID, "profile:"+targetUsername)
	if err != nil {
		return RelationshipResult{}, err
	}
	action, targetType := audit.ResourceCreated, "profile_follow"
	switch operation {
	case FollowOperation:
	case CancelFollowRequestOperation:
		action, targetType = audit.ResourceDeleted, "follow_request"
	case UnfollowOperation:
		action = audit.ResourceDeleted
	default:
		return RelationshipResult{}, errInvalidDependencies
	}
	command := RelationshipCommand{
		ActorUserID: principal.UserID, TargetUsername: targetUsername, OccurredAt: now,
		Idempotency: relationshipIdempotency(principal.UserID, operation, idempotencyKey, targetUsername),
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, action, targetType, targetUsername, audit.Succeeded),
	}
	command.Audit.OccurredAt = now
	var result RelationshipResult
	switch operation {
	case FollowOperation:
		result, err = service.Relationships.Follow(ctx, command)
	case CancelFollowRequestOperation:
		result, err = service.Relationships.CancelRequest(ctx, command)
	case UnfollowOperation:
		result, err = service.Relationships.Unfollow(ctx, command)
	}
	if err != nil {
		return RelationshipResult{}, service.relationshipError(ctx, principal.UserID, err, now)
	}
	if !validRelationshipResult(result, principal.UserID, targetUsername, operation) {
		return RelationshipResult{}, errInvalidDependencies
	}
	if result.AuthorizationChange.ID != "" {
		if err := service.reconcileAuthorizationChange(ctx, result.AuthorizationChange, result.Replayed); err != nil {
			return RelationshipResult{}, err
		}
	}
	return result, nil
}

func (service *Service) AcceptRequest(ctx context.Context, authorization, requestID, idempotencyKey string) (ReviewResult, error) {
	return service.reviewRequest(ctx, authorization, requestID, idempotencyKey, AcceptFollowRequestOperation)
}

func (service *Service) RejectRequest(ctx context.Context, authorization, requestID, idempotencyKey string) (ReviewResult, error) {
	return service.reviewRequest(ctx, authorization, requestID, idempotencyKey, RejectFollowRequestOperation)
}

func (service *Service) reviewRequest(ctx context.Context, authorization, requestID, idempotencyKey, operation string) (ReviewResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return ReviewResult{}, err
	}
	if !validOpaqueID(requestID) || !validRelationshipIdempotencyKey(idempotencyKey) {
		return ReviewResult{}, ports.ErrInvalidArgument
	}
	now, err := service.relationshipMutationReady(principal.UserID, "follow_request:"+requestID)
	if err != nil {
		return ReviewResult{}, err
	}
	action, decision := audit.ResourceUpdated, FollowRequestAccepted
	if operation == RejectFollowRequestOperation {
		action, decision = audit.ResourceDeleted, FollowRequestRejected
	} else if operation != AcceptFollowRequestOperation {
		return ReviewResult{}, errInvalidDependencies
	}
	command := RelationshipCommand{
		ActorUserID: principal.UserID, RequestID: requestID, OccurredAt: now,
		Idempotency: relationshipIdempotency(principal.UserID, operation, idempotencyKey, requestID),
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, action, "follow_request", requestID, audit.Succeeded),
	}
	command.Audit.OccurredAt = now
	var result ReviewResult
	if operation == AcceptFollowRequestOperation {
		result, err = service.Relationships.AcceptRequest(ctx, command)
	} else {
		result, err = service.Relationships.RejectRequest(ctx, command)
	}
	if err != nil {
		return ReviewResult{}, service.relationshipError(ctx, principal.UserID, err, now)
	}
	if !validReviewResult(result, principal.UserID, requestID, decision, now) {
		return ReviewResult{}, errInvalidDependencies
	}
	if result.AuthorizationChange.ID != "" {
		if err := service.reconcileAuthorizationChange(ctx, result.AuthorizationChange, result.Replayed); err != nil {
			return ReviewResult{}, err
		}
	}
	return result, nil
}

func (service *Service) ListIncomingRequests(ctx context.Context, authorization, cursor string, limit int) ([]domain.FollowRequest, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if service.Relationships == nil || service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil || len(service.CursorSigningKey) < 32 {
		return nil, "", errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return nil, "", errInvalidDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", platformapp.ErrRateLimited
	}
	pageRequest := ports.PageRequest{Snapshot: now, Limit: limit}
	if cursor != "" {
		claims, decodeErr := decodeFollowRequestCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || claims.Viewer != principal.UserID {
			return nil, "", ports.ErrInvalidArgument
		}
		pageRequest.Snapshot = claims.Snapshot
		pageRequest.AfterCreated = claims.AfterCreated
		pageRequest.AfterID = claims.AfterID
	}
	page, err := service.Relationships.ListIncoming(ctx, principal.UserID, pageRequest)
	if err != nil {
		return nil, "", err
	}
	if !validFollowRequestPage(page, principal.UserID, pageRequest) {
		return nil, "", errInvalidDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "follow_request", "incoming", audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	next := ""
	if page.HasMore {
		last := page.Requests[len(page.Requests)-1]
		next, err = encodeFollowRequestCursor(service.CursorSigningKey, followRequestCursorClaims{
			Version: 1, Viewer: principal.UserID, Snapshot: pageRequest.Snapshot,
			AfterCreated: last.CreatedAt, AfterID: last.ID,
		})
		if err != nil {
			return nil, "", err
		}
	}
	return page.Requests, next, nil
}

func (service *Service) relationshipMutationReady(actor, target string) (time.Time, error) {
	if service.Relationships == nil || service.RelationshipRateLimiter == nil || service.Authorizer == nil ||
		service.AuthorizationOutbox == nil || service.AuthorizationStatus == nil || service.AuthorizationSerializer == nil ||
		service.Audits == nil || service.Clock == nil || service.AuthorizationWorker == "" || service.AuthorizationLease <= 0 {
		return time.Time{}, errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return time.Time{}, errInvalidDependencies
	}
	if !service.RelationshipRateLimiter.Allow(actor, target, now) {
		return time.Time{}, platformapp.ErrRateLimited
	}
	return now, nil
}

func (service *Service) relationshipError(ctx context.Context, actor string, repositoryError error, now time.Time) error {
	if !errors.Is(repositoryError, ports.ErrNotFound) {
		return repositoryError
	}
	denial := shared.NewAuditEvent(ctx, service.Clock, actor, actor, audit.ResourceAccessDenied, "profile", "hidden", audit.Denied)
	denial.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, denial); err != nil {
		return err
	}
	return ports.ErrNotFound
}

func validRelationshipResult(result RelationshipResult, actor, username, operation string) bool {
	target := result.Target
	if !target.Valid() || target.ID == actor || !strings.EqualFold(target.Username, username) || target.Relationship == domain.RelationshipSelf {
		return false
	}
	changeEmpty := result.AuthorizationChange == (ports.AuthorizationChange{})
	switch operation {
	case FollowOperation:
		if target.Relationship == domain.RelationshipRequested {
			return validOpaqueID(result.RequestID) && changeEmpty
		}
		if target.Relationship != domain.RelationshipFollowing || result.RequestID != "" {
			return false
		}
		if !result.Changed {
			return changeEmpty
		}
		return validFollowerAuthorizationChange(result.AuthorizationChange, target.ID, actor, actor, ports.AuthorizationTouch)
	case CancelFollowRequestOperation:
		return target.Relationship == domain.RelationshipNone && validOpaqueID(result.RequestID) && changeEmpty
	case UnfollowOperation:
		if target.Relationship != domain.RelationshipNone || result.RequestID != "" {
			return false
		}
		if !result.Changed {
			return changeEmpty
		}
		return validFollowerAuthorizationChange(result.AuthorizationChange, target.ID, actor, actor, ports.AuthorizationDelete)
	default:
		return false
	}
}

func validReviewResult(result ReviewResult, actor, requestID string, decision FollowRequestDecision, now time.Time) bool {
	if result.Decision != decision || !result.Request.Valid() || result.Request.ID != requestID ||
		result.Request.RecipientUserID != actor || result.Request.CreatedAt.After(now) {
		return false
	}
	if decision == FollowRequestRejected {
		return result.AuthorizationChange == (ports.AuthorizationChange{})
	}
	return validFollowerAuthorizationChange(
		result.AuthorizationChange, actor, result.Request.Requester.ID, actor, ports.AuthorizationTouch,
	)
}

func validFollowerAuthorizationChange(change ports.AuthorizationChange, target, follower, actor string, operation ports.AuthorizationOperation) bool {
	return change.ID != "" && change.ResourceType == "user" && change.ResourceID == target && change.Relation == "follower" &&
		change.SubjectType == "user" && change.SubjectID == follower && change.OwnerUserID == target &&
		change.ActorUserID == actor && change.Operation == operation
}

func sameAuthorizationChange(left, right ports.AuthorizationChange) bool {
	return left.ID == right.ID && left.ResourceType == right.ResourceType && left.ResourceID == right.ResourceID &&
		left.Relation == right.Relation && left.SubjectType == right.SubjectType && left.SubjectID == right.SubjectID &&
		left.OwnerUserID == right.OwnerUserID && left.ActorUserID == right.ActorUserID && left.Operation == right.Operation
}

func (service *Service) reconcileAuthorizationChange(ctx context.Context, expected ports.AuthorizationChange, replayed bool) error {
	change := expected
	if replayed {
		claimed, err := service.AuthorizationOutbox.ClaimAuthorizationChange(
			ctx, expected.ID, service.AuthorizationWorker, service.AuthorizationLease,
		)
		if errors.Is(err, ports.ErrNotFound) {
			state, statusErr := service.AuthorizationStatus.AuthorizationChangeState(ctx, expected.ID)
			if statusErr != nil {
				return statusErr
			}
			switch state {
			case AuthorizationChangeCompleted:
				return nil
			case AuthorizationChangePending, AuthorizationChangeLocked:
				return ports.ErrAuthorizationPending
			case AuthorizationChangeDeadLettered:
				return ports.ErrAuthorizationDeadLettered
			default:
				return errInvalidDependencies
			}
		}
		if err != nil {
			return err
		}
		if !sameAuthorizationChange(claimed, expected) {
			return errInvalidDependencies
		}
		change = claimed
	}

	var relationshipErr error
	err := service.AuthorizationSerializer.WithinResource(ctx, change.ResourceType, change.ResourceID, func(locked context.Context) error {
		if err := service.AuthorizationOutbox.RenewAuthorizationChange(locked, change.ID, service.AuthorizationWorker, service.AuthorizationLease); err != nil {
			return err
		}
		switch change.Operation {
		case ports.AuthorizationTouch:
			relationshipErr = service.Authorizer.WriteRelationship(locked, change.ResourceType, change.ResourceID, change.Relation, change.SubjectType, change.SubjectID)
		case ports.AuthorizationDelete:
			relationshipErr = service.Authorizer.DeleteRelationship(locked, change.ResourceType, change.ResourceID, change.Relation, change.SubjectType, change.SubjectID)
		default:
			return errInvalidDependencies
		}
		if relationshipErr != nil {
			auditErr := service.Audits.AppendAuditEvent(locked, shared.NewAuditEvent(
				locked, service.Clock, change.OwnerUserID, change.ActorUserID,
				audit.AuthorizationFailed, change.ResourceType, change.ResourceID, audit.Failed,
			))
			return errors.Join(relationshipErr, auditErr)
		}
		event := shared.NewAuditEvent(
			locked, service.Clock, change.OwnerUserID, change.ActorUserID,
			audit.AuthorizationApplied, change.ResourceType, change.ResourceID, audit.Succeeded,
		)
		return service.AuthorizationOutbox.CompleteAuthorizationChangeWithAudit(locked, change.ID, service.AuthorizationWorker, event)
	})
	if relationshipErr != nil {
		_, failErr := service.AuthorizationOutbox.FailAuthorizationChange(ctx, change.ID, service.AuthorizationWorker, 5, "dependency_failure")
		return errors.Join(err, failErr)
	}
	return err
}

func validFollowRequestPage(page FollowRequestPage, viewer string, request ports.PageRequest) bool {
	if len(page.Requests) > request.Limit || (page.HasMore && len(page.Requests) == 0) {
		return false
	}
	seen := make(map[string]struct{}, len(page.Requests))
	for index, item := range page.Requests {
		if !item.Valid() || item.RecipientUserID != viewer || item.CreatedAt.After(request.Snapshot) {
			return false
		}
		if request.AfterID != "" && (item.CreatedAt.After(request.AfterCreated) ||
			(item.CreatedAt.Equal(request.AfterCreated) && item.ID >= request.AfterID)) {
			return false
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return false
		}
		seen[item.ID] = struct{}{}
		if index > 0 {
			previous := page.Requests[index-1]
			if item.CreatedAt.After(previous.CreatedAt) || (item.CreatedAt.Equal(previous.CreatedAt) && item.ID >= previous.ID) {
				return false
			}
		}
	}
	return true
}

func relationshipIdempotency(actor, operation, key, target string) ports.Idempotency {
	digest := sha256.Sum256([]byte(operation + "\x00" + target))
	return ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: digest[:]}
}

func validRelationshipIdempotencyKey(value string) bool {
	if len(value) < 16 || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for index := range len(value) {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func validOpaqueID(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value
}

type followRequestCursorClaims struct {
	Version      int       `json:"v"`
	Viewer       string    `json:"viewer"`
	Snapshot     time.Time `json:"snapshot"`
	AfterCreated time.Time `json:"afterCreated"`
	AfterID      string    `json:"afterId"`
}

func encodeFollowRequestCursor(key []byte, claims followRequestCursorClaims) (string, error) {
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(append(body, mac.Sum(nil)...)), nil
}

func decodeFollowRequestCursor(key []byte, value string) (followRequestCursorClaims, error) {
	var claims followRequestCursorClaims
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) <= sha256.Size || len(value) > 4096 {
		return claims, ports.ErrInvalidArgument
	}
	body, signature := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(body, &claims) != nil ||
		claims.Version != 1 || claims.Viewer == "" || claims.Snapshot.IsZero() || claims.Snapshot.Location() != time.UTC ||
		claims.AfterCreated.IsZero() || claims.AfterCreated.Location() != time.UTC || claims.AfterCreated.After(claims.Snapshot) || !validOpaqueID(claims.AfterID) {
		return followRequestCursorClaims{}, ports.ErrInvalidArgument
	}
	canonical, err := json.Marshal(claims)
	if err != nil || !hmac.Equal(body, canonical) {
		return followRequestCursorClaims{}, ports.ErrInvalidArgument
	}
	return claims, nil
}
