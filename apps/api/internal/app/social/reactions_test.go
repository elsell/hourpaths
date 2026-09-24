package social

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledReactions struct {
	target                        ReactionTarget
	result                        ReactionMutationResult
	err, resolveErr, mutationErr  error
	resolvedViewer, resolvedEvent string
	set, removed                  *ReactionCommand
}

func (repository *controlledReactions) ResolvePracticeReactionTarget(_ context.Context, viewer, event string) (ReactionTarget, error) {
	repository.resolvedViewer, repository.resolvedEvent = viewer, event
	err := repository.resolveErr
	if err == nil {
		err = repository.err
	}
	return repository.target, err
}
func (repository *controlledReactions) SetPracticeReaction(_ context.Context, command ReactionCommand) (ReactionMutationResult, error) {
	repository.set = &command
	err := repository.mutationErr
	if err == nil {
		err = repository.err
	}
	return repository.result, err
}
func (repository *controlledReactions) RemovePracticeReaction(_ context.Context, command ReactionCommand) (ReactionMutationResult, error) {
	repository.removed = &command
	err := repository.mutationErr
	if err == nil {
		err = repository.err
	}
	return repository.result, err
}

func reactionService(repository *controlledReactions) (*Service, *feedAuthorizer) {
	service := testService(&controlledProfiles{}, &controlledAudits{})
	authorizer := &feedAuthorizer{allowed: map[string]bool{"path": true}}
	service.Reactions, service.Authorizer = repository, authorizer
	service.ReactionRateLimiter = &controlledRelationshipLimiter{allowed: true}
	return service, authorizer
}

func TestSetAchievementReactionAuthorizesAndReturnsAuthoritativeProjection(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	want := domain.ReactionSummary{Counts: domain.ReactionCounts{Heart: 3, Fire: 1}, ViewerReaction: domain.ReactionFire}
	repository := &controlledReactions{
		target: ReactionTarget{EventID: "achievement:goal", PathID: "path", OwnerUserID: "owner"},
		result: ReactionMutationResult{Summary: want},
	}
	service, authorizer := reactionService(repository)
	service.Clock = fixedClock{now: now}
	got, err := service.SetPracticeReaction(context.Background(), "Bearer session", "achievement:goal", domain.ReactionFire, "0123456789abcdef")
	if err != nil || got != want {
		t.Fatalf("summary=%+v err=%v", got, err)
	}
	if repository.set == nil || repository.removed != nil || repository.set.ActorUserID != "viewer" || repository.set.Reaction != domain.ReactionFire || repository.set.NotificationEligibleAt != now.Add(5*time.Second) {
		t.Fatalf("command=%+v", repository.set)
	}
	if repository.set.Idempotency.PrincipalID != "viewer" || repository.set.Idempotency.Operation != SetPracticeReactionOperation || len(repository.set.Idempotency.RequestHash) != 32 {
		t.Fatalf("idempotency=%+v", repository.set.Idempotency)
	}
	if len(authorizer.checks) != 1 || authorizer.checks[0] != "path:path:view:viewer" {
		t.Fatalf("checks=%v", authorizer.checks)
	}
}

func TestRemovePracticeReactionIsIdempotentAndCancelsOwnerIntent(t *testing.T) {
	repository := &controlledReactions{
		target: ReactionTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"},
		result: ReactionMutationResult{Summary: domain.ReactionSummary{}},
	}
	service, _ := reactionService(repository)
	got, err := service.RemovePracticeReaction(context.Background(), "Bearer session", "practice:activity", "fedcba9876543210")
	if err != nil || !got.Valid() || repository.removed == nil || repository.removed.Reaction != "" || repository.removed.Idempotency.Operation != RemovePracticeReactionOperation {
		t.Fatalf("summary=%+v command=%+v err=%v", got, repository.removed, err)
	}
}

func TestPracticeReactionFailsOpaqueForMissingBlockedOrInaccessibleEvent(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*controlledReactions, *feedAuthorizer)
		wantCall  bool
	}{
		{name: "missing", configure: func(repository *controlledReactions, _ *feedAuthorizer) { repository.resolveErr = ports.ErrNotFound }},
		{name: "current policy denies", configure: func(_ *controlledReactions, authorizer *feedAuthorizer) { authorizer.allowed["path"] = false }},
		{name: "access changed before transaction", configure: func(repository *controlledReactions, _ *feedAuthorizer) { repository.mutationErr = ports.ErrNotFound }, wantCall: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledReactions{target: ReactionTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"}}
			service, authorizer := reactionService(repository)
			audits := &controlledAudits{}
			service.Audits = audits
			test.configure(repository, authorizer)
			got, err := service.SetPracticeReaction(context.Background(), "Bearer session", "practice:activity", domain.ReactionHeart, "0123456789abcdef")
			if !errors.Is(err, ports.ErrNotFound) || got != (domain.ReactionSummary{}) || (repository.set != nil) != test.wantCall {
				t.Fatalf("summary=%+v err=%v command=%+v", got, err, repository.set)
			}
			if len(audits.events) != 1 || audits.events[0].Action != audit.ResourceAccessDenied || audits.events[0].TargetType != "practice_reaction" || audits.events[0].TargetID != "hidden" || audits.events[0].Outcome != audit.Denied {
				t.Fatalf("audits=%+v", audits.events)
			}
		})
	}
}

func TestPracticeReactionDenialFailsClosedWhenAuditCannotPersist(t *testing.T) {
	auditFailure := errors.New("audit unavailable")
	for _, test := range []struct {
		name      string
		configure func(*controlledReactions, *feedAuthorizer)
	}{
		{name: "missing", configure: func(repository *controlledReactions, _ *feedAuthorizer) { repository.resolveErr = ports.ErrNotFound }},
		{name: "current policy denies", configure: func(_ *controlledReactions, authorizer *feedAuthorizer) { authorizer.allowed["path"] = false }},
		{name: "access changed before transaction", configure: func(repository *controlledReactions, _ *feedAuthorizer) { repository.mutationErr = ports.ErrNotFound }},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledReactions{target: ReactionTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"}}
			service, authorizer := reactionService(repository)
			audits := &controlledAudits{err: auditFailure}
			service.Audits = audits
			test.configure(repository, authorizer)
			got, err := service.SetPracticeReaction(context.Background(), "Bearer session", "practice:activity", domain.ReactionHeart, "0123456789abcdef")
			if !errors.Is(err, auditFailure) || got != (domain.ReactionSummary{}) || len(audits.events) != 1 {
				t.Fatalf("summary=%+v err=%v audits=%+v", got, err, audits.events)
			}
			if test.name != "access changed before transaction" && repository.set != nil {
				t.Fatalf("denial reached mutation: %+v", repository.set)
			}
		})
	}
}

func TestPracticeReactionRequiresAuditPersistenceBeforeRepositoryAccess(t *testing.T) {
	repository := &controlledReactions{target: ReactionTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"}}
	service, _ := reactionService(repository)
	service.Audits = nil
	got, err := service.SetPracticeReaction(context.Background(), "Bearer session", "practice:activity", domain.ReactionHeart, "0123456789abcdef")
	if !errors.Is(err, errInvalidDependencies) || got != (domain.ReactionSummary{}) || repository.resolvedEvent != "" || repository.set != nil {
		t.Fatalf("summary=%+v err=%v repository=%+v", got, err, repository)
	}
}

func TestPracticeReactionRejectsInvalidInputsAndRateLimitsBeforeMutation(t *testing.T) {
	repository := &controlledReactions{target: ReactionTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"}}
	service, _ := reactionService(repository)
	if _, err := service.SetPracticeReaction(context.Background(), "Bearer session", "practice:activity", "negative", "0123456789abcdef"); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("invalid reaction error=%v", err)
	}
	service.ReactionRateLimiter = &controlledRelationshipLimiter{}
	if _, err := service.SetPracticeReaction(context.Background(), "Bearer session", "practice:activity", domain.ReactionHeart, "0123456789abcdef"); !errors.Is(err, platformapp.ErrRateLimited) {
		t.Fatalf("rate limit error=%v", err)
	}
	if repository.set != nil {
		t.Fatalf("mutated while limited: %+v", repository.set)
	}
}
