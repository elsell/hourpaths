package activity

import (
	"context"
	"errors"
	"testing"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestDeleteActivityAuthorizesTrackingAndBuildsOwnerScopedAtomicCommand(t *testing.T) {
	var deletes []DeleteActivityCommand
	var authorization []authCall
	service := testService(testRepository{
		deletes:      &deletes,
		deleteResult: DeleteActivityResult{AccumulatedSeconds: 42, SessionCount: 3, UnreadNotificationCount: 4, RemovedFeedEventIDs: []string{"achievement:a", "practice:activity-1"}},
	})
	service.Authorizer = testAuthorizer{allowed: true, calls: &authorization}

	result, err := service.DeleteActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "delete-request-001")
	if err != nil || result.AccumulatedSeconds != 42 || result.SessionCount != 3 || result.UnreadNotificationCount != 4 || result.Replayed || len(result.RemovedFeedEventIDs) != 2 || len(deletes) != 1 {
		t.Fatalf("DeleteActivity() = (%+v, %v), deletes=%+v", result, err, deletes)
	}
	if len(authorization) != 1 || authorization[0] != (authCall{"path", "path-1", "track", "user-1"}) {
		t.Fatalf("authorization=%+v", authorization)
	}
	command := deletes[0]
	if command.ParticipantID != "user-1" || command.PathID != "path-1" || command.ActivityID != "activity-1" {
		t.Fatalf("command=%+v", command)
	}
	if command.Idempotency.PrincipalID != "user-1" || command.Idempotency.Operation != DeleteActivityOperation || command.Idempotency.Key != "delete-request-001" || len(command.Idempotency.RequestHash) != sha256Size {
		t.Fatalf("idempotency=%+v", command.Idempotency)
	}
	if !command.Audit.Valid() || command.Audit.Action != audit.ResourceDeleted || command.Audit.TargetType != "activity" || command.Audit.TargetID != "activity-1" || command.Audit.OwnerUserID != "user-1" || command.Audit.ActorUserID != "user-1" {
		t.Fatalf("audit=%+v", command.Audit)
	}
}

func TestDeleteActivityPreservesReplayAndAuditsOpaqueNotFound(t *testing.T) {
	service := testService(testRepository{deleteResult: DeleteActivityResult{AccumulatedSeconds: 7, SessionCount: 2, UnreadNotificationCount: 5, RemovedFeedEventIDs: []string{"practice:activity-1"}, Replayed: true}})
	result, err := service.DeleteActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "delete-request-001")
	if err != nil || result.AccumulatedSeconds != 7 || result.SessionCount != 2 || result.UnreadNotificationCount != 5 || len(result.RemovedFeedEventIDs) != 1 || result.RemovedFeedEventIDs[0] != "practice:activity-1" || !result.Replayed {
		t.Fatalf("replayed DeleteActivity() = (%+v, %v)", result, err)
	}

	var events []audit.Event
	service.Repository = testRepository{deleteErr: ports.ErrNotFound}
	service.Audits = testAudits{events: &events}
	_, err = service.DeleteActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "delete-request-002")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("opaque not-found error=%v", err)
	}
	if len(events) != 1 {
		t.Fatalf("opaque not-found audits=%+v", events)
	}
	event := events[0]
	if !event.Valid() || event.Action != audit.ResourceAccessDenied || event.Outcome != audit.Denied || event.TargetType != "activity" || event.TargetID != "activity-1" || event.OwnerUserID != "user-1" || event.ActorUserID != "user-1" {
		t.Fatalf("opaque not-found audit=%+v", event)
	}
}

func TestDeleteActivityFailsClosedWhenOpaqueNotFoundAuditCannotPersist(t *testing.T) {
	auditErr := errors.New("audit unavailable")
	var events []audit.Event
	service := testService(testRepository{deleteErr: ports.ErrNotFound})
	service.Audits = testAudits{events: &events, err: auditErr}

	_, err := service.DeleteActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "delete-request-001")
	if !errors.Is(err, auditErr) || errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeleteActivity() error=%v, want audit failure only", err)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceAccessDenied || events[0].TargetType != "activity" || events[0].TargetID != "activity-1" {
		t.Fatalf("failed opaque not-found audit=%+v", events)
	}
}

func TestDeleteActivityRejectsInvalidOrUnauthorizedRequestsBeforePersistence(t *testing.T) {
	tests := []struct {
		name                    string
		configure               func(*Service)
		pathID, activityID, key string
		want                    error
	}{
		{name: "invalid path", pathID: " path-1", activityID: "activity-1", key: "delete-request-001", want: ports.ErrInvalidArgument},
		{name: "invalid activity", pathID: "path-1", activityID: "activity-1 ", key: "delete-request-001", want: ports.ErrInvalidArgument},
		{name: "invalid key", pathID: "path-1", activityID: "activity-1", key: "short", want: ports.ErrInvalidArgument},
		{name: "authentication", pathID: "path-1", activityID: "activity-1", key: "delete-request-001", configure: func(service *Service) { service.Auth = testAuth{err: ports.ErrInvalidCredential} }, want: ports.ErrInvalidCredential},
		{name: "track denied", pathID: "path-1", activityID: "activity-1", key: "delete-request-001", configure: func(service *Service) { service.Authorizer = testAuthorizer{allowed: false} }, want: platformapp.ErrForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var deletes []DeleteActivityCommand
			service := testService(testRepository{deletes: &deletes})
			if test.configure != nil {
				test.configure(service)
			}
			_, err := service.DeleteActivity(context.Background(), "Bearer input", test.pathID, test.activityID, test.key)
			if !errors.Is(err, test.want) || len(deletes) != 0 {
				t.Fatalf("DeleteActivity() error=%v deletes=%+v", err, deletes)
			}
		})
	}
}

func TestDeleteActivityRejectsInvalidRepositoryProjection(t *testing.T) {
	for _, result := range []DeleteActivityResult{
		{AccumulatedSeconds: -1, RemovedFeedEventIDs: []string{"practice:activity-1"}},
		{SessionCount: -1, RemovedFeedEventIDs: []string{"practice:activity-1"}},
		{UnreadNotificationCount: -1, RemovedFeedEventIDs: []string{"practice:activity-1"}},
		{AccumulatedSeconds: 1},
		{AccumulatedSeconds: 1, RemovedFeedEventIDs: []string{"practice:activity-1", "achievement:z"}},
		{AccumulatedSeconds: 1, RemovedFeedEventIDs: []string{"practice:activity-1", "practice:activity-1"}},
		{AccumulatedSeconds: 1, RemovedFeedEventIDs: []string{"practice:activity-2"}},
		{AccumulatedSeconds: 1, RemovedFeedEventIDs: []string{"unknown:event", "practice:activity-1"}},
		{AccumulatedSeconds: 1, RemovedFeedEventIDs: []string{"achievement:", "practice:activity-1"}},
		{AccumulatedSeconds: 1, RemovedFeedEventIDs: []string{" invalid"}},
	} {
		service := testService(testRepository{deleteResult: result})
		_, err := service.DeleteActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "delete-request-001")
		if !errors.Is(err, errInvalidDependencies) {
			t.Fatalf("invalid deletion result %+v error=%v", result, err)
		}
	}
}
