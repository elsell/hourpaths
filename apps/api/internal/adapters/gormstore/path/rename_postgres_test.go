package pathstore

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresRenameIsActorScopedAtomicReplaySafeAndConflictAware(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 20, 0, 0, 0, time.UTC)
	owner, administrator, outsider := "rename-owner", "rename-administrator", "rename-outsider"
	pathID := domain.ID("rename-path")
	users := []string{owner, administrator, outsider}
	cleanupGoalUpdateFixture(t, migrationDB, users, []string{string(pathID)})
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migrationDB, users, []string{string(pathID)}) })
	seedGoalUpdateUsers(t, migrationDB, now, users...)
	existing := domain.Entity{
		ID: pathID, OwnerUserID: owner,
		Attributes: domain.Attributes{Name: "Before", Visibility: "private"},
		CreatedAt:  now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute),
	}
	seedGoalUpdatePath(t, migrationDB, existing, administrator)
	renamed, err := existing.Rename("After", now)
	if err != nil {
		t.Fatal(err)
	}
	repository := New(runtimeDB)
	command := renameCommand(administrator, "Before", renamed, "rename-key-0000001", bytes.Repeat([]byte{1}, 32), "rename-audit-1")

	result, err := repository.Rename(ctx, command)
	if err != nil || result.Path != renamed || result.Replayed {
		t.Fatalf("Rename() = %+v, %v", result, err)
	}
	replayed, err := repository.Rename(ctx, command)
	if err != nil || !replayed.Replayed || replayed.Path != renamed {
		t.Fatalf("Rename() replay = %+v, %v", replayed, err)
	}
	assertGoalUpdateAuditCount(t, migrationDB, string(pathID), audit.ResourceUpdated, 1)
	assertGoalUpdateReservationCount(t, migrationDB, administrator, application.RenameOperation, command.Idempotency.Key, 1)

	stale := renamed
	stale.UpdatedAt = now.Add(time.Minute)
	stale.Name = "Stale overwrite"
	staleCommand := renameCommand(administrator, "Before", stale, "rename-key-0000002", bytes.Repeat([]byte{2}, 32), "rename-audit-2")
	if _, err := repository.Rename(ctx, staleCommand); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale Rename() error = %v, want conflict", err)
	}
	persisted, err := repository.Get(ctx, administrator, pathID)
	if err != nil || persisted != renamed {
		t.Fatalf("Path after conflict = %+v, %v; want %+v", persisted, err, renamed)
	}
	assertGoalUpdateReservationCount(t, migrationDB, administrator, application.RenameOperation, staleCommand.Idempotency.Key, 0)

	outsiderCommand := renameCommand(outsider, "After", stale, "rename-key-0000003", bytes.Repeat([]byte{3}, 32), "rename-audit-3")
	if _, err := repository.Rename(ctx, outsiderCommand); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("outsider Rename() error = %v, want opaque not found", err)
	}
	assertGoalUpdateReservationCount(t, migrationDB, outsider, application.RenameOperation, outsiderCommand.Idempotency.Key, 0)
}

func renameCommand(actor, expectedName string, path domain.Entity, key string, hash []byte, auditID string) application.RenameCommand {
	return application.RenameCommand{
		ActorUserID: actor, ExpectedName: expectedName, Path: path,
		Idempotency: ports.Idempotency{
			PrincipalID: actor, Operation: application.RenameOperation, Key: key, RequestHash: hash,
		},
		Audit: audit.Event{
			ID: auditID, OwnerUserID: path.OwnerUserID, ActorUserID: actor,
			Action: audit.ResourceUpdated, TargetType: "path", TargetID: string(path.ID),
			Outcome: audit.Succeeded, CorrelationID: "rename", OccurredAt: path.UpdatedAt,
		},
	}
}
