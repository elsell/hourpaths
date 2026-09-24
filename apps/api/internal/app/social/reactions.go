package social

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	SetPracticeReactionOperation    = "social.practice_reaction.set"
	RemovePracticeReactionOperation = "social.practice_reaction.remove"
	reactionNotificationGrace       = 5 * time.Second
)

type ReactionTarget struct{ EventID, PathID, OwnerUserID string }

type ReactionCommand struct {
	ActorUserID                        string
	Target                             ReactionTarget
	Reaction                           domain.Reaction
	OccurredAt, NotificationEligibleAt time.Time
	Idempotency                        ports.Idempotency
	Audit                              audit.Event
}

type ReactionMutationResult struct {
	Summary  domain.ReactionSummary
	Replayed bool
}

type ReactionRepository interface {
	ResolvePracticeReactionTarget(context.Context, string, string) (ReactionTarget, error)
	SetPracticeReaction(context.Context, ReactionCommand) (ReactionMutationResult, error)
	RemovePracticeReaction(context.Context, ReactionCommand) (ReactionMutationResult, error)
}

func (service *Service) SetPracticeReaction(ctx context.Context, authorization, eventID string, reaction domain.Reaction, idempotencyKey string) (domain.ReactionSummary, error) {
	if !reaction.Valid() {
		return domain.ReactionSummary{}, ports.ErrInvalidArgument
	}
	return service.mutatePracticeReaction(ctx, authorization, eventID, reaction, idempotencyKey, SetPracticeReactionOperation)
}

func (service *Service) RemovePracticeReaction(ctx context.Context, authorization, eventID, idempotencyKey string) (domain.ReactionSummary, error) {
	return service.mutatePracticeReaction(ctx, authorization, eventID, "", idempotencyKey, RemovePracticeReactionOperation)
}

func (service *Service) mutatePracticeReaction(ctx context.Context, authorization, eventID string, reaction domain.Reaction, key, operation string) (domain.ReactionSummary, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return domain.ReactionSummary{}, err
	}
	if !validOpaqueID(eventID) || !validRelationshipIdempotencyKey(key) {
		return domain.ReactionSummary{}, ports.ErrInvalidArgument
	}
	if service.Reactions == nil || service.Authorizer == nil || service.ReactionRateLimiter == nil || service.Audits == nil || service.Clock == nil {
		return domain.ReactionSummary{}, errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return domain.ReactionSummary{}, errInvalidDependencies
	}
	if !service.ReactionRateLimiter.Allow(principal.UserID, "practice_event:"+eventID, now) {
		return domain.ReactionSummary{}, platformapp.ErrRateLimited
	}
	target, err := service.Reactions.ResolvePracticeReactionTarget(ctx, principal.UserID, eventID)
	if err != nil {
		return domain.ReactionSummary{}, service.reactionError(ctx, principal.UserID, err, now)
	}
	if !validReactionTarget(target, eventID) {
		return domain.ReactionSummary{}, errInvalidDependencies
	}
	allowed, err := service.Authorizer.Check(ctx, "path", target.PathID, "view", principal.UserID)
	if err != nil {
		return domain.ReactionSummary{}, err
	}
	if !allowed {
		return domain.ReactionSummary{}, service.reactionError(ctx, principal.UserID, ports.ErrNotFound, now)
	}
	action := audit.ResourceUpdated
	if operation == RemovePracticeReactionOperation {
		action = audit.ResourceDeleted
	}
	command := ReactionCommand{ActorUserID: principal.UserID, Target: target, Reaction: reaction, OccurredAt: now, NotificationEligibleAt: now.Add(reactionNotificationGrace)}
	command.Idempotency = reactionIdempotency(principal.UserID, operation, key, eventID, reaction)
	command.Audit = shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, action, "practice_reaction", eventID, audit.Succeeded)
	command.Audit.OccurredAt = now
	var result ReactionMutationResult
	if operation == SetPracticeReactionOperation {
		result, err = service.Reactions.SetPracticeReaction(ctx, command)
	} else {
		result, err = service.Reactions.RemovePracticeReaction(ctx, command)
	}
	if err != nil {
		return domain.ReactionSummary{}, service.reactionError(ctx, principal.UserID, err, now)
	}
	if !result.Summary.Valid() {
		return domain.ReactionSummary{}, errInvalidDependencies
	}
	return result.Summary, nil
}

func validReactionTarget(target ReactionTarget, eventID string) bool {
	return target.EventID == eventID && validOpaqueID(target.PathID) && validOpaqueID(target.OwnerUserID)
}

func (service *Service) reactionError(ctx context.Context, actor string, repositoryError error, now time.Time) error {
	if !errors.Is(repositoryError, ports.ErrNotFound) {
		return repositoryError
	}
	denial := shared.NewAuditEvent(ctx, service.Clock, actor, actor, audit.ResourceAccessDenied, "practice_reaction", "hidden", audit.Denied)
	denial.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, denial); err != nil {
		return err
	}
	return ports.ErrNotFound
}

func reactionIdempotency(actor, operation, key, event string, reaction domain.Reaction) ports.Idempotency {
	digest := sha256.Sum256([]byte(operation + "\x00" + event + "\x00" + string(reaction)))
	return ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: digest[:]}
}
