package pathstore

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type goalUpdateActivityRow struct {
	ID, PathID, ParticipantID, OccurrenceTimeZone string
	StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
}

func (goalUpdateActivityRow) TableName() string { return "recorded_activity_models" }

func TestPostgresUpdateGoalsAtomicallyReplacesRemovesAndProjectsCanonicalActivity(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	owner, administrator, pathID := "goal-update-owner", "goal-update-administrator", "goal-update-path"
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	cleanupGoalUpdateFixture(t, migrationDB, []string{owner, administrator}, []string{pathID})
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migrationDB, []string{owner, administrator}, []string{pathID}) })

	seedGoalUpdateUsers(t, migrationDB, now, owner, administrator)
	existing := domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{
			Name: "Guitar", Visibility: "followers",
			IntervalGoal:  domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceWeekly, Alignment: domain.GoalAlignment{ISOWeekday: 1}},
			OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 120},
		},
		CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	if err := migrationDB.Create(fromEntity(existing)).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: administrator, Role: "administrator"}).Error; err != nil {
		t.Fatal(err)
	}
	activity := goalUpdateActivityRow{
		ID: "goal-update-activity", PathID: pathID, ParticipantID: administrator,
		StartedAt: now.Add(-45 * time.Second), EndedAt: now, OccurrenceTimeZone: "Etc/UTC",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := migrationDB.Create(&activity).Error; err != nil {
		t.Fatal(err)
	}

	replacement := existing
	replacement.IntervalGoal = domain.IntervalGoal{Present: true, TargetSeconds: 30, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 0}}
	replacement.OverallTarget = domain.OverallTarget{Present: true, TargetSeconds: 90}
	replacement.UpdatedAt = now
	command := goalUpdateCommand(administrator, goalConfiguration(existing), replacement, "goal-update-key-0001", bytes.Repeat([]byte{1}, 32), "goal-update-audit-1", now)

	result, err := New(runtimeDB).UpdateGoals(ctx, command)
	if err != nil {
		t.Fatalf("UpdateGoals() error = %v", err)
	}
	wantProgress := &application.GoalIntervalProgress{
		TargetSeconds: 30, AccumulatedSeconds: 45,
		StartedAt: now.Truncate(24 * time.Hour), EndedAt: now.Truncate(24 * time.Hour).Add(24 * time.Hour),
	}
	if result.Path != replacement || result.Replayed || result.AccumulatedSeconds != 45 || result.IntervalProgress == nil || *result.IntervalProgress != *wantProgress {
		t.Fatalf("UpdateGoals() = %+v, want Path=%+v total=45 interval=%+v non-replayed", result, replacement, wantProgress)
	}
	persisted, err := New(runtimeDB).Get(ctx, administrator, replacement.ID)
	if err != nil || persisted != replacement {
		t.Fatalf("persisted replacement = %+v, %v; want %+v", persisted, err, replacement)
	}
	assertGoalUpdateAuditCount(t, migrationDB, pathID, audit.ResourceUpdated, 1)
	assertGoalUpdateReservationCount(t, migrationDB, administrator, application.UpdateGoalsOperation, command.Idempotency.Key, 1)
	var persistedAudit auditModel
	if err := migrationDB.Where("id = ?", command.Audit.ID).First(&persistedAudit).Error; err != nil {
		t.Fatal(err)
	}
	if persistedAudit.OwnerUserID != owner || persistedAudit.ActorUserID != administrator ||
		persistedAudit.TargetType != "path" || persistedAudit.TargetID != pathID ||
		persistedAudit.Action != audit.ResourceUpdated || persistedAudit.Outcome != audit.Succeeded {
		t.Fatalf("persisted goal update audit = %+v", persistedAudit)
	}

	replayed, err := New(runtimeDB).UpdateGoals(ctx, command)
	if err != nil || !replayed.Replayed || replayed.Path != replacement || replayed.AccumulatedSeconds != 45 || replayed.IntervalProgress == nil || *replayed.IntervalProgress != *wantProgress {
		t.Fatalf("replayed UpdateGoals() = %+v, %v", replayed, err)
	}
	assertGoalUpdateAuditCount(t, migrationDB, pathID, audit.ResourceUpdated, 1)
	assertGoalUpdateReservationCount(t, migrationDB, administrator, application.UpdateGoalsOperation, command.Idempotency.Key, 1)

	conflictingHash := command
	conflictingHash.Idempotency.RequestHash = bytes.Repeat([]byte{2}, 32)
	if _, err := New(runtimeDB).UpdateGoals(ctx, conflictingHash); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("same-key changed-hash replay error = %v, want idempotency conflict", err)
	}
	otherPath := existing
	otherPath.ID = "goal-update-other-path"
	otherPath.UpdatedAt = now
	seedGoalUpdatePath(t, migrationDB, otherPath, administrator)
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migrationDB, nil, []string{string(otherPath.ID)}) })
	conflictingPath := command
	conflictingPath.Path = otherPath
	conflictingPath.Audit.TargetID = string(otherPath.ID)
	if _, err := New(runtimeDB).UpdateGoals(ctx, conflictingPath); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("same-key changed-Path replay error = %v, want idempotency conflict", err)
	}

	staleReplacement := replacement
	staleReplacement.OverallTarget.TargetSeconds = 91
	staleReplacement.UpdatedAt = now.Add(30 * time.Second)
	stale := goalUpdateCommand(administrator, goalConfiguration(existing), staleReplacement, "goal-update-key-stale", bytes.Repeat([]byte{9}, 32), "goal-update-audit-stale", staleReplacement.UpdatedAt)
	if _, err := New(runtimeDB).UpdateGoals(ctx, stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale reviewed configuration error = %v, want conflict", err)
	}
	persistedAfterConflict, err := New(runtimeDB).Get(ctx, administrator, replacement.ID)
	if err != nil || persistedAfterConflict != replacement {
		t.Fatalf("stale conflict changed Path: got=%+v err=%v want=%+v", persistedAfterConflict, err, replacement)
	}
	assertGoalUpdateAuditCount(t, migrationDB, pathID, audit.ResourceUpdated, 1)
	assertGoalUpdateReservationCount(t, migrationDB, administrator, application.UpdateGoalsOperation, stale.Idempotency.Key, 0)

	removed := replacement
	removed.IntervalGoal = domain.IntervalGoal{}
	removed.OverallTarget = domain.OverallTarget{}
	removed.UpdatedAt = now.Add(time.Minute)
	removedCommand := goalUpdateCommand(administrator, goalConfiguration(replacement), removed, "goal-update-key-0002", bytes.Repeat([]byte{3}, 32), "goal-update-audit-2", removed.UpdatedAt)
	removedResult, err := New(runtimeDB).UpdateGoals(ctx, removedCommand)
	if err != nil || removedResult.Path != removed || removedResult.Replayed || removedResult.AccumulatedSeconds != 45 || removedResult.IntervalProgress != nil {
		t.Fatalf("goal removal result = %+v, %v; want unchanged total and omitted interval projection", removedResult, err)
	}
	var row model
	if err := migrationDB.Where("id = ?", pathID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.IntervalGoalTargetSeconds != nil || row.IntervalGoalRecurrence != nil || row.IntervalGoalStartMinute != nil ||
		row.IntervalGoalStartHour != nil || row.IntervalGoalStartWeekday != nil || row.IntervalGoalStartDay != nil ||
		row.IntervalGoalStartMonth != nil || row.OverallTargetSeconds != nil {
		t.Fatalf("removed goals did not persist as SQL NULL: %+v", row)
	}
	var activityCount int64
	if err := migrationDB.Model(&goalUpdateActivityRow{}).Where("id = ? AND started_at = ? AND ended_at = ?", activity.ID, activity.StartedAt, activity.EndedAt).Count(&activityCount).Error; err != nil || activityCount != 1 {
		t.Fatalf("canonical activity changed during goal removal: count=%d err=%v", activityCount, err)
	}
}

func TestPostgresUpdateGoalsRollsBackPathAndReservationWhenAuditFails(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	owner, pathID := "goal-update-rollback-owner", "goal-update-rollback-path"
	now := time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC)
	cleanupGoalUpdateFixture(t, migrationDB, []string{owner}, []string{pathID})
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migrationDB, []string{owner}, []string{pathID}) })
	seedGoalUpdateUsers(t, migrationDB, now, owner)
	existing := domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{
			Name: "Atomic", Visibility: "private",
			IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 0}},
		},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	seedGoalUpdatePath(t, migrationDB, existing, owner)
	duplicate := auditModel{
		ID: "goal-update-duplicate-audit", OwnerUserID: owner, ActorUserID: owner,
		Action: audit.ResourceViewed, TargetType: "path", TargetID: pathID,
		Outcome: audit.Succeeded, CorrelationID: "fixture", OccurredAt: now,
	}
	if err := migrationDB.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	changed := existing
	changed.IntervalGoal.TargetSeconds = 30
	changed.UpdatedAt = now
	command := goalUpdateCommand(owner, goalConfiguration(existing), changed, "goal-update-rollback-key", bytes.Repeat([]byte{4}, 32), duplicate.ID, now)

	if _, err := New(runtimeDB).UpdateGoals(context.Background(), command); err == nil {
		t.Fatal("UpdateGoals() succeeded despite duplicate audit identifier")
	}
	persisted, err := New(runtimeDB).Get(context.Background(), owner, domain.ID(pathID))
	if err != nil || persisted != existing {
		t.Fatalf("audit failure changed Path: got=%+v err=%v want=%+v", persisted, err, existing)
	}
	assertGoalUpdateReservationCount(t, migrationDB, owner, application.UpdateGoalsOperation, command.Idempotency.Key, 0)
}

func TestPostgresConcurrentGoalUpdatesFromSameReviewedConfigurationAllowExactlyOneWriter(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	owner, pathID := "goal-update-race-owner", "goal-update-race-path"
	now := time.Date(2026, 7, 23, 14, 0, 0, 0, time.UTC)
	cleanupGoalUpdateFixture(t, migrationDB, []string{owner}, []string{pathID})
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migrationDB, []string{owner}, []string{pathID}) })
	seedGoalUpdateUsers(t, migrationDB, now, owner)
	existing := domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{
			Name: "Concurrent", Visibility: "private",
			OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 60},
		},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	seedGoalUpdatePath(t, migrationDB, existing, owner)

	first, second := existing, existing
	first.OverallTarget.TargetSeconds = 120
	second.OverallTarget.TargetSeconds = 180
	first.UpdatedAt, second.UpdatedAt = now, now
	commands := []application.UpdateGoalsCommand{
		goalUpdateCommand(owner, goalConfiguration(existing), first, "goal-update-race-key-1", bytes.Repeat([]byte{5}, 32), "goal-update-race-audit-1", now),
		goalUpdateCommand(owner, goalConfiguration(existing), second, "goal-update-race-key-2", bytes.Repeat([]byte{6}, 32), "goal-update-race-audit-2", now),
	}
	start := make(chan struct{})
	results := make(chan error, len(commands))
	for _, command := range commands {
		command := command
		go func() {
			<-start
			_, err := New(runtimeDB).UpdateGoals(context.Background(), command)
			results <- err
		}()
	}
	close(start)

	successes, conflicts := 0, 0
	for range commands {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ports.ErrConflict):
			conflicts++
		default:
			t.Fatalf("concurrent UpdateGoals() error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent outcomes: successes=%d conflicts=%d", successes, conflicts)
	}
	persisted, err := New(runtimeDB).Get(context.Background(), owner, existing.ID)
	if err != nil || (persisted != first && persisted != second) {
		t.Fatalf("concurrent persisted Path = %+v, %v; want one complete replacement", persisted, err)
	}
	assertGoalUpdateAuditCount(t, migrationDB, pathID, audit.ResourceUpdated, 1)
	var reservations int64
	if err := migrationDB.Model(&idempotencyModel{}).
		Where("principal_id = ? AND operation = ? AND key IN ?", owner, application.UpdateGoalsOperation, []string{commands[0].Idempotency.Key, commands[1].Idempotency.Key}).
		Count(&reservations).Error; err != nil || reservations != 1 {
		t.Fatalf("concurrent reservation count = %d, %v; want 1", reservations, err)
	}
}

func TestPostgresGoalUpdatePreparedBeforeArchiveCannotMutateArchivedPath(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	owner, pathID := "goal-archive-race-owner", "goal-archive-race-path"
	archiveAt := time.Date(2026, 7, 23, 16, 0, 0, 0, time.UTC)
	cleanupGoalUpdateFixture(t, migrationDB, []string{owner}, []string{pathID})
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migrationDB, []string{owner}, []string{pathID}) })
	seedGoalUpdateUsers(t, migrationDB, archiveAt, owner)
	existing := domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{
			Name: "Archived race", Visibility: "private",
			OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 60},
		},
		CreatedAt: archiveAt.Add(-time.Hour), UpdatedAt: archiveAt.Add(-time.Minute),
	}
	seedGoalUpdatePath(t, migrationDB, existing, owner)

	prepared := existing
	prepared.OverallTarget.TargetSeconds = 120
	prepared.UpdatedAt = archiveAt.Add(time.Minute)
	command := goalUpdateCommand(
		owner,
		goalConfiguration(existing),
		prepared,
		"goal-archive-race-key-1",
		bytes.Repeat([]byte{7}, 32),
		"goal-archive-race-audit-1",
		prepared.UpdatedAt,
	)
	if err := migrationDB.Model(&model{}).Where("id = ?", pathID).
		Updates(map[string]any{"archived_at": archiveAt, "updated_at": archiveAt}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := New(runtimeDB).UpdateGoals(context.Background(), command); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("goal update racing after archive error = %v, want conflict", err)
	}
	persisted, err := New(runtimeDB).Get(context.Background(), owner, existing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.Archived() || persisted.OverallTarget != existing.OverallTarget || !persisted.UpdatedAt.Equal(archiveAt) {
		t.Fatalf("racing goal update changed archived Path: %+v", persisted)
	}
	assertGoalUpdateReservationCount(t, migrationDB, owner, application.UpdateGoalsOperation, command.Idempotency.Key, 0)
	assertGoalUpdateAuditCount(t, migrationDB, pathID, audit.ResourceUpdated, 0)
}

func goalUpdateDatabases(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
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
	t.Cleanup(func() {
		for name, database := range map[string]*gorm.DB{"runtime": runtimeDB, "migration": migrationDB} {
			sqlDB, sqlErr := database.DB()
			if sqlErr != nil {
				t.Errorf("%s database handle: %v", name, sqlErr)
				continue
			}
			if closeErr := sqlDB.Close(); closeErr != nil {
				t.Errorf("close %s database: %v", name, closeErr)
			}
		}
	})
	return runtimeDB, migrationDB
}

func seedGoalUpdateUsers(t *testing.T, db *gorm.DB, now time.Time, userIDs ...string) {
	t.Helper()
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Email, DisplayName   string
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	for _, userID := range userIDs {
		if err := db.Table("user_models").Create(&userRow{ID: userID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func seedGoalUpdatePath(t *testing.T, db *gorm.DB, entity domain.Entity, member string) {
	t.Helper()
	if err := db.Create(fromEntity(entity)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&membershipModel{PathID: string(entity.ID), UserID: member, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
}

func goalUpdateCommand(actor string, expected domain.GoalConfiguration, path domain.Entity, key string, hash []byte, auditID string, projectedAt time.Time) application.UpdateGoalsCommand {
	return application.UpdateGoalsCommand{
		ActorUserID: actor, ParticipantTimeZone: "Etc/UTC", ExpectedGoals: expected, Path: path, ProjectedAt: projectedAt,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: application.UpdateGoalsOperation, Key: key, RequestHash: hash},
		Audit: audit.Event{
			ID: auditID, OwnerUserID: path.OwnerUserID, ActorUserID: actor,
			Action: audit.ResourceUpdated, TargetType: "path", TargetID: string(path.ID),
			Outcome: audit.Succeeded, CorrelationID: "goal-update", OccurredAt: projectedAt,
		},
	}
}

func goalConfiguration(path domain.Entity) domain.GoalConfiguration {
	return domain.GoalConfiguration{IntervalGoal: path.IntervalGoal, OverallTarget: path.OverallTarget}
}

func cleanupGoalUpdateFixture(t *testing.T, db *gorm.DB, userIDs, pathIDs []string) {
	t.Helper()
	if len(userIDs) > 0 {
		_ = db.Table("audit_event_models").Where("owner_user_id IN ?", userIDs).Delete(&auditModel{}).Error
		_ = db.Table("idempotency_models").Where("principal_id IN ?", userIDs).Delete(&idempotencyModel{}).Error
	}
	if len(pathIDs) > 0 {
		_ = db.Table("path_models").Where("id IN ?", pathIDs).Delete(&model{}).Error
	}
	if len(userIDs) > 0 {
		_ = db.Table("user_models").Where("id IN ?", userIDs).Delete(map[string]any{}).Error
	}
}

func assertGoalUpdateAuditCount(t *testing.T, db *gorm.DB, pathID string, action audit.Action, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&auditModel{}).Where("target_type = ? AND target_id = ? AND action = ?", "path", pathID, action).Count(&count).Error; err != nil || count != want {
		t.Fatalf("goal update audit count = %d, %v; want %d", count, err, want)
	}
}

func assertGoalUpdateReservationCount(t *testing.T, db *gorm.DB, principal, operation, key string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&idempotencyModel{}).Where("principal_id = ? AND operation = ? AND key = ?", principal, operation, key).Count(&count).Error; err != nil || count != want {
		t.Fatalf("goal update reservation count = %d, %v; want %d", count, err, want)
	}
}
