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
	"gorm.io/gorm"
)

type archiveTimerRow struct {
	ID, PathID, ParticipantID, OccurrenceTimeZone string
	StartedAt                                     time.Time
}

func (archiveTimerRow) TableName() string { return "running_timer_models" }

func TestPostgresArchiveAtomicallyStopsOnlyPathTimersAndUnarchiveDoesNotRestartThem(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	owner, firstParticipant, secondParticipant := "archive-owner", "archive-first", "archive-second"
	pathID, otherPathID := "archive-path", "archive-other-path"
	now := time.Date(2026, 7, 23, 15, 0, 0, 123_456_000, time.UTC)
	users := []string{owner, firstParticipant, secondParticipant}
	paths := []string{pathID, otherPathID}
	cleanupGoalUpdateFixture(t, migrationDB, users, paths)
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migrationDB, users, paths) })
	seedGoalUpdateUsers(t, migrationDB, now, users...)
	existing := domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{
			Name: "Shared practice", Visibility: "followers",
			IntervalGoal:  domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 0}},
			OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 3_600},
		},
		CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	seedGoalUpdatePath(t, migrationDB, existing, owner)
	for _, participant := range []string{firstParticipant, secondParticipant} {
		if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: participant, Role: "participant"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	other := domain.Entity{
		ID: domain.ID(otherPathID), OwnerUserID: owner,
		Attributes: domain.Attributes{Name: "Other practice", Visibility: "private"},
		CreatedAt:  existing.CreatedAt, UpdatedAt: existing.UpdatedAt,
	}
	seedGoalUpdatePath(t, migrationDB, other, owner)
	timers := []archiveTimerRow{
		{ID: "archive-timer-first", PathID: pathID, ParticipantID: firstParticipant, StartedAt: now.Add(-45 * time.Second), OccurrenceTimeZone: "Etc/UTC"},
		{ID: "archive-timer-subsecond", PathID: pathID, ParticipantID: secondParticipant, StartedAt: now.Add(-500 * time.Millisecond), OccurrenceTimeZone: "Etc/UTC"},
		{ID: "archive-timer-other", PathID: otherPathID, ParticipantID: firstParticipant, StartedAt: now.Add(-time.Minute), OccurrenceTimeZone: "Etc/UTC"},
	}
	if err := migrationDB.Create(&timers).Error; err != nil {
		t.Fatal(err)
	}

	archived, err := existing.Archive(now)
	if err != nil {
		t.Fatal(err)
	}
	activityIDs := sequentialTestIDs("archive-activity-first", "archive-activity-subsecond")
	command := archiveStateCommand(owner, existing.Archived(), archived, "archive-state-key-1", bytes.Repeat([]byte{1}, 32), "archive-state-audit-1", now, activityIDs)
	result, err := New(runtimeDB).SetArchiveState(ctx, command)
	if err != nil || result.Path != archived || result.Replayed {
		t.Fatalf("SetArchiveState(archive) = %+v, %v; want %+v", result, err, archived)
	}
	assertArchiveTimerCount(t, migrationDB, pathID, 0)
	assertArchiveTimerCount(t, migrationDB, otherPathID, 1)
	var activities []goalUpdateActivityRow
	if err := migrationDB.Where("path_id = ?", pathID).Find(&activities).Error; err != nil {
		t.Fatal(err)
	}
	if len(activities) != 1 || activities[0].ID != "archive-activity-first" || activities[0].ParticipantID != firstParticipant ||
		!activities[0].StartedAt.Equal(timers[0].StartedAt) || !activities[0].EndedAt.Equal(now) ||
		!activities[0].CreatedAt.Equal(now) || !activities[0].UpdatedAt.Equal(now) {
		t.Fatalf("archived timer activity = %+v, want one exact shared-instant activity", activities)
	}
	activePage, err := New(runtimeDB).List(ctx, firstParticipant, application.PageRequest{Limit: 10, Snapshot: now.Add(time.Minute)})
	if err != nil || len(activePage.Items) != 0 {
		t.Fatalf("active list after archive = %+v, %v", activePage, err)
	}
	archivedPage, err := New(runtimeDB).List(ctx, firstParticipant, application.PageRequest{Limit: 10, Snapshot: now.Add(time.Minute), Archived: true})
	if err != nil || len(archivedPage.Items) != 1 || archivedPage.Items[0] != archived {
		t.Fatalf("archived list = %+v, %v; want archived Path", archivedPage, err)
	}
	if visible, err := New(runtimeDB).Get(ctx, secondParticipant, existing.ID); err != nil || visible != archived {
		t.Fatalf("member archived Get() = %+v, %v; want retained access", visible, err)
	}

	replay, err := New(runtimeDB).SetArchiveState(ctx, command)
	if err != nil || !replay.Replayed || replay.Path != archived {
		t.Fatalf("archive replay = %+v, %v", replay, err)
	}
	var activityCount int64
	if err := migrationDB.Model(&goalUpdateActivityRow{}).Where("path_id = ?", pathID).Count(&activityCount).Error; err != nil || activityCount != 1 {
		t.Fatalf("archive replay activity count = %d, %v", activityCount, err)
	}
	assertGoalUpdateAuditCount(t, migrationDB, pathID, audit.ResourceUpdated, 1)

	unarchivedAt := now.Add(time.Minute)
	unarchived, err := archived.Unarchive(unarchivedAt)
	if err != nil {
		t.Fatal(err)
	}
	unarchiveCommand := archiveStateCommand(owner, true, unarchived, "archive-state-key-2", bytes.Repeat([]byte{2}, 32), "archive-state-audit-2", unarchivedAt, sequentialTestIDs("must-not-be-used"))
	unarchiveResult, err := New(runtimeDB).SetArchiveState(ctx, unarchiveCommand)
	if err != nil || unarchiveResult.Path != unarchived || unarchiveResult.Replayed {
		t.Fatalf("SetArchiveState(unarchive) = %+v, %v; want %+v", unarchiveResult, err, unarchived)
	}
	assertArchiveTimerCount(t, migrationDB, pathID, 0)
	if err := migrationDB.Model(&goalUpdateActivityRow{}).Where("path_id = ?", pathID).Count(&activityCount).Error; err != nil || activityCount != 1 {
		t.Fatalf("unarchive changed historical activity count = %d, %v", activityCount, err)
	}
	activePage, err = New(runtimeDB).List(ctx, firstParticipant, application.PageRequest{Limit: 10, Snapshot: unarchivedAt.Add(time.Minute)})
	if err != nil || len(activePage.Items) != 1 || activePage.Items[0] != unarchived {
		t.Fatalf("active list after unarchive = %+v, %v", activePage, err)
	}
	assertGoalUpdateAuditCount(t, migrationDB, pathID, audit.ResourceUpdated, 2)

	stale := archiveStateCommand(owner, true, unarchived, "archive-state-stale", bytes.Repeat([]byte{3}, 32), "archive-state-audit-stale", unarchivedAt.Add(time.Minute), sequentialTestIDs("unused"))
	if _, err := New(runtimeDB).SetArchiveState(ctx, stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale unarchive error = %v, want conflict", err)
	}
	assertGoalUpdateReservationCount(t, migrationDB, owner, application.SetArchiveStateOperation, stale.Idempotency.Key, 0)
}

func archiveStateCommand(actor string, expectedArchived bool, path domain.Entity, key string, hash []byte, auditID string, changedAt time.Time, newActivityID func() string) application.SetArchiveStateCommand {
	return application.SetArchiveStateCommand{
		ActorUserID: actor, ExpectedArchived: expectedArchived, Archived: path.Archived(), Path: path, ChangedAt: changedAt,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: application.SetArchiveStateOperation, Key: key, RequestHash: hash},
		Audit: audit.Event{
			ID: auditID, OwnerUserID: path.OwnerUserID, ActorUserID: actor,
			Action: audit.ResourceUpdated, TargetType: "path", TargetID: string(path.ID),
			Outcome: audit.Succeeded, CorrelationID: "archive-state", OccurredAt: changedAt,
		},
		NewActivityID: newActivityID,
	}
}

func sequentialTestIDs(values ...string) func() string {
	index := 0
	return func() string {
		if index >= len(values) {
			return "extra-archive-id"
		}
		value := values[index]
		index++
		return value
	}
}

func assertArchiveTimerCount(t *testing.T, db *gorm.DB, pathID string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&archiveTimerRow{}).Where("path_id = ?", pathID).Count(&count).Error; err != nil || count != want {
		t.Fatalf("timer count for %s = %d, %v; want %d", pathID, count, err, want)
	}
}
