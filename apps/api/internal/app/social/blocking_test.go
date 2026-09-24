package social

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledBlocks struct {
	review          BlockReview
	result          BlockMutationResult
	page            BlockedAccountPage
	err             error
	actor, username string
	command         BlockCommand
	pageRequest     ports.PageRequest
	replayExpired   bool
}

func (r *controlledBlocks) Review(_ context.Context, actor, username string) (BlockReview, error) {
	r.actor, r.username = actor, username
	return r.review, r.err
}
func (r *controlledBlocks) Block(_ context.Context, command BlockCommand) (BlockMutationResult, error) {
	r.command = command
	if !command.OccurredAt.Before(command.ReviewExpiresAt) {
		if r.replayExpired {
			result := r.result
			result.Replayed = true
			return result, r.err
		}
		return BlockMutationResult{}, ErrBlockReviewRequired
	}
	return r.result, r.err
}

func TestBlockExactReplayRemainsAvailableAfterReviewExpiry(t *testing.T) {
	repository := &controlledBlocks{review: BlockReview{Target: blockTarget("target", "alice")}, result: BlockMutationResult{Target: blockTarget("target", "alice"), Blocked: true}, replayExpired: true}
	service := blockingService(repository)
	review, err := service.ReviewBlock(context.Background(), "Bearer session", "alice")
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = fixedClock{now: review.Acknowledgement.ExpiresAt.Add(time.Second)}
	result, err := service.BlockUser(context.Background(), "Bearer session", "alice", "block-request-0001", review.Acknowledgement)
	if err != nil || !result.Replayed || !result.Blocked {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
func (r *controlledBlocks) ListAuthored(_ context.Context, actor string, page ports.PageRequest) (BlockedAccountPage, error) {
	r.actor, r.pageRequest = actor, page
	return r.page, r.err
}
func (r *controlledBlocks) Unblock(_ context.Context, command BlockCommand) (BlockMutationResult, error) {
	r.command = command
	return r.result, r.err
}

func blockingService(repository *controlledBlocks) *Service {
	service := relationshipService(&controlledRelationships{}, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
	service.Blocks = repository
	return service
}

func blockTarget(id, username string) domain.BlockTarget {
	return domain.BlockTarget{UserID: id, Username: username, DisplayName: "Alice"}
}

func TestBlockReviewReturnsOnlyTargetAndCurrentSharedPaths(t *testing.T) {
	want := BlockReview{Target: blockTarget("target", "alice"), SharedPaths: []domain.SharedPath{{ID: "path-1", Name: "Piano"}}}
	repository := &controlledBlocks{review: want}
	service := blockingService(repository)
	got, err := service.ReviewBlock(context.Background(), "Bearer session", "ALICE")
	if err != nil || !reflect.DeepEqual(got.Target, want.Target) || !reflect.DeepEqual(got.SharedPaths, want.SharedPaths) || got.Acknowledgement.Version != 1 || got.Acknowledgement.Token == "" || !got.Acknowledgement.ExpiresAt.After(service.Clock.Now()) || repository.actor != "viewer" || repository.username != "alice" {
		t.Fatalf("review=%+v err=%v repository=%+v", got, err, repository)
	}
}

func TestBlockIsAtomicIdempotentAndReconcilesBothFollowDeletes(t *testing.T) {
	changes := []ports.AuthorizationChange{
		followerAuthorization("change-1", "target", "viewer", "viewer", ports.AuthorizationDelete),
		followerAuthorization("change-2", "viewer", "target", "viewer", ports.AuthorizationDelete),
	}
	repository := &controlledBlocks{result: BlockMutationResult{Target: blockTarget("target", "alice"), Blocked: true, Changed: true, AuthorizationChanges: changes}}
	service := blockingService(repository)
	var deleted []ports.AuthorizationChange
	service.Authorizer = controlledRelationshipAuthorizer{deletes: &deleted}
	reviewRepository := &controlledBlocks{review: BlockReview{Target: blockTarget("target", "alice"), SharedPaths: []domain.SharedPath{{ID: "path-1", Name: "Piano"}}}}
	reviewService := blockingService(reviewRepository)
	review, err := reviewService.ReviewBlock(context.Background(), "Bearer session", "ALICE")
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = reviewService.Clock
	service.CursorSigningKey = reviewService.CursorSigningKey
	result, err := service.BlockUser(context.Background(), "Bearer session", "ALICE", "block-request-0001", review.Acknowledgement)
	if err != nil || result.Target.UserID != "target" || !result.Blocked || len(deleted) != 2 {
		t.Fatalf("result=%+v err=%v deletes=%+v", result, err, deleted)
	}
	command := repository.command
	if command.ActorUserID != "viewer" || command.TargetUsername != "alice" || command.TargetUserID != "target" || !reflect.DeepEqual(command.ExpectedSharedPathIDs, []string{"path-1"}) || command.Idempotency.Operation != BlockOperation || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("command=%+v", command)
	}
	if !command.Audit.Valid() || command.Audit.Action != audit.ResourceCreated || command.Audit.TargetType != "user_block" || command.Audit.TargetID != "alice" {
		t.Fatalf("audit=%+v", command.Audit)
	}
}

func TestBlockRequiresCurrentActorTargetAndSharedPathAcknowledgement(t *testing.T) {
	repository := &controlledBlocks{review: BlockReview{Target: blockTarget("target", "alice"), SharedPaths: []domain.SharedPath{{ID: "path-1", Name: "Piano"}}}, result: BlockMutationResult{Target: blockTarget("target", "alice"), Blocked: true}}
	service := blockingService(repository)
	review, err := service.ReviewBlock(context.Background(), "Bearer session", "alice")
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name            string
		acknowledgement BlockReviewAcknowledgement
		username        string
	}{
		{name: "missing", username: "alice"},
		{name: "malformed", username: "alice", acknowledgement: BlockReviewAcknowledgement{Version: 1, Token: "not-signed"}},
		{name: "different target", username: "bob", acknowledgement: review.Acknowledgement},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository.command = BlockCommand{}
			_, err := service.BlockUser(context.Background(), "Bearer session", test.username, "block-request-0001", test.acknowledgement)
			if !errors.Is(err, ErrBlockReviewRequired) || !reflect.DeepEqual(repository.command, BlockCommand{}) {
				t.Fatalf("err=%v command=%+v", err, repository.command)
			}
		})
	}
	service.Auth = controlledAuth{principal: ports.Principal{UserID: "other-viewer", Scopes: []string{"api:user"}}}
	if _, err := service.BlockUser(context.Background(), "Bearer session", "alice", "block-request-0001", review.Acknowledgement); !errors.Is(err, ErrBlockReviewRequired) {
		t.Fatalf("cross-actor acknowledgement err=%v", err)
	}
	service.Auth = controlledAuth{principal: ports.Principal{UserID: "viewer", Scopes: []string{"api:user"}}}
	service.Clock = fixedClock{now: review.Acknowledgement.ExpiresAt}
	if _, err := service.BlockUser(context.Background(), "Bearer session", "alice", "block-request-0001", review.Acknowledgement); !errors.Is(err, ErrBlockReviewRequired) {
		t.Fatalf("expired acknowledgement err=%v", err)
	}
}

func TestBlockedAccountsPageIsViewerBoundAndUnblockDoesNotRestoreFollows(t *testing.T) {
	now := time.Date(2026, 7, 27, 11, 0, 0, 0, time.UTC)
	account := domain.BlockedAccount{Target: blockTarget("target", "alice"), BlockedAt: now}
	repository := &controlledBlocks{page: BlockedAccountPage{Accounts: []domain.BlockedAccount{account}, HasMore: true}}
	service := blockingService(repository)
	accounts, cursor, err := service.ListBlockedAccounts(context.Background(), "Bearer session", "", 1)
	if err != nil || len(accounts) != 1 || cursor == "" || repository.actor != "viewer" {
		t.Fatalf("accounts=%+v cursor=%q err=%v", accounts, cursor, err)
	}
	repository.result = BlockMutationResult{Target: account.Target, Blocked: false, Changed: true}
	result, err := service.UnblockUser(context.Background(), "Bearer session", "target", "unblock-request-1")
	if err != nil || result.Blocked || len(result.AuthorizationChanges) != 0 || repository.command.TargetUserID != "target" || repository.command.Idempotency.Operation != UnblockOperation {
		t.Fatalf("result=%+v err=%v command=%+v", result, err, repository.command)
	}
}

func TestBlockBoundariesFailClosedBeforeMutation(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*Service, *controlledBlocks)
		want      error
	}{
		{name: "unauthenticated", configure: func(s *Service, _ *controlledBlocks) { s.Auth = controlledAuth{err: ports.ErrInvalidCredential} }, want: ports.ErrInvalidCredential},
		{name: "self or hidden", configure: func(_ *Service, r *controlledBlocks) { r.err = ports.ErrNotFound }, want: ports.ErrNotFound},
		{name: "rate limited", configure: func(s *Service, _ *controlledBlocks) {
			s.RelationshipRateLimiter = &controlledRelationshipLimiter{allowed: false}
		}, want: platformapp.ErrRateLimited},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledBlocks{}
			service := blockingService(repository)
			acknowledgement, err := service.newBlockReviewAcknowledgement("viewer", BlockReview{Target: blockTarget("target", "alice")})
			if err != nil {
				t.Fatal(err)
			}
			test.configure(service, repository)
			result, err := service.BlockUser(context.Background(), "Bearer session", "alice", "block-request-0001", acknowledgement)
			if !errors.Is(err, test.want) || !reflect.DeepEqual(result, BlockMutationResult{}) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}
