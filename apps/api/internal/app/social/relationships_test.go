package social

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledRelationships struct {
	result       RelationshipResult
	review       ReviewResult
	page         FollowRequestPage
	err          error
	operation    string
	command      RelationshipCommand
	listViewer   string
	listPage     ports.PageRequest
	mutationCall int
}

func (repository *controlledRelationships) Follow(_ context.Context, command RelationshipCommand) (RelationshipResult, error) {
	repository.capture(FollowOperation, command)
	return repository.result, repository.err
}

func (repository *controlledRelationships) CancelRequest(_ context.Context, command RelationshipCommand) (RelationshipResult, error) {
	repository.capture(CancelFollowRequestOperation, command)
	return repository.result, repository.err
}

func (repository *controlledRelationships) Unfollow(_ context.Context, command RelationshipCommand) (RelationshipResult, error) {
	repository.capture(UnfollowOperation, command)
	return repository.result, repository.err
}

func (repository *controlledRelationships) AcceptRequest(_ context.Context, command RelationshipCommand) (ReviewResult, error) {
	repository.capture(AcceptFollowRequestOperation, command)
	return repository.review, repository.err
}

func (repository *controlledRelationships) RejectRequest(_ context.Context, command RelationshipCommand) (ReviewResult, error) {
	repository.capture(RejectFollowRequestOperation, command)
	return repository.review, repository.err
}

func (repository *controlledRelationships) ListIncoming(_ context.Context, viewer string, page ports.PageRequest) (FollowRequestPage, error) {
	repository.listViewer, repository.listPage = viewer, page
	return repository.page, repository.err
}

func (repository *controlledRelationships) capture(operation string, command RelationshipCommand) {
	repository.operation, repository.command = operation, command
	repository.mutationCall++
}

type controlledRelationshipLimiter struct {
	allowed       bool
	actor, target string
	now           time.Time
	calls         int
}

type controlledRelationshipAuthorizer struct {
	writes, deletes *[]ports.AuthorizationChange
	err             error
}

func (authorizer controlledRelationshipAuthorizer) WriteRelationship(_ context.Context, resourceType, resourceID, relation, subjectType, subjectID string) error {
	if authorizer.writes != nil {
		*authorizer.writes = append(*authorizer.writes, ports.AuthorizationChange{ResourceType: resourceType, ResourceID: resourceID, Relation: relation, SubjectType: subjectType, SubjectID: subjectID})
	}
	return authorizer.err
}

func (authorizer controlledRelationshipAuthorizer) DeleteRelationship(_ context.Context, resourceType, resourceID, relation, subjectType, subjectID string) error {
	if authorizer.deletes != nil {
		*authorizer.deletes = append(*authorizer.deletes, ports.AuthorizationChange{ResourceType: resourceType, ResourceID: resourceID, Relation: relation, SubjectType: subjectType, SubjectID: subjectID})
	}
	return authorizer.err
}

func (controlledRelationshipAuthorizer) Check(context.Context, string, string, string, string) (bool, error) {
	return true, nil
}

type controlledRelationshipOutbox struct {
	claimed                         ports.AuthorizationChange
	claimErr, renewErr, completeErr error
	renewed, completed, failed      *bool
}

func (controlledRelationshipOutbox) ClaimAuthorizationChanges(context.Context, string, time.Duration, int) ([]ports.AuthorizationChange, error) {
	return nil, nil
}
func (outbox controlledRelationshipOutbox) ClaimAuthorizationChange(context.Context, string, string, time.Duration) (ports.AuthorizationChange, error) {
	return outbox.claimed, outbox.claimErr
}
func (controlledRelationshipOutbox) ClaimAuthorizationChangeForResource(context.Context, string, string, string, time.Duration) (ports.AuthorizationChange, error) {
	return ports.AuthorizationChange{}, ports.ErrNotFound
}
func (outbox controlledRelationshipOutbox) RenewAuthorizationChange(context.Context, string, string, time.Duration) error {
	if outbox.renewed != nil {
		*outbox.renewed = true
	}
	return outbox.renewErr
}
func (outbox controlledRelationshipOutbox) CompleteAuthorizationChangeWithAudit(context.Context, string, string, audit.Event) error {
	if outbox.completed != nil {
		*outbox.completed = true
	}
	return outbox.completeErr
}
func (outbox controlledRelationshipOutbox) FailAuthorizationChange(context.Context, string, string, int, string) (bool, error) {
	if outbox.failed != nil {
		*outbox.failed = true
	}
	return false, nil
}
func (controlledRelationshipOutbox) ListAuthorizationDeadLetters(context.Context, string, ports.PageRequest) (ports.AuthorizationDeadLetterPage, error) {
	return ports.AuthorizationDeadLetterPage{}, nil
}
func (controlledRelationshipOutbox) RequeueAuthorizationDeadLetter(context.Context, string, string, string, time.Duration, audit.Event) (ports.AuthorizationChange, error) {
	return ports.AuthorizationChange{}, ports.ErrNotFound
}

type controlledRelationshipStatus struct {
	state AuthorizationChangeState
	err   error
}

func (status controlledRelationshipStatus) AuthorizationChangeState(context.Context, string) (AuthorizationChangeState, error) {
	return status.state, status.err
}

type controlledRelationshipSerializer struct{ err error }

func (serializer controlledRelationshipSerializer) WithinResource(ctx context.Context, _, _ string, operation func(context.Context) error) error {
	if serializer.err != nil {
		return serializer.err
	}
	return operation(ctx)
}

func (limiter *controlledRelationshipLimiter) Allow(actor, target string, now time.Time) bool {
	limiter.actor, limiter.target, limiter.now = actor, target, now
	limiter.calls++
	return limiter.allowed
}

func relationshipProfile(id, username string, state domain.RelationshipState) domain.PublicProfile {
	return domain.PublicProfile{ID: id, Username: username, DisplayName: username, Relationship: state}
}

func relationshipService(repository *controlledRelationships, limiter *controlledRelationshipLimiter, audits *controlledAudits) *Service {
	service := testService(&controlledProfiles{}, audits)
	service.Relationships = repository
	service.RelationshipRateLimiter = limiter
	service.Authorizer = controlledRelationshipAuthorizer{}
	service.AuthorizationOutbox = controlledRelationshipOutbox{}
	service.AuthorizationStatus = controlledRelationshipStatus{state: AuthorizationChangeCompleted}
	service.AuthorizationSerializer = controlledRelationshipSerializer{}
	service.AuthorizationWorker = "social-api"
	service.AuthorizationLease = time.Minute
	return service
}

func followerAuthorization(id, target, follower, actor string, operation ports.AuthorizationOperation) ports.AuthorizationChange {
	return ports.AuthorizationChange{
		ID: id, ResourceType: "user", ResourceID: target, Relation: "follower", SubjectType: "user", SubjectID: follower,
		OwnerUserID: target, ActorUserID: actor, Operation: operation,
	}
}

func TestFollowCreatesExactlyOnePublicRelationshipOrPrivateRequest(t *testing.T) {
	for _, test := range []struct {
		name      string
		state     domain.RelationshipState
		requestID string
	}{
		{name: "public", state: domain.RelationshipFollowing},
		{name: "private", state: domain.RelationshipRequested, requestID: "follow-request-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			change := ports.AuthorizationChange{}
			if test.state == domain.RelationshipFollowing {
				change = followerAuthorization("change-1", "target-user", "viewer", "viewer", ports.AuthorizationTouch)
			}
			repository := &controlledRelationships{result: RelationshipResult{
				Target: relationshipProfile("target-user", "Alice", test.state), RequestID: test.requestID,
				Changed: true, AuthorizationChange: change,
			}}
			limiter, audits := &controlledRelationshipLimiter{allowed: true}, &controlledAudits{}
			service := relationshipService(repository, limiter, audits)
			var writes []ports.AuthorizationChange
			service.Authorizer = controlledRelationshipAuthorizer{writes: &writes}
			renewed, completed := false, false
			service.AuthorizationOutbox = controlledRelationshipOutbox{renewed: &renewed, completed: &completed}
			got, err := service.Follow(context.Background(), "Bearer session", "ALICE", "follow-request-0001")
			if err != nil || got != repository.result {
				t.Fatalf("Follow() = %+v, %v", got, err)
			}
			command := repository.command
			if repository.operation != FollowOperation || command.ActorUserID != "viewer" || command.TargetUsername != "alice" || command.RequestID != "" {
				t.Fatalf("command=%+v operation=%q", command, repository.operation)
			}
			if limiter.actor != "viewer" || limiter.target != "profile:alice" || limiter.calls != 1 {
				t.Fatalf("limiter=%+v", limiter)
			}
			if command.Idempotency.PrincipalID != "viewer" || command.Idempotency.Operation != FollowOperation || command.Idempotency.Key != "follow-request-0001" || len(command.Idempotency.RequestHash) != 32 {
				t.Fatalf("idempotency=%+v", command.Idempotency)
			}
			if !command.Audit.Valid() || command.Audit.Action != audit.ResourceCreated || command.Audit.TargetType != "profile_follow" || command.Audit.TargetID != "alice" {
				t.Fatalf("audit=%+v", command.Audit)
			}
			if (test.state == domain.RelationshipFollowing && len(writes) != 1) || (test.state == domain.RelationshipRequested && len(writes) != 0) {
				t.Fatalf("authorization writes=%+v", writes)
			}
			if test.state == domain.RelationshipFollowing && (!renewed || !completed) {
				t.Fatalf("authorization reconciliation renewed=%v completed=%v", renewed, completed)
			}
			if test.state == domain.RelationshipRequested && (renewed || completed) {
				t.Fatalf("pending request touched authorization outbox renewed=%v completed=%v", renewed, completed)
			}
		})
	}
}

func TestTargetMutationsAreDistinctIdempotentCommandsWithStrictStates(t *testing.T) {
	for _, test := range []struct {
		name, operation string
		state           domain.RelationshipState
		requestID       string
		invoke          func(*Service) (RelationshipResult, error)
	}{
		{name: "cancel", operation: CancelFollowRequestOperation, state: domain.RelationshipNone, requestID: "request-1", invoke: func(service *Service) (RelationshipResult, error) {
			return service.CancelRequest(context.Background(), "Bearer session", "ALICE", "cancel-request-001")
		}},
		{name: "unfollow", operation: UnfollowOperation, state: domain.RelationshipNone, invoke: func(service *Service) (RelationshipResult, error) {
			return service.Unfollow(context.Background(), "Bearer session", "ALICE", "unfollow-key-0001")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			change := ports.AuthorizationChange{}
			if test.operation == UnfollowOperation {
				change = followerAuthorization("change-1", "target-user", "viewer", "viewer", ports.AuthorizationDelete)
			}
			repository := &controlledRelationships{result: RelationshipResult{
				Target: relationshipProfile("target-user", "alice", test.state), RequestID: test.requestID,
				Changed: true, AuthorizationChange: change,
			}}
			limiter := &controlledRelationshipLimiter{allowed: true}
			service := relationshipService(repository, limiter, &controlledAudits{})
			var deletes []ports.AuthorizationChange
			service.Authorizer = controlledRelationshipAuthorizer{deletes: &deletes}
			got, err := test.invoke(service)
			if err != nil || got != repository.result || repository.operation != test.operation {
				t.Fatalf("result=%+v err=%v operation=%q", got, err, repository.operation)
			}
			if repository.command.Idempotency.Operation != test.operation || len(repository.command.Idempotency.RequestHash) != 32 || !repository.command.Audit.Valid() {
				t.Fatalf("command=%+v", repository.command)
			}
			if (test.operation == UnfollowOperation && len(deletes) != 1) || (test.operation == CancelFollowRequestOperation && len(deletes) != 0) {
				t.Fatalf("authorization deletes=%+v", deletes)
			}
		})
	}
}

func TestAcceptAndRejectOnlyResolveOwnedIncomingRequest(t *testing.T) {
	created := time.Date(2026, 7, 27, 11, 0, 0, 0, time.UTC)
	request := domain.FollowRequest{
		ID: "request-1", RecipientUserID: "viewer", CreatedAt: created,
		Requester: relationshipProfile("sender", "alice", domain.RelationshipNone),
	}
	for _, test := range []struct {
		name, operation string
		decision        FollowRequestDecision
		invoke          func(*Service) (ReviewResult, error)
	}{
		{name: "accept", operation: AcceptFollowRequestOperation, decision: FollowRequestAccepted, invoke: func(service *Service) (ReviewResult, error) {
			return service.AcceptRequest(context.Background(), "Bearer session", "request-1", "accept-request-01")
		}},
		{name: "reject", operation: RejectFollowRequestOperation, decision: FollowRequestRejected, invoke: func(service *Service) (ReviewResult, error) {
			return service.RejectRequest(context.Background(), "Bearer session", "request-1", "reject-request-01")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			change := ports.AuthorizationChange{}
			if test.decision == FollowRequestAccepted {
				change = followerAuthorization("change-1", "viewer", "sender", "viewer", ports.AuthorizationTouch)
			}
			repository := &controlledRelationships{review: ReviewResult{Request: request, Decision: test.decision, AuthorizationChange: change}}
			limiter := &controlledRelationshipLimiter{allowed: true}
			service := relationshipService(repository, limiter, &controlledAudits{})
			got, err := test.invoke(service)
			if err != nil || got != repository.review || repository.command.RequestID != "request-1" || repository.operation != test.operation {
				t.Fatalf("review=%+v err=%v command=%+v", got, err, repository.command)
			}
			if limiter.target != "follow_request:request-1" || repository.command.TargetUsername != "" || repository.command.Audit.TargetID != "request-1" {
				t.Fatalf("limiter=%+v command=%+v", limiter, repository.command)
			}
		})
	}
}

func TestRelationshipMutationsRejectInvalidAuthKeyAndRateLimitBeforeRepository(t *testing.T) {
	for _, test := range []struct {
		name      string
		key       string
		configure func(*Service, *controlledRelationshipLimiter)
		want      error
	}{
		{name: "weak key", key: "short", want: ports.ErrInvalidArgument},
		{name: "wrong scope", key: "valid-request-key", configure: func(service *Service, _ *controlledRelationshipLimiter) {
			service.Auth = controlledAuth{principal: ports.Principal{UserID: "viewer", Scopes: []string{"api:onboarding"}}}
		}, want: platformapp.ErrUnauthenticated},
		{name: "limited target", key: "valid-request-key", configure: func(_ *Service, limiter *controlledRelationshipLimiter) {
			limiter.allowed = false
		}, want: platformapp.ErrRateLimited},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository, limiter := &controlledRelationships{}, &controlledRelationshipLimiter{allowed: true}
			service := relationshipService(repository, limiter, &controlledAudits{})
			if test.configure != nil {
				test.configure(service, limiter)
			}
			_, err := service.Follow(context.Background(), "Bearer session", "alice", test.key)
			if !errors.Is(err, test.want) || repository.mutationCall != 0 {
				t.Fatalf("err=%v calls=%d", err, repository.mutationCall)
			}
			if test.name == "weak key" && limiter.calls != 0 {
				t.Fatalf("invalid key reached limiter: %+v", limiter)
			}
		})
	}
}

func TestRelationshipMutationHashBindsOperationAndCanonicalTarget(t *testing.T) {
	repository := &controlledRelationships{result: RelationshipResult{Target: relationshipProfile("target", "alice", domain.RelationshipFollowing)}}
	service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
	_, _ = service.Follow(context.Background(), "Bearer session", "ALICE", "same-request-key1")
	followHash := append([]byte(nil), repository.command.Idempotency.RequestHash...)
	_, _ = service.Unfollow(context.Background(), "Bearer session", "alice", "same-request-key1")
	unfollowHash := append([]byte(nil), repository.command.Idempotency.RequestHash...)
	if bytes.Equal(followHash, unfollowHash) {
		t.Fatal("different relationship operations shared an idempotency request hash")
	}
}

func TestRelationshipAuthorizationReplayConvergesOrFailsClosed(t *testing.T) {
	change := followerAuthorization("change-1", "target", "viewer", "viewer", ports.AuthorizationTouch)
	for _, test := range []struct {
		name  string
		state AuthorizationChangeState
		want  error
	}{
		{name: "completed", state: AuthorizationChangeCompleted},
		{name: "pending", state: AuthorizationChangePending, want: ports.ErrAuthorizationPending},
		{name: "dead lettered", state: AuthorizationChangeDeadLettered, want: ports.ErrAuthorizationDeadLettered},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledRelationships{result: RelationshipResult{
				Target: relationshipProfile("target", "alice", domain.RelationshipFollowing), Changed: true,
				AuthorizationChange: change, Replayed: true,
			}}
			service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
			service.AuthorizationOutbox = controlledRelationshipOutbox{claimErr: ports.ErrNotFound}
			service.AuthorizationStatus = controlledRelationshipStatus{state: test.state}
			result, err := service.Follow(context.Background(), "Bearer session", "alice", "follow-request-0001")
			if !errors.Is(err, test.want) || (test.want != nil && result != (RelationshipResult{})) {
				t.Fatalf("result=%+v err=%v want=%v", result, err, test.want)
			}
		})
	}
}

func TestRelationshipAuthorizationReplayRejectsAnotherOutboxChange(t *testing.T) {
	expected := followerAuthorization("change-1", "target", "viewer", "viewer", ports.AuthorizationTouch)
	repository := &controlledRelationships{result: RelationshipResult{
		Target: relationshipProfile("target", "alice", domain.RelationshipFollowing), Changed: true,
		AuthorizationChange: expected, Replayed: true,
	}}
	service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
	claimed := expected
	claimed.SubjectID = "other"
	service.AuthorizationOutbox = controlledRelationshipOutbox{claimed: claimed}
	if result, err := service.Follow(context.Background(), "Bearer session", "alice", "follow-request-0001"); err == nil || result != (RelationshipResult{}) {
		t.Fatalf("mismatched outbox change leaked: %+v err=%v", result, err)
	}
}

func TestRelationshipAuthorizationFailureIsRecordedAndResultFailsClosed(t *testing.T) {
	change := followerAuthorization("change-1", "target", "viewer", "viewer", ports.AuthorizationTouch)
	repository := &controlledRelationships{result: RelationshipResult{
		Target: relationshipProfile("target", "alice", domain.RelationshipFollowing), Changed: true, AuthorizationChange: change,
	}}
	service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
	dependencyFailure := errors.New("authorization unavailable")
	failed := false
	service.Authorizer = controlledRelationshipAuthorizer{err: dependencyFailure}
	service.AuthorizationOutbox = controlledRelationshipOutbox{failed: &failed}
	if result, err := service.Follow(context.Background(), "Bearer session", "alice", "follow-request-0001"); !errors.Is(err, dependencyFailure) || result != (RelationshipResult{}) || !failed {
		t.Fatalf("result=%+v err=%v failed=%v", result, err, failed)
	}
}

func TestRelationshipMutationFailsClosedWhenAuthorizationReconciliationIsUnconfigured(t *testing.T) {
	for _, configure := range []func(*Service){
		func(service *Service) { service.Authorizer = nil },
		func(service *Service) { service.AuthorizationOutbox = nil },
		func(service *Service) { service.AuthorizationStatus = nil },
		func(service *Service) { service.AuthorizationSerializer = nil },
		func(service *Service) { service.AuthorizationWorker = "" },
		func(service *Service) { service.AuthorizationLease = 0 },
	} {
		repository := &controlledRelationships{}
		service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
		configure(service)
		if _, err := service.Follow(context.Background(), "Bearer session", "alice", "follow-request-0001"); err == nil || repository.mutationCall != 0 {
			t.Fatalf("unconfigured reconciliation reached repository: err=%v calls=%d", err, repository.mutationCall)
		}
	}
}

func TestListIncomingRequestsUsesViewerSnapshotCursorAndAudits(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 123456000, time.UTC)
	newer := domain.FollowRequest{ID: "request-2", RecipientUserID: "viewer", CreatedAt: now.Add(-time.Minute), Requester: relationshipProfile("sender-2", "bob", domain.RelationshipNone)}
	older := domain.FollowRequest{ID: "request-1", RecipientUserID: "viewer", CreatedAt: now.Add(-2 * time.Minute), Requester: relationshipProfile("sender-1", "alice", domain.RelationshipFollowing)}
	repository := &controlledRelationships{page: FollowRequestPage{Requests: []domain.FollowRequest{newer, older}, HasMore: true}}
	audits := &controlledAudits{}
	service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, audits)
	service.Clock = fixedClock{now: now}
	requests, cursor, err := service.ListIncomingRequests(context.Background(), "Bearer session", "", 2)
	if err != nil || len(requests) != 2 || cursor == "" {
		t.Fatalf("requests=%+v cursor=%q err=%v", requests, cursor, err)
	}
	if repository.listViewer != "viewer" || repository.listPage.Limit != 2 || !repository.listPage.Snapshot.Equal(now) || !repository.listPage.AfterCreated.IsZero() || repository.listPage.AfterID != "" {
		t.Fatalf("page=%+v viewer=%q", repository.listPage, repository.listViewer)
	}
	if len(audits.events) != 1 || audits.events[0].Action != audit.ResourceListed || audits.events[0].TargetType != "follow_request" {
		t.Fatalf("audits=%+v", audits.events)
	}
	repository.page = FollowRequestPage{}
	_, _, err = service.ListIncomingRequests(context.Background(), "Bearer session", cursor, 2)
	if err != nil || repository.listPage.AfterID != older.ID || !repository.listPage.AfterCreated.Equal(older.CreatedAt) || !repository.listPage.Snapshot.Equal(now) {
		t.Fatalf("next page=%+v err=%v", repository.listPage, err)
	}
}

func TestIncomingRequestCursorRejectsTamperingAndAnotherViewer(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	request := domain.FollowRequest{ID: "request-1", RecipientUserID: "viewer", CreatedAt: now.Add(-time.Minute), Requester: relationshipProfile("sender", "alice", domain.RelationshipNone)}
	repository := &controlledRelationships{page: FollowRequestPage{Requests: []domain.FollowRequest{request}, HasMore: true}}
	service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
	service.Clock = fixedClock{now: now}
	_, cursor, err := service.ListIncomingRequests(context.Background(), "Bearer session", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{cursor + "x", "not-a-cursor"} {
		repository.listViewer = ""
		if _, _, err := service.ListIncomingRequests(context.Background(), "Bearer session", invalid, 1); !errors.Is(err, ports.ErrInvalidArgument) || repository.listViewer != "" {
			t.Fatalf("cursor=%q err=%v reached repository=%q", invalid, err, repository.listViewer)
		}
	}
	service.Auth = controlledAuth{principal: ports.Principal{UserID: "other", Scopes: []string{"api:user"}}}
	repository.listViewer = ""
	if _, _, err := service.ListIncomingRequests(context.Background(), "Bearer session", cursor, 1); !errors.Is(err, ports.ErrInvalidArgument) || repository.listViewer != "" {
		t.Fatalf("cross-viewer cursor err=%v repository=%q", err, repository.listViewer)
	}
}

func TestIncomingRequestPageMustContinueStrictlyAfterItsCursor(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	boundary := domain.FollowRequest{ID: "request-2", RecipientUserID: "viewer", CreatedAt: now.Add(-time.Minute), Requester: relationshipProfile("sender-2", "bob", domain.RelationshipNone)}
	repository := &controlledRelationships{page: FollowRequestPage{Requests: []domain.FollowRequest{boundary}, HasMore: true}}
	service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
	service.Clock = fixedClock{now: now}
	_, cursor, err := service.ListIncomingRequests(context.Background(), "Bearer session", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []domain.FollowRequest{
		boundary,
		{ID: "request-3", RecipientUserID: "viewer", CreatedAt: boundary.CreatedAt, Requester: relationshipProfile("sender-3", "carol", domain.RelationshipNone)},
		{ID: "request-1", RecipientUserID: "viewer", CreatedAt: boundary.CreatedAt.Add(time.Second), Requester: relationshipProfile("sender-1", "alice", domain.RelationshipNone)},
	} {
		repository.page = FollowRequestPage{Requests: []domain.FollowRequest{item}}
		if requests, _, err := service.ListIncomingRequests(context.Background(), "Bearer session", cursor, 1); err == nil || requests != nil {
			t.Fatalf("out-of-bound page leaked: %+v err=%v", requests, err)
		}
	}
}

func TestRelationshipServiceRejectsMalformedRepositoryOutput(t *testing.T) {
	for _, test := range []struct {
		name   string
		result RelationshipResult
	}{
		{name: "self target", result: RelationshipResult{Target: relationshipProfile("viewer", "alice", domain.RelationshipFollowing)}},
		{name: "wrong username", result: RelationshipResult{Target: relationshipProfile("target", "mallory", domain.RelationshipFollowing)}},
		{name: "request missing id", result: RelationshipResult{Target: relationshipProfile("target", "alice", domain.RelationshipRequested)}},
		{name: "follow returned none", result: RelationshipResult{Target: relationshipProfile("target", "alice", domain.RelationshipNone)}},
		{name: "changed follow omitted authorization", result: RelationshipResult{Target: relationshipProfile("target", "alice", domain.RelationshipFollowing), Changed: true}},
		{name: "private request carried authorization", result: RelationshipResult{Target: relationshipProfile("target", "alice", domain.RelationshipRequested), RequestID: "request-1", Changed: true, AuthorizationChange: followerAuthorization("change-1", "target", "viewer", "viewer", ports.AuthorizationTouch)}},
		{name: "follow authorization targeted another viewer", result: RelationshipResult{Target: relationshipProfile("target", "alice", domain.RelationshipFollowing), Changed: true, AuthorizationChange: followerAuthorization("change-1", "target", "other", "viewer", ports.AuthorizationTouch)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledRelationships{result: test.result}
			service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
			if result, err := service.Follow(context.Background(), "Bearer session", "alice", "follow-request-0001"); err == nil || result != (RelationshipResult{}) {
				t.Fatalf("malformed result leaked: %+v err=%v", result, err)
			}
		})
	}
}

func TestRelationshipServiceRejectsMalformedReviewAndRequestPageOutput(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	request := domain.FollowRequest{ID: "request-1", RecipientUserID: "viewer", CreatedAt: now.Add(-time.Minute), Requester: relationshipProfile("sender", "alice", domain.RelationshipNone)}
	repository := &controlledRelationships{review: ReviewResult{Request: request, Decision: FollowRequestRejected}}
	service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
	service.Clock = fixedClock{now: now}
	if result, err := service.AcceptRequest(context.Background(), "Bearer session", "request-1", "accept-request-01"); err == nil || result != (ReviewResult{}) {
		t.Fatalf("wrong decision leaked: %+v err=%v", result, err)
	}
	repository.page = FollowRequestPage{Requests: []domain.FollowRequest{request, request}}
	if requests, _, err := service.ListIncomingRequests(context.Background(), "Bearer session", "", 2); err == nil || requests != nil {
		t.Fatalf("duplicate request page leaked: %+v err=%v", requests, err)
	}
	request.RecipientUserID = "other"
	repository.page = FollowRequestPage{Requests: []domain.FollowRequest{request}}
	if requests, _, err := service.ListIncomingRequests(context.Background(), "Bearer session", "", 1); err == nil || requests != nil {
		t.Fatalf("cross-user request page leaked: %+v err=%v", requests, err)
	}
}

func TestMissingOrBlockedRelationshipTargetIsOpaqueAndDenialAudited(t *testing.T) {
	repository := &controlledRelationships{err: ports.ErrNotFound}
	audits := &controlledAudits{}
	service := relationshipService(repository, &controlledRelationshipLimiter{allowed: true}, audits)
	if result, err := service.Follow(context.Background(), "Bearer session", "hidden", "follow-request-0001"); !errors.Is(err, ports.ErrNotFound) || result != (RelationshipResult{}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(audits.events) != 1 || audits.events[0].Action != audit.ResourceAccessDenied || audits.events[0].TargetID != "hidden" || audits.events[0].Outcome != audit.Denied {
		t.Fatalf("audits=%+v", audits.events)
	}
}
