package gormstore

import (
	"bytes"
	"context"
	"flag"
	spice "github.com/elsell/hour-paths/apps/api/internal/adapters/spicedb"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

var deletionSpiceEndpoint = flag.String("deletion-spicedb-endpoint", "", "runtime proxy endpoint for isolated deletion fixture")
var deletionSpiceToken = flag.String("deletion-spicedb-token", "", "runtime proxy token for isolated deletion fixture")

type deletionTestClock struct{ now time.Time }

func (c deletionTestClock) Now() time.Time { return c.now }

func TestPostgresDeletionAuthorizationConvergesAgainstRuntimeProxy(t *testing.T) {
	if *deletionSpiceEndpoint == "" || *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("explicit database and runtime proxy required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := spice.New(*deletionSpiceEndpoint, *deletionSpiceToken, true)
	if err != nil {
		t.Fatal(err)
	}
	owner, resourceID := newTestID(), newTestID()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err = seed.DB.Create(&userModel{ID: owner, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err = seed.DB.Create(&resourceModel{ID: resourceID, Domain: "example", OwnerUserID: owner, Name: "deletion fixture", CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	target := "example/" + resourceID
	if err = authorizer.WriteRelationship(ctx, "resource", target, "owner", "user", owner); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = authorizer.DeleteRelationship(cleanup, "resource", target, "owner", "user", owner)
	})
	if allowed, err := authorizer.Check(ctx, "resource", target, "view", owner); err != nil || !allowed {
		t.Fatalf("fixture permission: %t %v", allowed, err)
	}
	survivor, owned, joined := newTestID(), newTestID(), newTestID()
	if err = seed.DB.Create(&userModel{ID: survivor, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	for pathID, creator := range map[string]string{owned: owner, joined: survivor} {
		if err = seed.DB.Table("path_models").Create(map[string]any{"id": pathID, "owner_user_id": creator, "name": "permission fixture", "visibility": "private", "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		for _, participant := range []string{owner, survivor} {
			if err = seed.DB.Table("path_membership_models").Create(map[string]any{"path_id": pathID, "user_id": participant, "role": "participant"}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	relationships := []struct{ path, role, user string }{{owned, "creator", owner}, {owned, "participant", survivor}, {joined, "creator", survivor}, {joined, "participant", owner}}
	for _, rel := range relationships {
		if err = authorizer.WriteRelationship(ctx, "path", rel.path, rel.role, "user", rel.user); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = authorizer.DeleteRelationship(cleanup, "path", rel.path, rel.role, "user", rel.user)
		})
	}
	// A queued grant predating deletion must be followed by a removal, not revive access.
	if err = seed.DB.Create(&authorizationOutboxModel{ID: newTestID(), ResourceType: "path", ResourceID: joined, Relation: "participant", SubjectType: "user", SubjectID: owner, OwnerUserID: owner, ActorUserID: owner, Operation: ports.AuthorizationTouch, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	event := audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceDeleted, TargetType: "account", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
	if err = store.DeleteAccount(ctx, application.AccountDeletionCommand{UserID: owner, DeletedAt: now, Audit: event, NewID: newTestID, ReceiptHash: bytes.Repeat([]byte{1}, 32)}); err != nil {
		t.Fatal(err)
	}
	service := application.App{Authorizer: authorizer, AuthorizationOutbox: store, AuthorizationSerializer: store, Audits: store, Clock: deletionTestClock{now}}
	var queued []authorizationOutboxModel
	if err = seed.DB.Where("owner_user_id = ? AND completed_at IS NULL", owner).Order("created_at,id").Find(&queued).Error; err != nil {
		t.Fatal(err)
	}
	if len(queued) == 0 {
		t.Fatal("no authorization cleanup queued")
	}
	for _, change := range queued {
		if err = service.ReconcileAuthorizationChange(ctx, change.ID, newTestID()); err != nil {
			t.Fatal(err)
		}
	}
	if allowed, err := authorizer.Check(ctx, "resource", target, "view", owner); err != nil || allowed {
		t.Fatalf("deleted owner retains permission: %t %v", allowed, err)
	}
	for _, check := range []struct {
		path, user string
		allowed    bool
	}{{owned, owner, false}, {owned, survivor, false}, {joined, owner, false}, {joined, survivor, true}} {
		allowed, err := authorizer.Check(ctx, "path", check.path, "view", check.user)
		if err != nil || allowed != check.allowed {
			t.Fatalf("path permission after deletion: got %t want %t error %v", allowed, check.allowed, err)
		}
	}

}
