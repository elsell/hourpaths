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

type leaveRepository struct {
	controlledRepository
	replayed bool
	commands *[]LeavePathCommand
	result   LeavePathResult
}

func (controlledRepository) LeavePathReplay(context.Context, string, domain.ID, ports.Idempotency) (bool, error) {
	return false, nil
}
func (controlledRepository) LeavePath(context.Context, LeavePathCommand) (LeavePathResult, error) {
	return LeavePathResult{}, nil
}
func (r leaveRepository) LeavePathReplay(context.Context, string, domain.ID, ports.Idempotency) (bool, error) {
	return r.replayed, r.err
}
func (r leaveRepository) LeavePath(_ context.Context, command LeavePathCommand) (LeavePathResult, error) {
	if r.commands != nil {
		*r.commands = append(*r.commands, command)
	}
	return r.result, r.err
}

func TestLeavePathUsesAuthoritativePermissionAndAtomicRetainedActivityCommand(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 123456789, time.UTC)
	change := ports.AuthorizationChange{ID: "auth-delete", ResourceType: "path", ResourceID: "path-1", Relation: "participant", SubjectType: "user", SubjectID: "member", OwnerUserID: "creator", ActorUserID: "member", Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute}
	var commands []LeavePathCommand
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.NewID = sequentialIDs("audit-id", "activity-id", "notification-id", "auth-delete")
	dependencies.Repository = leaveRepository{commands: &commands, result: LeavePathResult{PathID: "path-1", Left: true, ActivityRetained: true, AuthorizationChanges: []ports.AuthorizationChange{change}}}
	dependencies.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{change}}

	result, err := New(dependencies).Leave(context.Background(), "Bearer valid", "leave-path-key-0001", "path-1", true, true)
	if err != nil || !result.Left || !result.ActivityRetained || len(commands) != 1 {
		t.Fatalf("Leave() = %+v, %v; commands=%+v", result, err, commands)
	}
	command := commands[0]
	if command.ActorUserID != "member" || command.PathID != "path-1" || !command.RetainActivity || command.LeftAt != now.Truncate(time.Microsecond) {
		t.Fatalf("command = %+v", command)
	}
	if command.Idempotency.Operation != LeavePathOperation || command.Idempotency.Key != "leave-path-key-0001" || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("idempotency = %+v", command.Idempotency)
	}
	if command.Audit.Action != audit.ResourceUpdated || command.Audit.Outcome != audit.Succeeded {
		t.Fatalf("audit = %+v", command.Audit)
	}
}

func TestLeavePathUsesAuthoritativePermissionAndAtomicDeletedActivityCommand(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 123456789, time.UTC)
	change := ports.AuthorizationChange{ID: "auth-delete", ResourceType: "path", ResourceID: "path-1", Relation: "administrator", SubjectType: "user", SubjectID: "member", OwnerUserID: "creator", ActorUserID: "member", Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute}
	var commands []LeavePathCommand
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.NewID = sequentialIDs("audit-id", "notification-id", "auth-delete")
	dependencies.Repository = leaveRepository{commands: &commands, result: LeavePathResult{PathID: "path-1", Left: true, ActivityRetained: false, AuthorizationChanges: []ports.AuthorizationChange{change}}}
	dependencies.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{change}}

	result, err := New(dependencies).Leave(context.Background(), "Bearer valid", "leave-path-delete-0001", "path-1", true, false)
	if err != nil || !result.Left || result.ActivityRetained || len(commands) != 1 {
		t.Fatalf("Leave() = %+v, %v; commands=%+v", result, err, commands)
	}
	if commands[0].RetainActivity || commands[0].LeftAt != now.Truncate(time.Microsecond) {
		t.Fatalf("command = %+v", commands[0])
	}
}

func TestLeavePathFailsClosedForCreatorOrNonmemberAndInvalidChoice(t *testing.T) {
	for _, test := range []struct {
		name                       string
		confirmed, retain, allowed bool
		want                       error
	}{
		{"creator or nonmember", true, true, false, platformapp.ErrForbidden},
		{"not confirmed", false, true, true, ports.ErrInvalidArgument},
	} {
		t.Run(test.name, func(t *testing.T) {
			var commands []LeavePathCommand
			dependencies := configuredDependencies()
			dependencies.Authorizer = controlledAuthorizer{allowed: test.allowed}
			dependencies.Repository = leaveRepository{commands: &commands}
			_, err := New(dependencies).Leave(context.Background(), "Bearer valid", "leave-path-key-0001", "secret", test.confirmed, test.retain)
			if !errors.Is(err, test.want) || len(commands) != 0 {
				t.Fatalf("Leave() = %v; commands=%+v", err, commands)
			}
		})
	}
}

func TestLeavePathChoiceIsBoundToIdempotencyAndReplayReceipt(t *testing.T) {
	retained := canonicalLeavePathRequestHash("path-1", true, true)
	deleted := canonicalLeavePathRequestHash("path-1", true, false)
	if retained == deleted {
		t.Fatal("retain and delete choices have the same request hash")
	}
	dependencies := configuredDependencies()
	dependencies.Repository = leaveRepository{replayed: true}
	result, err := New(dependencies).Leave(context.Background(), "Bearer valid", "leave-path-delete-0001", "path-1", true, false)
	if err != nil || !result.Replayed || !result.Left || result.ActivityRetained {
		t.Fatalf("Leave() = %+v, %v", result, err)
	}
}

func TestLeavePathReplaySucceedsBeforeRemovedAuthorizationIsRechecked(t *testing.T) {
	var checks []authorizationCall
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{calls: &checks}
	dependencies.Repository = leaveRepository{replayed: true}
	result, err := New(dependencies).Leave(context.Background(), "Bearer valid", "leave-path-key-0001", "path-1", true, true)
	if err != nil || !result.Replayed || !result.Left || !result.ActivityRetained || len(checks) != 0 {
		t.Fatalf("Leave() = %+v, %v; checks=%+v", result, err, checks)
	}
}

func TestLeavePathReplayReconcilesPendingAuthorizationBeforeReportingSuccess(t *testing.T) {
	change := ports.AuthorizationChange{ID: "auth-delete", ResourceType: "path", ResourceID: "path-1", Relation: "participant", SubjectType: "user", SubjectID: "member", OwnerUserID: "creator", ActorUserID: "member", Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute}
	failure := errors.New("SpiceDB unavailable")
	failed := configuredDependencies()
	failed.Repository = leaveRepository{replayed: true}
	failed.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{change}}
	failed.Authorizer = controlledAuthorizer{err: failure}
	if _, err := New(failed).Leave(context.Background(), "Bearer valid", "leave-path-key-0001", "path-1", true, true); !errors.Is(err, failure) {
		t.Fatalf("first replay error = %v", err)
	}

	var deleted []relationshipWrite
	retry := configuredDependencies()
	retry.Repository = leaveRepository{replayed: true}
	retry.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{change}}
	retry.Authorizer = deletionOrderingAuthorizer{controlledAuthorizer: controlledAuthorizer{}, operations: &[]ports.AuthorizationOperation{}, relationships: &deleted}
	result, err := New(retry).Leave(context.Background(), "Bearer valid", "leave-path-key-0001", "path-1", true, true)
	if err != nil || !result.Replayed || len(deleted) != 1 || deleted[0].relation != "participant" {
		t.Fatalf("retry = %+v, %v; deletes=%+v", result, err, deleted)
	}
}

func TestLeavePathRepositoryRaceReplayAlsoReconcilesAuthorization(t *testing.T) {
	change := ports.AuthorizationChange{ID: "auth-delete", ResourceType: "path", ResourceID: "path-1", Relation: "supporter", SubjectType: "user", SubjectID: "member", OwnerUserID: "creator", ActorUserID: "member", Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute}
	var deleted []relationshipWrite
	dependencies := configuredDependencies()
	dependencies.Repository = leaveRepository{result: LeavePathResult{Replayed: true}}
	dependencies.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{change}}
	dependencies.Authorizer = deletionOrderingAuthorizer{controlledAuthorizer: controlledAuthorizer{allowed: true}, operations: &[]ports.AuthorizationOperation{}, relationships: &deleted}
	result, err := New(dependencies).Leave(context.Background(), "Bearer valid", "leave-path-key-0001", "path-1", true, true)
	if err != nil || !result.Replayed || len(deleted) != 1 {
		t.Fatalf("race replay = %+v, %v; deletes=%+v", result, err, deleted)
	}
}

func TestLeavePathDoesNotReportSuccessWhileAuthorizationBatchIsPending(t *testing.T) {
	dependencies := configuredDependencies()
	dependencies.Repository = leaveRepository{replayed: true}
	dependencies.AuthorizationOutbox = controlledOutbox{claimErr: ports.ErrAuthorizationPending}
	if _, err := New(dependencies).Leave(context.Background(), "Bearer valid", "leave-path-key-0001", "path-1", true, true); !errors.Is(err, ports.ErrAuthorizationPending) {
		t.Fatalf("Leave() error = %v", err)
	}
}

func TestLeavePathFailsClosedForArchivedPathDespiteStaleLeavePermission(t *testing.T) {
	var commands []LeavePathCommand
	dependencies := configuredDependencies()
	dependencies.Repository = leaveRepository{controlledRepository: controlledRepository{entity: domain.Entity{ID: "path-1", OwnerUserID: "creator", ArchivedAt: dependencies.Clock.Now(), Attributes: domain.Attributes{Name: "Read", Visibility: "private"}}}, commands: &commands}
	_, err := New(dependencies).Leave(context.Background(), "Bearer valid", "leave-path-key-0001", "path-1", true, true)
	if !errors.Is(err, platformapp.ErrForbidden) || len(commands) != 0 {
		t.Fatalf("archived Leave() = %v; commands=%+v", err, commands)
	}
}
