package path

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestSetVisibilityUsesCreatorPermissionProfileBoundAndAtomicCommand(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 123456789, time.UTC)
	existing := domain.Entity{ID: "path-1", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Read", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Truncate(time.Microsecond)}
	want, _ := existing.SetVisibility("followers", existing.UpdatedAt.Add(time.Microsecond))
	var checks []authorizationCall
	var commands []SetVisibilityCommand
	d := configuredDependencies()
	d.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
	d.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	d.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPrivate, firstDayOfWeek: identity.FirstDayMonday}
	d.Clock = controlledClock{now: now}
	change := ports.AuthorizationChange{ID: "visibility-change", ResourceType: "path", ResourceID: "path-1", Relation: "followers_owner", SubjectType: "user", SubjectID: "creator", OwnerUserID: "creator", ActorUserID: "creator", Operation: ports.AuthorizationTouch, LockedBy: "path-api", Lease: time.Minute}
	d.Repository = controlledRepository{entity: existing, visibilityChanges: &commands, visibilityResult: SetVisibilityResult{Path: want, AuthorizationChanges: []ports.AuthorizationChange{change}}}
	got, err := New(d).SetVisibility(context.Background(), "Bearer valid", "visibility-key-0001", "path-1", true, "private", "followers")
	if err != nil || got.Path != want {
		t.Fatalf("SetVisibility()=%+v err=%v", got, err)
	}
	if len(checks) != 1 || checks[0] != (authorizationCall{"path", "path-1", "manage_visibility", "creator"}) {
		t.Fatalf("checks=%+v", checks)
	}
	if len(commands) != 1 || commands[0].ExpectedVisibility != "private" || commands[0].Path != want || commands[0].Idempotency.Operation != SetVisibilityOperation || commands[0].Audit.Action != audit.PathVisibilityChanged {
		t.Fatalf("command=%+v", commands)
	}
}

func TestSetVisibilityRejectsMalformedAuthorizationChanges(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	existing := domain.Entity{ID: "path-1", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Read", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	want, _ := existing.SetVisibility("followers", now)
	d := configuredDependencies()
	d.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
	d.Authorizer = controlledAuthorizer{allowed: true}
	d.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPrivate, firstDayOfWeek: identity.FirstDayMonday}
	d.Clock = controlledClock{now: now}
	d.Repository = controlledRepository{entity: existing, visibilityResult: SetVisibilityResult{Path: want}}
	if _, err := New(d).SetVisibility(context.Background(), "Bearer valid", "visibility-key-0004", "path-1", true, "private", "followers"); !errors.Is(err, errInvalidPathDependencies) {
		t.Fatalf("err=%v want invalid dependencies", err)
	}
}

func TestSetVisibilityFailsClosedForRoleProfileArchiveAndStaleExpectedValue(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	base := domain.Entity{ID: "path-1", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Read", Visibility: "followers"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	for _, tc := range []struct {
		name           string
		allowed        bool
		profile        identity.ProfileVisibility
		path           domain.Entity
		expected, next string
		want           error
	}{
		{"administrator", false, identity.ProfileVisibilityPublic, base, "followers", "private", platformapp.ErrForbidden},
		{"private profile public", true, identity.ProfileVisibilityPrivate, base, "followers", "public", ports.ErrInvalidArgument},
		{"stale", true, identity.ProfileVisibilityPublic, base, "private", "public", ports.ErrConflict},
		{"archived", true, identity.ProfileVisibilityPublic, func() domain.Entity { p := base; p.ArchivedAt = now.Add(-time.Second); return p }(), "followers", "private", ports.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := configuredDependencies()
			d.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
			d.Authorizer = controlledAuthorizer{allowed: tc.allowed}
			d.Profiles = controlledProfiles{visibility: tc.profile, firstDayOfWeek: identity.FirstDayMonday}
			d.Repository = controlledRepository{entity: tc.path}
			d.Clock = controlledClock{now: now}
			_, err := New(d).SetVisibility(context.Background(), "Bearer valid", "visibility-key-0002", "path-1", true, tc.expected, tc.next)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
		})
	}
}

func TestSetVisibilityReplaysOriginalSnapshotBeforeCurrentProfileOrPathState(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	original := domain.Entity{ID: "path-1", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Before", Visibility: "followers"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	d := configuredDependencies()
	d.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
	d.Authorizer = controlledAuthorizer{allowed: true}
	d.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPrivate, err: errors.New("profile must not be read for replay")}
	d.Repository = controlledRepository{visibilityReplay: &SetVisibilityResult{Path: original, Replayed: true}, entity: domain.Entity{ID: "path-1", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Later", Visibility: "public"}, CreatedAt: original.CreatedAt, UpdatedAt: now}}
	result, err := New(d).SetVisibility(context.Background(), "Bearer valid", "visibility-key-0003", "path-1", true, "private", "followers")
	if err != nil || result.Path != original || !result.Replayed {
		t.Fatalf("replay=%+v err=%v", result, err)
	}
}

func TestSetVisibilityReconcilesEveryDurableResourceChangeInOrderBeforeSuccess(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	existing := domain.Entity{ID: "path-1", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Read", Visibility: "followers"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	updated, _ := existing.SetVisibility("public", now)
	change := func(id, relation, subject string, operation ports.AuthorizationOperation) ports.AuthorizationChange {
		return ports.AuthorizationChange{ID: id, ResourceType: "path", ResourceID: "path-1", Relation: relation, SubjectType: "user", SubjectID: subject, OwnerUserID: "creator", ActorUserID: "creator", Operation: operation, LockedBy: "path-api", Lease: time.Minute}
	}
	resultChanges := []ports.AuthorizationChange{
		change("current-delete", "followers_owner", "creator", ports.AuthorizationDelete),
		change("current-touch", "public_viewer", "*", ports.AuthorizationTouch),
	}
	queue := []ports.AuthorizationChange{
		change("earlier-delete", "public_viewer", "*", ports.AuthorizationDelete),
		resultChanges[0], resultChanges[1],
	}
	claimIndex := 0
	var operations []string
	d := configuredDependencies()
	d.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
	d.Authorizer = controlledAuthorizer{allowed: true, operations: &operations}
	d.AuthorizationOutbox = controlledOutbox{changes: &queue, claimIndex: &claimIndex}
	d.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPublic, firstDayOfWeek: identity.FirstDayMonday}
	d.Clock = controlledClock{now: now}
	d.Repository = controlledRepository{entity: existing, visibilityResult: SetVisibilityResult{Path: updated, AuthorizationChanges: resultChanges}}
	if _, err := New(d).SetVisibility(context.Background(), "Bearer valid", "visibility-key-ordered", "path-1", true, "followers", "public"); err != nil {
		t.Fatal(err)
	}
	want := []string{"delete:public_viewer:*", "delete:followers_owner:creator", "touch:public_viewer:*"}
	if !slices.Equal(operations, want) {
		t.Fatalf("authorization operations=%v want=%v", operations, want)
	}
}

func TestSetVisibilityFailsWhileAuthorizationIsPendingAndReplayCompletesIt(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	existing := domain.Entity{ID: "path-1", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Read", Visibility: "public"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	updated, _ := existing.SetVisibility("private", now)
	change := ports.AuthorizationChange{ID: "delete-public", ResourceType: "path", ResourceID: "path-1", Relation: "public_viewer", SubjectType: "user", SubjectID: "*", OwnerUserID: "creator", ActorUserID: "creator", Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute}
	dependencies := func() Dependencies {
		d := configuredDependencies()
		d.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "creator", Scopes: []string{"api:user"}}}
		d.Authorizer = controlledAuthorizer{allowed: true}
		d.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPublic, firstDayOfWeek: identity.FirstDayMonday}
		d.Clock = controlledClock{now: now}
		return d
	}
	first := dependencies()
	first.AuthorizationOutbox = controlledOutbox{claimErr: ports.ErrAuthorizationPending}
	first.Repository = controlledRepository{entity: existing, visibilityResult: SetVisibilityResult{Path: updated, AuthorizationChanges: []ports.AuthorizationChange{change}}}
	if _, err := New(first).SetVisibility(context.Background(), "Bearer valid", "visibility-key-retry", "path-1", true, "public", "private"); !errors.Is(err, ports.ErrAuthorizationPending) {
		t.Fatalf("first err=%v", err)
	}
	queue, claimIndex := []ports.AuthorizationChange{change}, 0
	var deletes []relationshipWrite
	replay := dependencies()
	replay.Authorizer = controlledAuthorizer{allowed: true, deletes: &deletes}
	replay.AuthorizationOutbox = controlledOutbox{changes: &queue, claimIndex: &claimIndex}
	replay.Repository = controlledRepository{visibilityReplay: &SetVisibilityResult{Path: updated, Replayed: true}}
	result, err := New(replay).SetVisibility(context.Background(), "Bearer valid", "visibility-key-retry", "path-1", true, "public", "private")
	if err != nil || !result.Replayed || len(deletes) != 1 || deletes[0].relation != "public_viewer" {
		t.Fatalf("replay=%+v deletes=%+v err=%v", result, deletes, err)
	}
}
