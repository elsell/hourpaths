package gormstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestResourceClassifiesConflictingIdempotencyReplay(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	tx := store.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	txStore := &Store{DB: tx}
	now := time.Now().UTC().Truncate(time.Microsecond)
	ownerID := "resource-owner-" + newTestID()
	if err := tx.Create(&userModel{ID: ownerID, Email: ownerID + "@example.com", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	resource := ports.Resource{ID: newTestID(), Domain: "example", OwnerUserID: ownerID, Name: "first request", CreatedAt: now}
	change := ports.AuthorizationChange{ID: newTestID(), ResourceType: "resource", ResourceID: "example/" + resource.ID, Relation: "owner", SubjectType: "user", SubjectID: ownerID, OwnerUserID: ownerID, ActorUserID: ownerID, Operation: ports.AuthorizationTouch, LockedBy: newTestID(), Lease: time.Minute}
	event := audit.Event{ID: newTestID(), OwnerUserID: ownerID, ActorUserID: ownerID, Action: audit.ResourceCreated, TargetType: resource.Domain, TargetID: resource.ID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
	idempotency := ports.Idempotency{PrincipalID: ownerID, Operation: "resource.create:example", Key: "resource-idempotency-key", RequestHash: make([]byte, 32)}
	if _, replayed, err := txStore.CreateResource(context.Background(), resource, change, event, idempotency); err != nil || replayed {
		t.Fatalf("initial resource create failed: replayed=%v err=%v", replayed, err)
	}
	secondHash := make([]byte, 32)
	secondHash[0] = 1
	idempotency.RequestHash = secondHash
	if _, _, err := txStore.CreateResource(context.Background(), resource, change, event, idempotency); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("conflicting idempotency replay returned %v, want ErrIdempotencyConflict", err)
	}
}
