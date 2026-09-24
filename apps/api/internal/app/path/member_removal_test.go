package path

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type removalRepository struct {
	review       MemberRemovalReview
	page         MemberPage
	pages        *[]MemberPageRequest
	replayed     bool
	result       RemoveMemberResult
	commands     *[]RemoveMemberCommand
	err          error
	roleReplayed bool
	roleResult   ChangeMemberRoleResult
	roleCommands *[]ChangeMemberRoleCommand
}

func (r removalRepository) ChangeMemberRoleReplay(context.Context, string, domain.ID, string, ports.Idempotency) (bool, error) {
	return r.roleReplayed, r.err
}
func (r removalRepository) ChangeMemberRole(_ context.Context, command ChangeMemberRoleCommand) (ChangeMemberRoleResult, error) {
	if r.roleCommands != nil {
		*r.roleCommands = append(*r.roleCommands, command)
	}
	return r.roleResult, r.err
}

func (r removalRepository) ListMembers(context.Context, MemberListQuery, MemberPageRequest) (MemberPage, error) {
	page := r.page
	if page.ProjectionFingerprint == "" {
		page.ProjectionFingerprint = strings.Repeat("a", 64)
	}
	return page, r.err
}

func (r removalRepository) ReviewMemberRemoval(context.Context, MemberRemovalQuery) (MemberRemovalReview, error) {
	return r.review, r.err
}
func (r removalRepository) RemoveMemberReplay(context.Context, string, domain.ID, string, ports.Idempotency) (bool, error) {
	return r.replayed, r.err
}
func (r removalRepository) RemoveMember(_ context.Context, command RemoveMemberCommand) (RemoveMemberResult, error) {
	if r.commands != nil {
		*r.commands = append(*r.commands, command)
	}
	return r.result, r.err
}

func TestMemberRemovalReviewIsAuthorizedAndSideEffectFree(t *testing.T) {
	now := time.Date(2026, 7, 29, 16, 0, 0, 0, time.UTC)
	want := MemberRemovalReview{UserID: "target", Username: "reader", DisplayName: "Reader", Role: domain.RoleParticipant, SessionCount: 3, TotalTrackedSeconds: 7200, RunningTimer: true}
	var checks []authorizationCall
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	dependencies.MemberRemoval = removalRepository{review: want}
	got, err := New(dependencies).ReviewMemberRemoval(context.Background(), "Bearer valid", "path-1", "target")
	if err != nil || got != want {
		t.Fatalf("ReviewMemberRemoval() = %+v, %v", got, err)
	}
	if len(checks) != 1 || checks[0] != (authorizationCall{"path", "path-1", "manage_members", "member"}) {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestListMembersUsesPathBoundSignedCursorAndAuthorization(t *testing.T) {
	now := time.Date(2026, 7, 29, 16, 0, 0, 0, time.UTC)
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.Authorizer = controlledAuthorizer{allowed: true}
	dependencies.CursorSigningKey = []byte("01234567890123456789012345678901")
	dependencies.MemberRemoval = removalRepository{page: MemberPage{Items: []Member{{UserID: "target", Username: "reader", DisplayName: "Reader", Role: "participant", SessionCount: 3, TotalTrackedSeconds: 7200, BlockedByViewer: true, CanRemove: true, CreatedAt: now.Add(-time.Hour)}}, HasMore: true}}
	items, cursor, err := New(dependencies).ListMembers(context.Background(), "Bearer valid", "path-1", "", 1)
	if err != nil || len(items) != 1 || cursor == "" {
		t.Fatalf("ListMembers()=%+v cursor=%q err=%v", items, cursor, err)
	}
	if !items[0].BlockedByViewer || !items[0].CanRemove || items[0].SessionCount != 3 || items[0].TotalTrackedSeconds != 7200 {
		t.Fatalf("minimum Path-scoped member projection=%+v", items[0])
	}
	if got := dependencies.Authorizer.(controlledAuthorizer); got.allowed != true {
		t.Fatalf("authorizer=%+v", got)
	}
	if _, _, err := New(dependencies).ListMembers(context.Background(), "Bearer valid", "path-2", cursor, 1); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("cross-Path cursor error=%v", err)
	}
	dependencies.Authorizer = controlledAuthorizer{allowed: false}
	if _, _, err := New(dependencies).ListMembers(context.Background(), "Bearer valid", "path-1", "", 25); !errors.Is(err, platformapp.ErrForbidden) {
		t.Fatalf("denied error=%v", err)
	}
}

func TestListMembersRequiresPathViewRatherThanManagementAuthority(t *testing.T) {
	now := time.Date(2026, 7, 29, 16, 0, 0, 0, time.UTC)
	var checks []authorizationCall
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	dependencies.CursorSigningKey = []byte("01234567890123456789012345678901")
	dependencies.MemberRemoval = removalRepository{page: MemberPage{Items: []Member{{UserID: "member", Username: "member", DisplayName: "Member", Role: "participant", CanLeave: true, CreatedAt: now.Add(-time.Hour)}}}}
	items, _, err := New(dependencies).ListMembers(context.Background(), "Bearer valid", "path-1", "", 25)
	if err != nil || len(items) != 1 || !items[0].CanLeave {
		t.Fatalf("ordinary member People projection=%+v err=%v", items, err)
	}
	if len(checks) != 1 || checks[0] != (authorizationCall{"path", "path-1", "view", "member"}) {
		t.Fatalf("checks=%+v", checks)
	}
}

func TestListMembersReturnsOptionalGoalProgressAndRejectsMalformedProjection(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	valid := Member{
		UserID: "member", Username: "member", DisplayName: "Member", Role: "participant",
		SessionCount: 2, TotalTrackedSeconds: 900,
		IntervalProgress: &GoalProgress{AccumulatedSeconds: 300, TargetSeconds: 600},
		OverallProgress:  &GoalProgress{AccumulatedSeconds: 900, TargetSeconds: 3600},
		CreatedAt:        now.Add(-time.Hour),
	}
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.Authorizer = controlledAuthorizer{allowed: true}
	dependencies.CursorSigningKey = []byte("01234567890123456789012345678901")
	dependencies.MemberRemoval = removalRepository{page: MemberPage{Items: []Member{valid}}}
	items, _, err := New(dependencies).ListMembers(context.Background(), "Bearer valid", "path-1", "", 25)
	if err != nil || len(items) != 1 || items[0].IntervalProgress == nil || *items[0].IntervalProgress != (GoalProgress{AccumulatedSeconds: 300, TargetSeconds: 600}) || items[0].OverallProgress == nil || *items[0].OverallProgress != (GoalProgress{AccumulatedSeconds: 900, TargetSeconds: 3600}) {
		t.Fatalf("progress projection=%+v err=%v", items, err)
	}

	for name, mutate := range map[string]func(*Member){
		"negative interval":               func(member *Member) { member.IntervalProgress.AccumulatedSeconds = -1 },
		"interval exceeds Path total":     func(member *Member) { member.IntervalProgress.AccumulatedSeconds = 901 },
		"missing interval target":         func(member *Member) { member.IntervalProgress.TargetSeconds = 0 },
		"overall differs from Path total": func(member *Member) { member.OverallProgress.AccumulatedSeconds = 899 },
		"supporter exposes goal progress": func(member *Member) { member.Role = "supporter" },
	} {
		t.Run(name, func(t *testing.T) {
			malformed := valid
			interval, overall := *valid.IntervalProgress, *valid.OverallProgress
			malformed.IntervalProgress, malformed.OverallProgress = &interval, &overall
			mutate(&malformed)
			deps := configuredDependencies()
			deps.Clock = controlledClock{now: now}
			deps.Authorizer = controlledAuthorizer{allowed: true}
			deps.CursorSigningKey = []byte("01234567890123456789012345678901")
			deps.MemberRemoval = removalRepository{page: MemberPage{Items: []Member{malformed}}}
			if _, _, err := New(deps).ListMembers(context.Background(), "Bearer valid", "path-1", "", 25); !errors.Is(err, errInvalidPathDependencies) {
				t.Fatalf("malformed projection error=%v", err)
			}
		})
	}
}

func TestListMembersRejectsAContinuationWhenTheProgressProjectionChanges(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	member := Member{UserID: "member", Username: "member", DisplayName: "Member", Role: "participant", CreatedAt: now.Add(-time.Hour)}
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.Authorizer = controlledAuthorizer{allowed: true}
	dependencies.CursorSigningKey = []byte("01234567890123456789012345678901")
	dependencies.MemberRemoval = removalRepository{page: MemberPage{Items: []Member{member}, ProjectionFingerprint: strings.Repeat("a", 64), HasMore: true}}
	_, cursor, err := New(dependencies).ListMembers(context.Background(), "Bearer valid", "path-1", "", 1)
	if err != nil || cursor == "" {
		t.Fatalf("first People page cursor=%q err=%v", cursor, err)
	}
	dependencies.MemberRemoval = removalRepository{page: MemberPage{Items: []Member{member}, ProjectionFingerprint: strings.Repeat("b", 64)}}
	if _, _, err := New(dependencies).ListMembers(context.Background(), "Bearer valid", "path-1", cursor, 1); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("changed progress projection error=%v", err)
	}
}

func TestRemoveMemberBindsRoleAndReconcilesAuthorization(t *testing.T) {
	now := time.Date(2026, 7, 29, 16, 0, 0, 123456789, time.UTC)
	change := ports.AuthorizationChange{ID: "auth-delete", ResourceType: "path", ResourceID: "path-1", Relation: "participant", SubjectType: "user", SubjectID: "target", OwnerUserID: "creator", ActorUserID: "member", Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute}
	var commands []RemoveMemberCommand
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.Authorizer = controlledAuthorizer{allowed: true}
	dependencies.MemberRemoval = removalRepository{commands: &commands, result: RemoveMemberResult{PathID: "path-1", UserID: "target", Removed: true, ActivityDeleted: true, AuthorizationChanges: []ports.AuthorizationChange{change}}}
	dependencies.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{change}}
	result, err := New(dependencies).RemoveMember(context.Background(), "Bearer valid", "remove-member-key-0001", "path-1", "target", true, domain.RoleParticipant)
	if err != nil || !result.Removed || !result.ActivityDeleted || len(commands) != 1 {
		t.Fatalf("RemoveMember() = %+v, %v commands=%+v", result, err, commands)
	}
	command := commands[0]
	if command.ExpectedRole != domain.RoleParticipant || command.ActorUserID != "member" || command.TargetUserID != "target" || command.RemovedAt != now.Truncate(time.Microsecond) || command.Idempotency.Operation != RemoveMemberOperation || len(command.Idempotency.RequestHash) != 32 || command.Notification.Kind != NotificationPathMemberRemoved || command.Notification.RecipientUserID != "target" || command.Notification.Role != domain.RoleParticipant {
		t.Fatalf("command = %+v", command)
	}
}

func TestRemoveMemberFailsClosedAndReplayPrecedesAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name               string
		confirmed, allowed bool
		role               domain.MembershipRole
		want               error
	}{
		{"unconfirmed", false, true, domain.RoleParticipant, ports.ErrInvalidArgument},
		{"invalid role", true, true, "administrator", ports.ErrInvalidArgument},
		{"denied", true, false, domain.RoleParticipant, platformapp.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var commands []RemoveMemberCommand
			dependencies := configuredDependencies()
			dependencies.Authorizer = controlledAuthorizer{allowed: tc.allowed}
			dependencies.MemberRemoval = removalRepository{commands: &commands}
			_, err := New(dependencies).RemoveMember(context.Background(), "Bearer valid", "remove-member-key-0001", "path-1", "target", tc.confirmed, tc.role)
			if !errors.Is(err, tc.want) || len(commands) != 0 {
				t.Fatalf("error=%v commands=%+v", err, commands)
			}
		})
	}
	var checks []authorizationCall
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{calls: &checks}
	dependencies.MemberRemoval = removalRepository{replayed: true}
	result, err := New(dependencies).RemoveMember(context.Background(), "Bearer valid", "remove-member-key-0001", "path-1", "target", true, domain.RoleParticipant)
	if err != nil || !result.Replayed || len(checks) != 0 {
		t.Fatalf("replay=%+v err=%v checks=%+v", result, err, checks)
	}
}

func TestChangeMemberRoleBindsExpectedRoleAndReconcilesBothRelationships(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 123456789, time.UTC)
	deleted := ports.AuthorizationChange{ID: "auth-delete", ResourceType: "path", ResourceID: "path-1", Relation: "participant", SubjectType: "user", SubjectID: "target", OwnerUserID: "creator", ActorUserID: "member", Operation: ports.AuthorizationDelete, LockedBy: "path-api", Lease: time.Minute}
	added := ports.AuthorizationChange{ID: "auth-add", ResourceType: "path", ResourceID: "path-1", Relation: "supporter", SubjectType: "user", SubjectID: "target", OwnerUserID: "creator", ActorUserID: "member", Operation: ports.AuthorizationTouch, LockedBy: "path-api", Lease: time.Minute}
	var commands []ChangeMemberRoleCommand
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.Authorizer = controlledAuthorizer{allowed: true}
	dependencies.MemberRemoval = removalRepository{roleCommands: &commands, roleResult: ChangeMemberRoleResult{PathID: "path-1", UserID: "target", Role: domain.RoleSupporter, ActivityDeleted: true, AuthorizationChanges: []ports.AuthorizationChange{deleted, added}}}
	dependencies.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{deleted, added}}
	result, err := New(dependencies).ChangeMemberRole(context.Background(), "Bearer valid", "change-member-role-0001", "path-1", "target", true, domain.RoleParticipant, domain.RoleSupporter)
	if err != nil || result.Role != domain.RoleSupporter || !result.ActivityDeleted || len(commands) != 1 {
		t.Fatalf("ChangeMemberRole()=%+v err=%v commands=%+v", result, err, commands)
	}
	command := commands[0]
	if command.ExpectedRole != domain.RoleParticipant || command.Role != domain.RoleSupporter || command.ChangedAt != now.Truncate(time.Microsecond) || command.Idempotency.Operation != ChangeMemberRoleOperation || command.Notification.Kind != NotificationPathMemberRoleChanged || command.Notification.RecipientUserID != "target" {
		t.Fatalf("command=%+v", command)
	}
}

func TestChangeMemberRoleRejectsNoopUnconfirmedAndUnauthorizedMutations(t *testing.T) {
	for _, tc := range []struct {
		name               string
		confirmed, allowed bool
		from, to           domain.MembershipRole
		want               error
	}{
		{"unconfirmed", false, true, domain.RoleParticipant, domain.RoleSupporter, ports.ErrInvalidArgument},
		{"noop", true, true, domain.RoleParticipant, domain.RoleParticipant, ports.ErrInvalidArgument},
		{"administrator target", true, true, "administrator", domain.RoleSupporter, ports.ErrInvalidArgument},
		{"denied", true, false, domain.RoleParticipant, domain.RoleSupporter, platformapp.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var commands []ChangeMemberRoleCommand
			dependencies := configuredDependencies()
			dependencies.Authorizer = controlledAuthorizer{allowed: tc.allowed}
			dependencies.MemberRemoval = removalRepository{roleCommands: &commands}
			_, err := New(dependencies).ChangeMemberRole(context.Background(), "Bearer valid", "change-member-role-0001", "path-1", "target", tc.confirmed, tc.from, tc.to)
			if !errors.Is(err, tc.want) || len(commands) != 0 {
				t.Fatalf("error=%v commands=%+v", err, commands)
			}
		})
	}
}

func TestAdministratorLifecycleUsesCreatorOnlyAndSelfStepDownPermissions(t *testing.T) {
	now := time.Date(2026, 8, 3, 15, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, actor, target, permission string
		from, to                        domain.MembershipRole
	}{
		{"creator grant", "member", "target", "manage_administrators", domain.RoleParticipant, domain.RoleAdministrator},
		{"creator revoke", "member", "target", "manage_administrators", domain.RoleAdministrator, domain.RoleParticipant},
		{"administrator self step down", "member", "member", "step_down_administrator", domain.RoleAdministrator, domain.RoleParticipant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleted := ports.AuthorizationChange{ID: "delete", ResourceType: "path", ResourceID: "path-1", Relation: string(tc.from), SubjectType: "user", SubjectID: tc.target, OwnerUserID: "creator", ActorUserID: tc.actor, Operation: ports.AuthorizationDelete, LockedBy: "worker", Lease: time.Minute}
			added := ports.AuthorizationChange{ID: "touch", ResourceType: "path", ResourceID: "path-1", Relation: string(tc.to), SubjectType: "user", SubjectID: tc.target, OwnerUserID: "creator", ActorUserID: tc.actor, Operation: ports.AuthorizationTouch, LockedBy: "worker", Lease: time.Minute}
			var checks []authorizationCall
			var commands []ChangeMemberRoleCommand
			dependencies := configuredDependencies()
			dependencies.Clock = controlledClock{now: now}
			dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
			dependencies.MemberRemoval = removalRepository{roleCommands: &commands, roleResult: ChangeMemberRoleResult{PathID: "path-1", UserID: tc.target, Role: tc.to, AuthorizationChanges: []ports.AuthorizationChange{deleted, added}}}
			dependencies.AuthorizationOutbox = &orderedDeletionOutbox{changes: []ports.AuthorizationChange{deleted, added}}
			result, err := New(dependencies).ChangeMemberRole(context.Background(), "Bearer valid", "administrator-role-0001", "path-1", tc.target, true, tc.from, tc.to)
			if err != nil || result.ActivityDeleted || len(commands) != 1 {
				t.Fatalf("ChangeMemberRole()=%+v err=%v commands=%+v", result, err, commands)
			}
			if len(checks) != 1 || checks[0] != (authorizationCall{"path", "path-1", tc.permission, tc.actor}) {
				t.Fatalf("authorization checks=%+v", checks)
			}
			if commands[0].Notification.RecipientUserID != tc.target || commands[0].Notification.Role != tc.to {
				t.Fatalf("notification=%+v", commands[0].Notification)
			}
		})
	}
}

func TestAdministratorLifecycleRejectsEveryUnapprovedTransitionBeforePersistence(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		from, to     domain.MembershipRole
	}{
		{"self grant", "member", domain.RoleParticipant, domain.RoleAdministrator},
		{"self ordinary role change", "member", domain.RoleParticipant, domain.RoleSupporter},
		{"administrator to supporter", "target", domain.RoleAdministrator, domain.RoleSupporter},
		{"supporter to administrator", "target", domain.RoleSupporter, domain.RoleAdministrator},
		{"administrator self to supporter", "member", domain.RoleAdministrator, domain.RoleSupporter},
		{"creator pseudo role", "target", "creator", domain.RoleParticipant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var commands []ChangeMemberRoleCommand
			dependencies := configuredDependencies()
			dependencies.Authorizer = controlledAuthorizer{allowed: true}
			dependencies.MemberRemoval = removalRepository{roleCommands: &commands}
			_, err := New(dependencies).ChangeMemberRole(context.Background(), "Bearer valid", "administrator-role-0001", "path-1", tc.target, true, tc.from, tc.to)
			if !errors.Is(err, ports.ErrInvalidArgument) || len(commands) != 0 {
				t.Fatalf("err=%v commands=%+v", err, commands)
			}
		})
	}
}
