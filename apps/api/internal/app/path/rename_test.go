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

func TestRenamePathUsesDedicatedRolePermissionAndAtomicIdempotentCommand(t *testing.T) {
	for _, actor := range []string{"creator", "administrator"} {
		t.Run(actor, func(t *testing.T) {
			now := time.Date(2026, 7, 24, 20, 0, 0, 123456000, time.UTC)
			existing := domain.Entity{
				ID: "path-1", OwnerUserID: "creator",
				Attributes: domain.Attributes{Name: "Before", Visibility: "private"},
				CreatedAt:  now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute),
			}
			want, err := existing.Rename("After", now)
			if err != nil {
				t.Fatal(err)
			}
			var checks []authorizationCall
			var gets []repositoryGet
			var renames []RenameCommand
			dependencies := configuredDependencies()
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: actor, Scopes: []string{"api:user"}}}
			dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
			dependencies.Clock = controlledClock{now: now}
			dependencies.Repository = controlledRepository{
				entity: existing, gets: &gets, renames: &renames,
				renameResult: RenameResult{Path: want},
			}

			result, err := New(dependencies).Rename(
				context.Background(), "Bearer valid", "rename-path-key-0001", existing.ID, "Before", "After",
			)
			if err != nil {
				t.Fatalf("Rename() error = %v", err)
			}
			if result.Path != want || result.Replayed {
				t.Fatalf("Rename() = %+v, want %+v", result, want)
			}
			if len(checks) != 1 || checks[0] != (authorizationCall{"path", "path-1", "rename", actor}) {
				t.Fatalf("authorization checks = %+v", checks)
			}
			if len(gets) != 1 || gets[0] != (repositoryGet{userID: actor, id: existing.ID}) {
				t.Fatalf("scoped reads = %+v", gets)
			}
			if len(renames) != 1 {
				t.Fatalf("rename commands = %+v", renames)
			}
			command := renames[0]
			if command.ActorUserID != actor || command.ExpectedName != "Before" || command.Path != want ||
				command.Idempotency.PrincipalID != actor || command.Idempotency.Operation != RenameOperation ||
				command.Idempotency.Key != "rename-path-key-0001" || len(command.Idempotency.RequestHash) != 32 {
				t.Fatalf("rename command = %+v", command)
			}
			if command.Audit.Action != audit.ResourceUpdated || command.Audit.Outcome != audit.Succeeded ||
				command.Audit.OwnerUserID != "creator" || command.Audit.ActorUserID != actor ||
				command.Audit.TargetType != "path" || command.Audit.TargetID != "path-1" ||
				command.Audit.OccurredAt != now {
				t.Fatalf("rename audit = %+v", command.Audit)
			}
		})
	}
}

func TestRenamePathDeniedOrArchivedLeavesPathUnchanged(t *testing.T) {
	now := time.Date(2026, 7, 24, 20, 0, 0, 0, time.UTC)
	base := domain.Entity{
		ID: "secret-path", OwnerUserID: "creator",
		Attributes: domain.Attributes{Name: "Before", Visibility: "private"},
		CreatedAt:  now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute),
	}
	for _, test := range []struct {
		name, actor string
		allowed     bool
		archived    bool
		want        error
	}{
		{name: "participant", actor: "participant", want: platformapp.ErrForbidden},
		{name: "supporter", actor: "supporter", want: platformapp.ErrForbidden},
		{name: "archived creator", actor: "creator", allowed: true, archived: true, want: ports.ErrConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			entity := base
			if test.archived {
				entity.ArchivedAt = now.Add(-time.Second)
			}
			var gets []repositoryGet
			var renames []RenameCommand
			var events []audit.Event
			dependencies := configuredDependencies()
			dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: test.actor, Scopes: []string{"api:user"}}}
			dependencies.Authorizer = controlledAuthorizer{allowed: test.allowed}
			dependencies.Repository = controlledRepository{entity: entity, gets: &gets, renames: &renames}
			dependencies.Audits = controlledAudits{events: &events}

			_, err := New(dependencies).Rename(
				context.Background(), "Bearer valid", "rename-path-key-0001", entity.ID, "Before", "After",
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("Rename() error = %v, want %v", err, test.want)
			}
			if len(renames) != 0 {
				t.Fatalf("rejected rename mutated persistence: %+v", renames)
			}
			if !test.allowed {
				if len(gets) != 0 || len(events) != 1 || events[0].Action != audit.ResourceAccessDenied {
					t.Fatalf("denial side effects gets=%+v audits=%+v", gets, events)
				}
			}
		})
	}
}

func TestRenamePathRejectsInvalidInputBeforeAuthorizationOrPersistence(t *testing.T) {
	for _, test := range []struct {
		name, expectedName, proposedName, key string
	}{
		{name: "missing expected name", proposedName: "After", key: "rename-path-key-0001"},
		{name: "noncanonical expected name", expectedName: " Before ", proposedName: "After", key: "rename-path-key-0001"},
		{name: "invalid proposed name", expectedName: "Before", proposedName: "line\nbreak", key: "rename-path-key-0001"},
		{name: "invalid replay key", expectedName: "Before", proposedName: "After", key: "short"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var checks []authorizationCall
			var gets []repositoryGet
			var renames []RenameCommand
			dependencies := configuredDependencies()
			dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
			dependencies.Repository = controlledRepository{gets: &gets, renames: &renames}
			_, err := New(dependencies).Rename(
				context.Background(), "Bearer valid", test.key, "path-1", test.expectedName, test.proposedName,
			)
			if !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("Rename() error = %v, want invalid argument", err)
			}
			if len(checks) != 0 || len(gets) != 0 || len(renames) != 0 {
				t.Fatalf("invalid rename reached dependencies: checks=%+v gets=%+v renames=%+v", checks, gets, renames)
			}
		})
	}
}
