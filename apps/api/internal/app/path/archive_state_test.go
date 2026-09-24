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

func (f controlledRepository) SetArchiveState(_ context.Context, command SetArchiveStateCommand) (SetArchiveStateResult, error) {
	if f.archiveChanges != nil {
		*f.archiveChanges = append(*f.archiveChanges, command)
	}
	return f.archiveResult, f.err
}

func TestConfirmedArchiveStateChangeUsesCreatorOnlyPermissionAndOneLifecycleInstant(t *testing.T) {
	existing := domain.Entity{
		ID: "shared-path", OwnerUserID: "creator",
		Attributes: domain.Attributes{Name: "Practice", Visibility: "followers"},
		CreatedAt:  time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC),
	}
	var checks []authorizationCall
	var gets []repositoryGet
	var changes []SetArchiveStateCommand
	dependencies := configuredDependencies()
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	rawNow := time.Date(2026, 7, 23, 15, 0, 0, 123_456_789, time.UTC)
	durableNow := rawNow.Truncate(time.Microsecond)
	dependencies.Clock = controlledClock{now: rawNow}
	wantPath, err := existing.Archive(durableNow)
	if err != nil {
		t.Fatalf("Archive() setup error = %v", err)
	}
	wantResult := SetArchiveStateResult{Path: wantPath}
	dependencies.Repository = controlledRepository{
		entity: existing, gets: &gets, archiveChanges: &changes, archiveResult: wantResult,
	}

	got, err := New(dependencies).SetArchiveState(
		context.Background(), "Bearer valid", "archive-key-0001", existing.ID, true, false, true,
	)
	if err != nil {
		t.Fatalf("SetArchiveState() error = %v", err)
	}
	if got != wantResult {
		t.Fatalf("SetArchiveState() = %+v, want %+v", got, wantResult)
	}
	if len(checks) != 1 || checks[0] != (authorizationCall{"path", string(existing.ID), "manage_lifecycle", "creator"}) {
		t.Fatalf("authorization checks = %+v", checks)
	}
	if len(gets) != 1 || gets[0] != (repositoryGet{userID: "creator", id: existing.ID}) {
		t.Fatalf("Path reads = %+v", gets)
	}
	if len(changes) != 1 {
		t.Fatalf("archive-state commands = %+v", changes)
	}
	command := changes[0]
	if command.ActorUserID != "creator" || command.ExpectedArchived || !command.Archived || command.Path != wantPath || command.ChangedAt != durableNow {
		t.Fatalf("archive-state command = %+v", command)
	}
	if command.Idempotency.PrincipalID != "creator" || command.Idempotency.Operation != SetArchiveStateOperation ||
		command.Idempotency.Key != "archive-key-0001" || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("archive-state idempotency = %+v", command.Idempotency)
	}
	if command.NewActivityID == nil || command.NewActivityID() == "" {
		t.Fatal("archive-state command did not inject activity IDs")
	}
	if command.Audit.Action != audit.ResourceUpdated || command.Audit.Outcome != audit.Succeeded ||
		command.Audit.OwnerUserID != "creator" || command.Audit.ActorUserID != "creator" ||
		command.Audit.TargetType != "path" || command.Audit.TargetID != string(existing.ID) || command.Audit.OccurredAt != durableNow {
		t.Fatalf("archive-state audit = %+v", command.Audit)
	}
}

func TestArchiveStateChangeRequiresConfirmationAndCreatorAuthorizationBeforePathRead(t *testing.T) {
	for _, test := range []struct {
		name      string
		confirmed bool
		allowed   bool
		wantErr   error
		wantAudit bool
	}{
		{name: "unconfirmed", allowed: true, wantErr: ports.ErrInvalidArgument},
		{name: "non creator", confirmed: true, wantErr: platformapp.ErrForbidden, wantAudit: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var checks []authorizationCall
			var gets []repositoryGet
			var changes []SetArchiveStateCommand
			var events []audit.Event
			dependencies := configuredDependencies()
			dependencies.Authorizer = controlledAuthorizer{allowed: test.allowed, calls: &checks}
			dependencies.Repository = controlledRepository{gets: &gets, archiveChanges: &changes}
			dependencies.Audits = controlledAudits{events: &events}

			_, err := New(dependencies).SetArchiveState(
				context.Background(), "Bearer valid", "archive-key-0001", "secret-path", test.confirmed, false, true,
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("SetArchiveState() error = %v, want %v", err, test.wantErr)
			}
			if len(gets) != 0 || len(changes) != 0 {
				t.Fatalf("rejected change reached persistence: gets=%+v changes=%+v", gets, changes)
			}
			if !test.confirmed && (len(checks) != 0 || len(events) != 0) {
				t.Fatalf("unconfirmed change reached authorization/audit: checks=%+v events=%+v", checks, events)
			}
			if test.wantAudit && (len(checks) != 1 || len(events) != 1 || events[0].Action != audit.ResourceAccessDenied || events[0].Outcome != audit.Denied) {
				t.Fatalf("denied change evidence: checks=%+v events=%+v", checks, events)
			}
		})
	}
}
