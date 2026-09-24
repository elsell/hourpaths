package path

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type deletionOrderingAuthorizer struct {
	controlledAuthorizer
	operations    *[]ports.AuthorizationOperation
	relationships *[]relationshipWrite
}

func (f deletionOrderingAuthorizer) WriteRelationship(ctx context.Context, resourceType, resourceID, relation, subjectType, subjectID string) error {
	*f.operations = append(*f.operations, ports.AuthorizationTouch)
	return f.controlledAuthorizer.WriteRelationship(ctx, resourceType, resourceID, relation, subjectType, subjectID)
}

func (f deletionOrderingAuthorizer) DeleteRelationship(_ context.Context, resourceType, resourceID, relation, subjectType, subjectID string) error {
	*f.operations = append(*f.operations, ports.AuthorizationDelete)
	if f.relationships != nil {
		*f.relationships = append(*f.relationships, relationshipWrite{resourceType, resourceID, relation, subjectType, subjectID})
	}
	return f.err
}

type orderedDeletionOutbox struct {
	controlledOutbox
	changes []ports.AuthorizationChange
	next    int
}

type deletionRepository struct {
	controlledRepository
	replayed  bool
	deletions *[]DeletePathCommand
	result    DeletePathResult
	err       error
}

func (controlledRepository) DeletionReplay(context.Context, string, domain.ID, ports.Idempotency) (bool, error) {
	return false, nil
}

func (controlledRepository) DeletePath(context.Context, DeletePathCommand) (DeletePathResult, error) {
	return DeletePathResult{}, nil
}

func (f deletionRepository) DeletionReplay(context.Context, string, domain.ID, ports.Idempotency) (bool, error) {
	return f.replayed, f.err
}

func (f deletionRepository) DeletePath(_ context.Context, command DeletePathCommand) (DeletePathResult, error) {
	if f.deletions != nil {
		*f.deletions = append(*f.deletions, command)
	}
	return f.result, f.err
}

func (f *orderedDeletionOutbox) ClaimAuthorizationChangeForResource(context.Context, string, string, string, time.Duration) (ports.AuthorizationChange, error) {
	if f.next >= len(f.changes) {
		return ports.AuthorizationChange{}, ports.ErrNotFound
	}
	change := f.changes[f.next]
	f.next++
	return change, nil
}

func TestConfirmedPathDeletionUsesCreatorPermissionAndAtomicCommand(t *testing.T) {
	now := time.Date(2026, 7, 27, 15, 0, 0, 123456789, time.UTC)
	var checks []authorizationCall
	var commands []DeletePathCommand
	dependencies := configuredDependencies()
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	dependencies.Clock = controlledClock{now: now}
	dependencies.NewID = sequentialIDs("audit-id", "authorization-id")
	dependencies.Repository = deletionRepository{deletions: &commands, result: DeletePathResult{PathID: "path-id", Deleted: true, AuthorizationChanges: []ports.AuthorizationChange{{ID: "authorization-id", ResourceType: "path", ResourceID: "path-id", Relation: "creator", SubjectType: "user", SubjectID: "creator", OwnerUserID: "creator", ActorUserID: "creator", Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute}}}}

	if _, err := New(dependencies).Delete(context.Background(), "Bearer valid", "delete-path-key-0001", "path-id", true, "Guitar"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(checks) != 1 || checks[0] != (authorizationCall{"path", "path-id", "manage_lifecycle", "creator"}) {
		t.Fatalf("checks = %+v", checks)
	}
	if len(commands) != 1 {
		t.Fatalf("commands = %+v", commands)
	}
	command := commands[0]
	if command.ActorUserID != "creator" || command.PathID != "path-id" || command.ExpectedName != "Guitar" || command.DeletedAt != now.Truncate(time.Microsecond) {
		t.Fatalf("command = %+v", command)
	}
	if command.Idempotency.PrincipalID != "creator" || command.Idempotency.Operation != DeletePathOperation || command.Idempotency.Key != "delete-path-key-0001" || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("idempotency = %+v", command.Idempotency)
	}
	if command.Audit.Action != audit.ResourceDeleted || command.Audit.OwnerUserID != "creator" || command.Audit.ActorUserID != "creator" || command.Audit.TargetID != "path-id" {
		t.Fatalf("audit = %+v", command.Audit)
	}
}

func TestPathDeletionRequiresConfirmationAndFailsClosedForNonCreator(t *testing.T) {
	for _, test := range []struct {
		name               string
		confirmed, allowed bool
		want               error
	}{
		{"unconfirmed", false, true, ports.ErrInvalidArgument},
		{"non creator", true, false, platformapp.ErrForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			var commands []DeletePathCommand
			dependencies := configuredDependencies()
			dependencies.Authorizer = controlledAuthorizer{allowed: test.allowed}
			dependencies.Repository = deletionRepository{deletions: &commands}
			_, err := New(dependencies).Delete(context.Background(), "Bearer valid", "delete-path-key-0001", domain.ID("secret"), test.confirmed, "Guitar")
			if !errors.Is(err, test.want) || len(commands) != 0 {
				t.Fatalf("Delete() = %v, commands=%+v", err, commands)
			}
		})
	}
}

func TestPathDeletionFailsClosedWithoutAuthorizationCleanupDependencies(t *testing.T) {
	for _, test := range []struct {
		name   string
		remove func(*Dependencies)
	}{
		{"authorization outbox", func(dependencies *Dependencies) { dependencies.AuthorizationOutbox = nil }},
		{"authorization serializer", func(dependencies *Dependencies) { dependencies.AuthorizationSerializer = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dependencies := configuredDependencies()
			test.remove(&dependencies)
			if _, err := New(dependencies).Delete(context.Background(), "Bearer valid", "delete-path-key-0001", "path-id", true, "Guitar"); !errors.Is(err, errInvalidPathDependencies) {
				t.Fatalf("Delete() error = %v, want invalid dependencies", err)
			}
		})
	}
}

func TestPathDeletionReplaySucceedsBeforeAuthorizationRecheck(t *testing.T) {
	var checks []authorizationCall
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{calls: &checks}
	dependencies.Repository = deletionRepository{replayed: true}
	result, err := New(dependencies).Delete(context.Background(), "Bearer valid", "delete-path-key-0001", "deleted-path", true, "Guitar")
	if err != nil || result.PathID != "deleted-path" || !result.Deleted || !result.Replayed {
		t.Fatal(err)
	}
	if len(checks) != 0 {
		t.Fatalf("replay rechecked removed authorization: %+v", checks)
	}
}

func TestPathDeletionReconcilesOlderAuthorizationWorkBeforeCleanup(t *testing.T) {
	olderTouch := ports.AuthorizationChange{
		ID: "older-touch", ResourceType: "path", ResourceID: "path-id", Relation: "public_viewer",
		SubjectType: "user", SubjectID: "*", OwnerUserID: "creator", ActorUserID: "creator",
		Operation: ports.AuthorizationTouch, LockedBy: "path-api", Lease: time.Minute,
	}
	cleanupDelete := olderTouch
	cleanupDelete.ID = "cleanup-delete"
	cleanupDelete.Operation = ports.AuthorizationDelete
	var operations []ports.AuthorizationOperation
	dependencies := configuredDependencies()
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
	dependencies.Authorizer = deletionOrderingAuthorizer{controlledAuthorizer: controlledAuthorizer{allowed: true}, operations: &operations}
	dependencies.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{olderTouch, cleanupDelete}}
	dependencies.Repository = deletionRepository{result: DeletePathResult{
		PathID: "path-id", Deleted: true, AuthorizationChanges: []ports.AuthorizationChange{cleanupDelete},
	}}

	if _, err := New(dependencies).Delete(context.Background(), "Bearer valid", "delete-path-key-0001", "path-id", true, "Guitar"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(operations) != 2 || operations[0] != ports.AuthorizationTouch || operations[1] != ports.AuthorizationDelete {
		t.Fatalf("authorization operations = %v, want touch then delete", operations)
	}
}

func TestPathDeletionRemovesCompletedPublicViewerAuthorization(t *testing.T) {
	cleanupDelete := ports.AuthorizationChange{
		ID: "public-cleanup-delete", ResourceType: "path", ResourceID: "path-id", Relation: "public_viewer",
		SubjectType: "user", SubjectID: "*", OwnerUserID: "creator", ActorUserID: "creator",
		Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute,
	}
	var operations []ports.AuthorizationOperation
	var relationships []relationshipWrite
	dependencies := configuredDependencies()
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
	dependencies.Authorizer = deletionOrderingAuthorizer{
		controlledAuthorizer: controlledAuthorizer{allowed: true},
		operations:           &operations,
		relationships:        &relationships,
	}
	dependencies.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{cleanupDelete}}
	dependencies.Repository = deletionRepository{result: DeletePathResult{
		PathID: "path-id", Deleted: true, AuthorizationChanges: []ports.AuthorizationChange{cleanupDelete},
	}}

	if _, err := New(dependencies).Delete(context.Background(), "Bearer valid", "delete-path-key-0001", "path-id", true, "Guitar"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(operations) != 1 || operations[0] != ports.AuthorizationDelete {
		t.Fatalf("authorization operations = %v, want public-viewer delete", operations)
	}
	want := relationshipWrite{"path", "path-id", "public_viewer", "user", "*"}
	if len(relationships) != 1 || relationships[0] != want {
		t.Fatalf("deleted relationships = %+v, want %+v", relationships, want)
	}
}

func TestPathDeletionDefersCleanupBehindAnEarlierAuthorizationBatch(t *testing.T) {
	cleanupDelete := ports.AuthorizationChange{
		ID: "cleanup-delete", ResourceType: "path", ResourceID: "path-id", Relation: "creator",
		SubjectType: "user", SubjectID: "creator", OwnerUserID: "creator", ActorUserID: "creator",
		Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute,
	}
	var operations []ports.AuthorizationOperation
	dependencies := configuredDependencies()
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
	dependencies.Authorizer = deletionOrderingAuthorizer{controlledAuthorizer: controlledAuthorizer{allowed: true}, operations: &operations}
	dependencies.AuthorizationOutbox = controlledOutbox{claimErr: ports.ErrAuthorizationPending}
	dependencies.Repository = deletionRepository{result: DeletePathResult{
		PathID: "path-id", Deleted: true, AuthorizationChanges: []ports.AuthorizationChange{cleanupDelete},
	}}

	result, err := New(dependencies).Delete(context.Background(), "Bearer valid", "delete-path-key-0001", "path-id", true, "Guitar")
	if err != nil || !result.Deleted {
		t.Fatalf("Delete() = %+v, %v", result, err)
	}
	if len(operations) != 0 {
		t.Fatalf("cleanup bypassed earlier authorization batch: %v", operations)
	}
}
