package gormstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestAuthorizationOutboxLeasesAreOwnedRecoverableAndFIFO(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", "lease-test").Delete(&authorizationOutboxModel{}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", "lease-test").Delete(&authorizationOutboxModel{}).Error
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", "lease-test").Delete(&authorizationResourceLockModel{}).Error
	})
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	if err := store.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&userModel{ID: "outbox-user", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []authorizationOutboxModel{
		{ID: "a", ResourceType: "resource", ResourceID: "lease-test", Relation: "owner", SubjectType: "user", SubjectID: "outbox-user", OwnerUserID: "outbox-user", ActorUserID: "outbox-user", Operation: ports.AuthorizationTouch, CreatedAt: now.Add(time.Hour)},
		{ID: "b", ResourceType: "resource", ResourceID: "lease-test", Relation: "owner", SubjectType: "user", SubjectID: "outbox-user", OwnerUserID: "outbox-user", ActorUserID: "outbox-user", Operation: ports.AuthorizationDelete, CreatedAt: now.Add(-time.Hour)},
	} {
		if err := store.DB.WithContext(ctx).Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.ClaimAuthorizationChanges(ctx, "worker-a", 30*time.Second, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].ID != "a" {
		t.Fatalf("database did not preserve insertion order across skewed producer clocks: %+v", first)
	}
	var databaseNow time.Time
	if err := store.DB.WithContext(ctx).Model(&authorizationOutboxModel{}).Select("CURRENT_TIMESTAMP").Where("id = ?", "a").Scan(&databaseNow).Error; err != nil {
		t.Fatal(err)
	}
	if delta := first[0].LockedUntil.Sub(databaseNow); delta < 25*time.Second || delta > 35*time.Second {
		t.Fatalf("lease was not derived from PostgreSQL time: database=%v locked_until=%v delta=%v", databaseNow, first[0].LockedUntil, delta)
	}
	second, err := store.ClaimAuthorizationChanges(ctx, "worker-b", 30*time.Second, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second worker claimed locked resource stream: %+v", second)
	}
	completionEvent := audit.Event{ID: "lease-completed", OwnerUserID: "outbox-user", ActorUserID: "outbox-user", Action: audit.AuthorizationApplied, TargetType: "resource", TargetID: "lease-test", Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: time.Now().UTC()}
	if err := store.CompleteAuthorizationChangeWithAudit(ctx, "a", "worker-b", completionEvent); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("non-owner completed lease: %v", err)
	}
	if err := store.RenewAuthorizationChange(ctx, "a", "worker-b", 2*time.Minute); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("non-owner renewed lease: %v", err)
	}
	if err := store.DB.WithContext(ctx).Model(&authorizationOutboxModel{}).Where("id = ?", "a").Update("locked_until", gorm.Expr("CURRENT_TIMESTAMP - INTERVAL '1 second'")).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteAuthorizationChangeWithAudit(ctx, "a", "worker-a", completionEvent); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("expired owner completed lease: %v", err)
	}
	if err := store.RenewAuthorizationChange(ctx, "a", "worker-a", 2*time.Minute); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("expired owner renewed lease: %v", err)
	}
	recovered, err := store.ClaimAuthorizationChanges(ctx, "worker-b", 30*time.Second, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || recovered[0].ID != "a" {
		t.Fatalf("expired lease was not recovered: %+v", recovered)
	}
	if err := store.CompleteAuthorizationChangeWithAudit(ctx, "a", "worker-b", completionEvent); err != nil {
		t.Fatal(err)
	}
	next, err := store.ClaimAuthorizationChanges(ctx, "worker-b", 30*time.Second, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].ID != "b" {
		t.Fatalf("next FIFO operation unavailable after completion: %+v", next)
	}
}

func TestAuthorizationMigrationLockBlocksLegacyProducer(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL migration locking")
	}
	runtimeStore, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	migrationSQLDB, err := migrationStore.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	migrationConnection, err := migrationSQLDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = migrationConnection.Close() })
	migrationTransaction, err := migrationConnection.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = migrationTransaction.Rollback() })
	if _, err := migrationTransaction.ExecContext(ctx, "LOCK TABLE public.authorization_outbox_models IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}

	sqlDB, err := runtimeStore.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	producer, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = producer.Close() })
	observer, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = observer.Close() })
	var producerPID int
	if err := producer.QueryRowContext(ctx, "SELECT pg_catalog.pg_backend_pid()").Scan(&producerPID); err != nil {
		t.Fatal(err)
	}
	changeID := "migration-lock-" + newTestID()
	resourceID := "migration-lock-" + newTestID()
	t.Cleanup(func() {
		_ = runtimeStore.DB.Where("id = ?", changeID).Delete(&authorizationOutboxModel{}).Error
		_ = runtimeStore.DB.Where("resource_type = ? AND resource_id = ?", "resource", resourceID).Delete(&authorizationResourceLockModel{}).Error
	})
	producerDone := make(chan error, 1)
	go func() {
		_, err := producer.ExecContext(ctx, `
			INSERT INTO authorization_outbox_models (
				id, resource_type, resource_id, relation, subject_type, subject_id,
				owner_user_id, actor_user_id, operation, created_at
			) VALUES ($1, 'resource', $2, 'owner', 'user', 'migration-lock-owner',
				'migration-lock-owner', 'migration-lock-owner', 'touch', CURRENT_TIMESTAMP)`,
			changeID, resourceID,
		)
		producerDone <- err
	}()
	waitForPostgresLock(t, ctx, observer, producerPID, producerDone)
	if err := migrationTransaction.Rollback(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-producerDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatalf("legacy producer remained blocked after migration lock release: %v", context.Cause(ctx))
	}
}
