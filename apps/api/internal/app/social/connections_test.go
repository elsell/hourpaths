package social

import (
	"context"
	"errors"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type connectionFake struct {
	controlledRelationships
	page  ConnectionPage
	reads int
}

func (r *connectionFake) ListConnections(context.Context, string, string, ConnectionDirection, ports.PageRequest) (ConnectionPage, error) {
	r.reads++
	return r.page, r.err
}
func (r *connectionFake) RemoveFollower(_ context.Context, c RelationshipCommand) (RelationshipResult, error) {
	r.capture(RemoveFollowerOperation, c)
	return r.result, r.err
}

type connectionAuthorization struct {
	controlledRelationshipAuthorizer
	allow    bool
	checkErr error
}

func (a connectionAuthorization) Check(_ context.Context, resource, id, permission, viewer string) (bool, error) {
	return a.allow && resource == "user" && id == "owner" && permission == "follower" && viewer == "viewer", a.checkErr
}

func TestConnectionCursorCannotCrossViewerProfileOrDirection(t *testing.T) {
	repo := &connectionFake{page: ConnectionPage{Profiles: []domain.PublicProfile{relationshipProfile("person", "person", domain.RelationshipNone)}, HasMore: true, LastID: "person", LastCreated: time.Date(2026, 7, 27, 11, 0, 0, 0, time.UTC)}}
	profiles := &controlledProfiles{profile: relationshipProfile("owner", "owner", domain.RelationshipFollowing)}
	service := testService(profiles, &controlledAudits{})
	service.Relationships = repo
	service.Authorizer = connectionAuthorization{allow: true}
	_, cursor, err := service.ListConnections(context.Background(), "session", "owner", Followers, "", 1)
	if err != nil || cursor == "" {
		t.Fatalf("first page: %q %v", cursor, err)
	}
	for _, change := range []string{"direction", "profile", "viewer", "signature"} {
		t.Run(change, func(t *testing.T) {
			s := *service
			p := *profiles
			s.Profiles = &p
			direction := Followers
			value := cursor
			switch change {
			case "direction":
				direction = Following
			case "profile":
				p.profile.ID = "another"
				s.Authorizer = controlledRelationshipAuthorizer{}
			case "viewer":
				s.Auth = controlledAuth{principal: ports.Principal{UserID: "another", Scopes: []string{"api:user"}}}
				s.Authorizer = controlledRelationshipAuthorizer{}
			case "signature":
				value = "x" + cursor
			}
			before := repo.reads
			if _, _, err := s.ListConnections(context.Background(), "session", "owner", direction, value, 1); !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("cross-scope cursor: %v", err)
			}
			if repo.reads != before {
				t.Fatal("invalid cursor reached identities")
			}
		})
	}
	service.Authorizer = connectionAuthorization{allow: false}
	if _, _, err = service.ListConnections(context.Background(), "session", "owner", Followers, cursor, 1); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("revoked follower: %v", err)
	}
	service.Authorizer = connectionAuthorization{checkErr: ports.ErrUnavailable}
	if _, _, err = service.ListConnections(context.Background(), "session", "owner", Followers, "", 1); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("dependency failure: %v", err)
	}
	service.Auth = controlledAuth{err: platformapp.ErrUnauthenticated}
	if _, _, err = service.ListConnections(context.Background(), "invalid", "owner", Followers, "", 1); !errors.Is(err, platformapp.ErrUnauthenticated) {
		t.Fatalf("authentication: %v", err)
	}
}

func TestRemoveFollowerOnlyReconcilesInboundGrant(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "inbound", true: "forged reverse"}[reverse], func(t *testing.T) {
			change := followerAuthorization("delete", "viewer", "follower", "viewer", ports.AuthorizationDelete)
			if reverse {
				change = followerAuthorization("delete", "follower", "viewer", "viewer", ports.AuthorizationDelete)
			}
			base := controlledRelationships{result: RelationshipResult{Target: relationshipProfile("follower", "follower", domain.RelationshipFollowing), Changed: true, AuthorizationChange: change}}
			repo := &connectionFake{controlledRelationships: base}
			service := relationshipService(&repo.controlledRelationships, &controlledRelationshipLimiter{allowed: true}, &controlledAudits{})
			service.Relationships = repo
			var deleted []ports.AuthorizationChange
			service.Authorizer = controlledRelationshipAuthorizer{deletes: &deleted}
			_, err := service.RemoveFollower(context.Background(), "session", "follower", "remove-follower-key")
			if reverse {
				if err == nil || len(deleted) != 0 {
					t.Fatal("forged reverse relationship applied")
				}
				return
			}
			if err != nil || len(deleted) != 1 || deleted[0].ResourceID != "viewer" || deleted[0].SubjectID != "follower" {
				t.Fatalf("inbound deletion: %+v %v", deleted, err)
			}
		})
	}
}
