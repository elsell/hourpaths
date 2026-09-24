package path

import (
	"context"
	"testing"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestCreatePathReplayReconcilesPendingFollowersOwnerRelationship(t *testing.T) {
	stored := domain.Entity{ID: "original-path", OwnerUserID: "member", Attributes: domain.Attributes{Name: "Read", Visibility: "followers"}, CreatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)}
	change := ports.AuthorizationChange{ID: "followers-owner-change", ResourceType: "path", ResourceID: "original-path", Relation: "followers_owner", SubjectType: "user", SubjectID: "member", OwnerUserID: "member", ActorUserID: "member", Operation: ports.AuthorizationTouch}
	var writes []relationshipWrite
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{entity: stored, replayed: true}
	dependencies.Authorizer = controlledAuthorizer{writes: &writes}
	dependencies.AuthorizationOutbox = controlledOutbox{change: change}
	created, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read"})
	if err != nil || created != stored {
		t.Fatalf("replay = %+v, %v", created, err)
	}
	if len(writes) != 1 || writes[0] != (relationshipWrite{"path", "original-path", "followers_owner", "user", "member"}) {
		t.Fatalf("followers_owner reconciliation writes=%+v", writes)
	}
}

func TestCreatePathReplayReconcilesPendingPublicViewerRelationship(t *testing.T) {
	stored := domain.Entity{ID: "public-path", OwnerUserID: "member", Attributes: domain.Attributes{Name: "Read", Visibility: "public"}, CreatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)}
	change := ports.AuthorizationChange{ID: "public-viewer-change", ResourceType: "path", ResourceID: "public-path", Relation: "public_viewer", SubjectType: "user", SubjectID: "*", OwnerUserID: "member", ActorUserID: "member", Operation: ports.AuthorizationTouch}
	var writes []relationshipWrite
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{entity: stored, replayed: true}
	dependencies.Authorizer = controlledAuthorizer{writes: &writes}
	dependencies.AuthorizationOutbox = controlledOutbox{change: change}
	created, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read"})
	if err != nil || created != stored {
		t.Fatalf("replay = %+v, %v", created, err)
	}
	if len(writes) != 1 || writes[0] != (relationshipWrite{"path", "public-path", "public_viewer", "user", "*"}) {
		t.Fatalf("public_viewer reconciliation writes=%+v", writes)
	}
}
