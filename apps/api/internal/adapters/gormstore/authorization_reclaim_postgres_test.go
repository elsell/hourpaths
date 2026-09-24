package gormstore

import (
	"context"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestClaimAuthorizationChangeForResourceRenewsItsExistingLease(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ownerID, resourceID, changeID := "lease-owner-"+newTestID(), "lease-resource-"+newTestID(), "lease-change-"+newTestID()
	now := time.Now().UTC()
	if err := store.DB.WithContext(ctx).Create(&userModel{ID: ownerID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "path", resourceID).Delete(&authorizationOutboxModel{}).Error
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "path", resourceID).Delete(&authorizationResourceLockModel{}).Error
		_ = store.DB.WithContext(ctx).Where("id = ?", ownerID).Delete(&userModel{}).Error
	})
	lockedBy := "path-api"
	lockedUntil := now.Add(time.Minute)
	row := authorizationOutboxModel{
		ID: changeID, ResourceType: "path", ResourceID: resourceID, Relation: "creator", SubjectType: "user",
		SubjectID: ownerID, OwnerUserID: ownerID, ActorUserID: ownerID, Operation: ports.AuthorizationDelete,
		LockedBy: lockedBy, LockedUntil: &lockedUntil, CreatedAt: now,
	}
	if err := store.DB.WithContext(ctx).Create(&row).Error; err != nil {
		t.Fatal(err)
	}

	claimed, err := store.ClaimAuthorizationChangeForResource(ctx, "path", resourceID, lockedBy, 2*time.Minute)
	if err != nil {
		t.Fatalf("same worker could not reclaim its preclaimed cleanup: %v", err)
	}
	if claimed.ID != changeID || claimed.LockedBy != lockedBy || claimed.Lease != 2*time.Minute || !claimed.LockedUntil.After(now.Add(time.Minute)) {
		t.Fatalf("claimed change = %+v", claimed)
	}
}
