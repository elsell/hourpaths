package path

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

type projectionAuthorizer struct {
	controlledAuthorizer
	permissions map[string]bool
	failOn      string
	failure     error
}

func (a projectionAuthorizer) Check(ctx context.Context, resourceType, resourceID, permission, userID string) (bool, error) {
	if a.calls != nil {
		*a.calls = append(*a.calls, authorizationCall{resourceType, resourceID, permission, userID})
	}
	if permission == a.failOn {
		return false, a.failure
	}
	return a.permissions[permission], nil
}

func TestGetPathProjectionUsesAuthoritativeRoleCapabilities(t *testing.T) {
	tests := []struct {
		name        string
		permissions map[string]bool
		want        Capabilities
	}{
		{
			name:        "creator",
			permissions: map[string]bool{"view": true, "track": true, "rename": true, "manage_members": true, "manage_goals": true, "manage_lifecycle": true, "manage_visibility": true, "transfer_ownership": true},
			want:        Capabilities{TrackTime: true, RenamePath: true, InviteMembers: true, ManageMembers: true, ManageGoals: true, ManageLifecycle: true, ManageVisibility: true, TransferOwnership: true},
		},
		{
			name:        "administrator",
			permissions: map[string]bool{"view": true, "track": true, "rename": true, "manage_members": true, "manage_goals": true, "leave": true},
			want:        Capabilities{TrackTime: true, RenamePath: true, InviteMembers: true, ManageMembers: true, ManageGoals: true, LeavePath: true},
		},
		{
			name:        "participant",
			permissions: map[string]bool{"view": true, "track": true, "leave": true},
			want:        Capabilities{TrackTime: true, LeavePath: true},
		},
		{
			name:        "supporter",
			permissions: map[string]bool{"view": true, "leave": true},
			want:        Capabilities{LeavePath: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var checks []authorizationCall
			dependencies := configuredDependencies()
			dependencies.Authorizer = projectionAuthorizer{
				controlledAuthorizer: controlledAuthorizer{calls: &checks},
				permissions:          test.permissions,
			}
			projection, err := New(dependencies).GetProjected(context.Background(), "Bearer valid", "path-id")
			if err != nil {
				t.Fatal(err)
			}
			if projection.Path.ID != "path-id" || projection.Capabilities != test.want {
				t.Fatalf("projection = %+v, want capabilities %+v", projection, test.want)
			}
			wantChecks := []authorizationCall{
				{"path", "path-id", "view", "member"},
				{"path", "path-id", "track", "member"},
				{"path", "path-id", "rename", "member"},
				{"path", "path-id", "manage_members", "member"},
				{"path", "path-id", "manage_goals", "member"},
				{"path", "path-id", "manage_lifecycle", "member"},
				{"path", "path-id", "manage_visibility", "member"},
				{"path", "path-id", "transfer_ownership", "member"},
				{"path", "path-id", "leave", "member"},
			}
			if len(checks) != len(wantChecks) {
				t.Fatalf("authorization checks = %+v", checks)
			}
			for index := range wantChecks {
				if checks[index] != wantChecks[index] {
					t.Fatalf("authorization checks = %+v", checks)
				}
			}
		})
	}
}

func TestArchivedPathProjectionIsReadOnlyExceptCreatorLifecycle(t *testing.T) {
	archivedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{entity: domain.Entity{
		ID:          "path-id",
		OwnerUserID: "creator",
		ArchivedAt:  archivedAt,
		Attributes:  domain.Attributes{Name: "Guitar", Visibility: "private"},
	}}
	dependencies.Authorizer = projectionAuthorizer{permissions: map[string]bool{
		"view": true, "track": true, "rename": true, "manage_members": true, "manage_goals": true, "manage_lifecycle": true, "transfer_ownership": true,
	}}
	projection, err := New(dependencies).GetProjected(context.Background(), "Bearer valid", "path-id")
	if err != nil {
		t.Fatal(err)
	}
	want := Capabilities{ManageLifecycle: true}
	if projection.Capabilities != want {
		t.Fatalf("archived capabilities = %+v, want %+v", projection.Capabilities, want)
	}
}

func TestListPathProjectionFailsClosedWhenCapabilityCheckFails(t *testing.T) {
	failure := errors.New("SpiceDB unavailable")
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{page: Page{Items: []domain.Entity{{
		ID: "path-id", OwnerUserID: "creator", CreatedAt: dependencies.Clock.Now(),
	}}}}
	dependencies.Authorizer = projectionAuthorizer{
		permissions: map[string]bool{"view": true, "track": true},
		failOn:      "manage_members",
		failure:     failure,
	}
	if _, _, err := New(dependencies).ListProjected(context.Background(), "Bearer valid", "", 25); !errors.Is(err, failure) {
		t.Fatalf("capability dependency failure = %v", err)
	}
}
