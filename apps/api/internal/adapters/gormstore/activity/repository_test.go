package activitystore

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	databaseDSN          = flag.String("database-dsn", "", "PostgreSQL runtime integration test DSN")
	migrationDatabaseDSN = flag.String("migration-database-dsn", "", "PostgreSQL migration-owner integration test DSN")
)

func TestPersistenceMappingRetainsCanonicalInstantsAndZone(t *testing.T) {
	now := time.Date(2026, 7, 21, 20, 30, 0, 123, time.UTC)
	timer, err := domain.StartTimer("timer", "path", "participant", now, "America/New_York", now)
	if err != nil {
		t.Fatal(err)
	}
	if got := toTimer(*fromTimer(timer)); got != timer {
		t.Fatalf("timer mapping = %+v, want %+v", got, timer)
	}
	entry, saved, err := timer.Stop("activity", now.Add(time.Second), now.Add(2*time.Second))
	if err != nil || !saved {
		t.Fatalf("Stop() = %+v, %v, %v", entry, saved, err)
	}
	if got := toActivity(*fromActivity(entry)); got != entry {
		t.Fatalf("activity mapping = %+v, want %+v", got, entry)
	}
}

func TestPostgresTimerLifecycleIsAtomicUniqueAndIdempotent(t *testing.T) {
	db := postgresDB(t, true)
	now := time.Now().UTC().Truncate(time.Microsecond).Add(789 * time.Nanosecond)
	participantID, pathID := "activity-participant", "activity-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)

	repository := New(db)
	timer, err := domain.StartTimer("timer-one", pathID, participantID, now, "America/New_York", now)
	if err != nil {
		t.Fatal(err)
	}
	canonicalTimer := timer
	canonicalTimer.StartedAt = timer.StartedAt.UTC().Truncate(time.Microsecond)
	start := application.StartTimerCommand{Timer: timer, Idempotency: idempotency(participantID, application.StartTimerOperation, "start-key-one", 0), Audit: timerAudit("start-audit-one", participantID, timer.ID, audit.ActivityTimerStarted, now)}
	started, err := repository.StartTimer(context.Background(), start)
	if err != nil || started.Replayed || started.Timer != canonicalTimer {
		t.Fatalf("StartTimer() = %+v, %v", started, err)
	}
	replayedStart, err := repository.StartTimer(context.Background(), start)
	stableStart := started
	stableStart.Replayed = true
	if err != nil || replayedStart != stableStart {
		t.Fatalf("replayed StartTimer() = %+v, %v", replayedStart, err)
	}
	conflictingReplay := start
	conflictingReplay.Idempotency.RequestHash = bytes.Repeat([]byte{1}, 32)
	if _, err := repository.StartTimer(context.Background(), conflictingReplay); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("conflicting start replay error = %v", err)
	}
	secondTimer, _ := domain.StartTimer("timer-two", pathID, participantID, now.Add(time.Second), "Etc/UTC", now.Add(time.Second))
	secondStart := application.StartTimerCommand{Timer: secondTimer, Idempotency: idempotency(participantID, application.StartTimerOperation, "start-key-two", 2), Audit: timerAudit("start-audit-two", participantID, secondTimer.ID, audit.ActivityTimerStarted, now.Add(time.Second))}
	duplicateConflictAudit := timerAudit(secondStart.Audit.ID, participantID, "unrelated-timer", audit.ActivityTimerStarted, now)
	if err := db.Create(fromAudit(duplicateConflictAudit)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.StartTimer(context.Background(), secondStart); err == nil || errors.Is(err, ports.ErrConflict) {
		t.Fatalf("second active timer ignored conflict audit failure: %v", err)
	}
	var failedReservationCount int64
	if err := db.Model(&mutationModel{}).Where("participant_id = ? AND operation = ? AND key = ?", participantID, application.StartTimerOperation, secondStart.Idempotency.Key).Count(&failedReservationCount).Error; err != nil || failedReservationCount != 0 {
		t.Fatalf("failed conflict audit retained %d reservations: %v", failedReservationCount, err)
	}
	secondStart.Audit.ID = "start-audit-two-retry"
	if _, err := repository.StartTimer(context.Background(), secondStart); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("second active timer error = %v, want conflict", err)
	}
	var conflictAudit auditModel
	if err := db.Where("id = ?", secondStart.Audit.ID).First(&conflictAudit).Error; err != nil || conflictAudit.Action != audit.ResourceViewed || conflictAudit.TargetID != timer.ID {
		t.Fatalf("conflicting start audit = %+v, %v", conflictAudit, err)
	}
	got, err := repository.GetRunningTimer(context.Background(), participantID, pathID)
	if err != nil || got != canonicalTimer {
		t.Fatalf("GetRunningTimer() = %+v, %v", got, err)
	}

	stop := application.StopTimerCommand{TimerID: timer.ID, PathID: pathID, ParticipantID: participantID, ActivityID: "activity-one", StoppedAt: now.Add(3 * time.Second), RecordedAt: now.Add(4 * time.Second), Idempotency: idempotency(participantID, application.StopTimerOperation, "stop-key-one", 3), Audit: timerAudit("stop-audit-one", participantID, timer.ID, audit.ActivityTimerStopped, now.Add(4*time.Second))}
	stopped, err := repository.StopTimer(context.Background(), stop)
	if err != nil || stopped.Replayed || !stopped.Saved || stopped.Activity.ID != "activity-one" || stopped.Activity.DurationSeconds() != 3 || stopped.AccumulatedSeconds != 3 {
		t.Fatalf("StopTimer() = %+v, %v", stopped, err)
	}
	if stopped.Activity.StartedAt.Nanosecond()%1000 != 0 || stopped.Activity.EndedAt.Nanosecond()%1000 != 0 || stopped.Activity.CreatedAt.Nanosecond()%1000 != 0 || stopped.Activity.UpdatedAt.Nanosecond()%1000 != 0 {
		t.Fatalf("StopTimer() returned instants beyond PostgreSQL precision: %+v", stopped.Activity)
	}
	replayedStop, err := repository.StopTimer(context.Background(), stop)
	stableStop := stopped
	stableStop.Replayed = true
	if err != nil || replayedStop != stableStop {
		t.Fatalf("replayed StopTimer() = %+v, %v", replayedStop, err)
	}
	assertPracticeFeedEvent(t, db, stopped.Activity, 1)
	delayedStart, err := repository.StartTimer(context.Background(), start)
	if err != nil || !delayedStart.Replayed || delayedStart.Timer != (domain.RunningTimer{}) || delayedStart.AccumulatedSeconds != 3 {
		t.Fatalf("delayed StartTimer() = %+v, %v; want stopped current state", delayedStart, err)
	}
	if _, err := repository.GetRunningTimer(context.Background(), participantID, pathID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("stopped timer remains: %v", err)
	}
	delayedSecondStart, err := repository.StartTimer(context.Background(), secondStart)
	if err != nil || !delayedSecondStart.Replayed || delayedSecondStart.Timer != (domain.RunningTimer{}) || delayedSecondStart.AccumulatedSeconds != 3 {
		t.Fatalf("delayed conflicting StartTimer() = %+v, %v; want stopped replay state", delayedSecondStart, err)
	}
	if _, err := repository.GetRunningTimer(context.Background(), participantID, pathID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("delayed conflicting start replay resurrected a timer: %v", err)
	}
	restartedTimer, _ := domain.StartTimer("timer-restarted", pathID, participantID, now.Add(5*time.Second), "America/New_York", now.Add(5*time.Second))
	restartedStart := application.StartTimerCommand{Timer: restartedTimer, Idempotency: idempotency(participantID, application.StartTimerOperation, "start-key-restarted", 4), Audit: timerAudit("start-audit-restarted", participantID, restartedTimer.ID, audit.ActivityTimerStarted, now.Add(5*time.Second))}
	startedAgain, err := repository.StartTimer(context.Background(), restartedStart)
	if err != nil || startedAgain.Replayed {
		t.Fatalf("restarted StartTimer() = %+v, %v", startedAgain, err)
	}
	delayedStop, err := repository.StopTimer(context.Background(), stop)
	if err != nil || !delayedStop.Replayed || delayedStop.CurrentTimer == nil || *delayedStop.CurrentTimer != startedAgain.Timer || delayedStop.Activity != stopped.Activity {
		t.Fatalf("delayed StopTimer() = %+v, %v; want restarted current timer with original activity", delayedStop, err)
	}
	var activities []activityModel
	if err := db.Where("participant_id = ? AND path_id = ?", participantID, pathID).Find(&activities).Error; err != nil || len(activities) != 1 {
		t.Fatalf("activities = %+v, %v", activities, err)
	}
	// PostgreSQL preserves the instant, while drivers may decode timestamptz in
	// the host's local location. The repository boundary canonicalizes it to UTC.
	if !activities[0].StartedAt.Equal(stopped.Activity.StartedAt) || !activities[0].EndedAt.Equal(stopped.Activity.EndedAt) || activities[0].OccurrenceTimeZone != "America/New_York" {
		t.Fatalf("canonical persistence drifted: %+v", activities[0])
	}
	if total, err := repository.AccumulatedSeconds(context.Background(), participantID, pathID); err != nil || total != 3 {
		t.Fatalf("AccumulatedSeconds() = %d, %v", total, err)
	}
	columns, err := db.Migrator().ColumnTypes(&activityModel{})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range columns {
		if column.Name() == "duration" {
			t.Fatal("recorded activity persisted a duration column")
		}
	}
	var auditCount int64
	if err := db.Model(&auditModel{}).Where("id IN ?", []string{"start-audit-one", "stop-audit-one"}).Count(&auditCount).Error; err != nil || auditCount != 2 {
		t.Fatalf("timer audit count = %d, %v", auditCount, err)
	}
	if err := db.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{}).Error; err != nil {
		t.Fatal(err)
	}
	var mutationCount int64
	if err := db.Model(&mutationModel{}).Where("participant_id = ?", participantID).Count(&mutationCount).Error; err != nil || mutationCount != 0 {
		t.Fatalf("Path deletion retained %d activity mutations: %v", mutationCount, err)
	}
}

func TestPostgresTimersOnDifferentPathsRemainIndependent(t *testing.T) {
	db := postgresDB(t, true)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, firstPathID, secondPathID := "simultaneous-participant", "simultaneous-path-one", "simultaneous-path-two"
	seedParticipantAndPath(t, db, participantID, firstPathID, now)
	seedPath(t, db, participantID, secondPathID, now)
	repository := New(db)

	firstTimer, err := domain.StartTimer("simultaneous-timer-one", firstPathID, participantID, now, "Etc/UTC", now)
	if err != nil {
		t.Fatal(err)
	}
	secondTimer, err := domain.StartTimer("simultaneous-timer-two", secondPathID, participantID, now.Add(time.Second), "Etc/UTC", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	firstStart := application.StartTimerCommand{Timer: firstTimer, Idempotency: idempotency(participantID, application.StartTimerOperation, "simultaneous-start-one", 20), Audit: timerAudit("simultaneous-start-audit-one", participantID, firstTimer.ID, audit.ActivityTimerStarted, now)}
	secondStart := application.StartTimerCommand{Timer: secondTimer, Idempotency: idempotency(participantID, application.StartTimerOperation, "simultaneous-start-two", 21), Audit: timerAudit("simultaneous-start-audit-two", participantID, secondTimer.ID, audit.ActivityTimerStarted, now.Add(time.Second))}
	if result, err := repository.StartTimer(context.Background(), firstStart); err != nil || result.Timer != firstTimer {
		t.Fatalf("first StartTimer() = %+v, %v", result, err)
	}
	if result, err := repository.StartTimer(context.Background(), secondStart); err != nil || result.Timer != secondTimer {
		t.Fatalf("second StartTimer() = %+v, %v", result, err)
	}
	for pathID, want := range map[string]domain.RunningTimer{firstPathID: firstTimer, secondPathID: secondTimer} {
		if got, err := repository.GetRunningTimer(context.Background(), participantID, pathID); err != nil || got != want {
			t.Fatalf("GetRunningTimer(%q) = %+v, %v; want %+v", pathID, got, err, want)
		}
	}

	firstStop := application.StopTimerCommand{TimerID: firstTimer.ID, PathID: firstPathID, ParticipantID: participantID, ActivityID: "simultaneous-activity-one", StoppedAt: now.Add(4 * time.Second), RecordedAt: now.Add(4 * time.Second), Idempotency: idempotency(participantID, application.StopTimerOperation, "simultaneous-stop-one", 22), Audit: timerAudit("simultaneous-stop-audit-one", participantID, firstTimer.ID, audit.ActivityTimerStopped, now.Add(4*time.Second))}
	firstResult, err := repository.StopTimer(context.Background(), firstStop)
	if err != nil || !firstResult.Saved || firstResult.Activity.DurationSeconds() != 4 || firstResult.AccumulatedSeconds != 4 {
		t.Fatalf("first StopTimer() = %+v, %v", firstResult, err)
	}
	if got, err := repository.GetRunningTimer(context.Background(), participantID, secondPathID); err != nil || got != secondTimer {
		t.Fatalf("stopping first Path altered second timer: %+v, %v", got, err)
	}
	secondStop := application.StopTimerCommand{TimerID: secondTimer.ID, PathID: secondPathID, ParticipantID: participantID, ActivityID: "simultaneous-activity-two", StoppedAt: now.Add(6 * time.Second), RecordedAt: now.Add(6 * time.Second), Idempotency: idempotency(participantID, application.StopTimerOperation, "simultaneous-stop-two", 23), Audit: timerAudit("simultaneous-stop-audit-two", participantID, secondTimer.ID, audit.ActivityTimerStopped, now.Add(6*time.Second))}
	secondResult, err := repository.StopTimer(context.Background(), secondStop)
	if err != nil || !secondResult.Saved || secondResult.Activity.DurationSeconds() != 5 || secondResult.AccumulatedSeconds != 5 {
		t.Fatalf("second StopTimer() = %+v, %v", secondResult, err)
	}
	if !firstResult.Activity.StartedAt.Before(secondResult.Activity.EndedAt) || !secondResult.Activity.StartedAt.Before(firstResult.Activity.EndedAt) {
		t.Fatalf("completed activities do not overlap: first=%+v second=%+v", firstResult.Activity, secondResult.Activity)
	}
	for _, replay := range []struct {
		command application.StopTimerCommand
		want    application.StopTimerResult
	}{{firstStop, firstResult}, {secondStop, secondResult}} {
		got, err := repository.StopTimer(context.Background(), replay.command)
		replay.want.Replayed = true
		if err != nil || got != replay.want {
			t.Fatalf("replayed StopTimer(%q) = %+v, %v; want %+v", replay.command.PathID, got, err, replay.want)
		}
	}
	for pathID, want := range map[string]int64{firstPathID: 4, secondPathID: 5} {
		if got, err := repository.AccumulatedSeconds(context.Background(), participantID, pathID); err != nil || got != want {
			t.Fatalf("AccumulatedSeconds(%q) = %d, %v; want %d", pathID, got, err, want)
		}
	}
}

func TestPostgresOverlappingActivitiesContributeFullDuration(t *testing.T) {
	db := postgresDB(t, true)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "overlap-participant", "overlap-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)

	firstTimer, _ := domain.StartTimer("overlap-source-one", pathID, participantID, now, "Etc/UTC", now)
	first, saved, err := firstTimer.Stop("overlap-activity-one", now.Add(10*time.Second), now.Add(20*time.Second))
	if err != nil || !saved {
		t.Fatalf("first Stop() = %+v, %v, %v", first, saved, err)
	}
	secondTimer, _ := domain.StartTimer("overlap-source-two", pathID, participantID, now.Add(5*time.Second), "Etc/UTC", now.Add(5*time.Second))
	second, saved, err := secondTimer.Stop("overlap-activity-two", now.Add(15*time.Second), now.Add(20*time.Second))
	if err != nil || !saved {
		t.Fatalf("second Stop() = %+v, %v, %v", second, saved, err)
	}
	if err := db.Create([]*activityModel{fromActivity(first), fromActivity(second)}).Error; err != nil {
		t.Fatalf("persist overlapping activities: %v", err)
	}
	if total, err := New(db).AccumulatedSeconds(context.Background(), participantID, pathID); err != nil || total != 20 {
		t.Fatalf("AccumulatedSeconds() = %d, %v; want full 10 + 10 seconds", total, err)
	}
	var rows []activityModel
	if err := db.Where("participant_id = ? AND path_id = ?", participantID, pathID).Order("started_at, id").Find(&rows).Error; err != nil || len(rows) != 2 {
		t.Fatalf("overlapping rows = %+v, %v", rows, err)
	}
	if toActivity(rows[0]) != first || toActivity(rows[1]) != second {
		t.Fatalf("overlapping boundaries changed: %+v", rows)
	}
}

func TestPostgresManualActivityCreateEditRevisionReplayAndPrivacy(t *testing.T) {
	db := postgresDB(t, false)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "manual-participant", "manual-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	if err := db.Table("path_models").Where("id = ?", pathID).Update("visibility", "public").Error; err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "manual-activity", PathID: pathID, ParticipantID: participantID,
		StartedAt: now.Add(-30 * time.Second), DurationSeconds: 10, OccurrenceTimeZone: "Etc/UTC", Note: "private before",
	}, now.Add(-20*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	create := application.CreateManualActivityCommand{
		Activity: entry, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "manual-create-key", 31),
		Audit: activityAudit("manual-create-audit", participantID, entry.ID, audit.ResourceCreated, entry.CreatedAt),
	}
	created, err := repository.CreateManualActivity(context.Background(), create)
	if err != nil || created.Replayed || created.Activity != entry || created.Version != 1 || created.AccumulatedSeconds != 10 {
		t.Fatalf("CreateManualActivity() = %+v, %v", created, err)
	}
	replayedCreate, err := repository.CreateManualActivity(context.Background(), create)
	wantCreateReplay := created
	wantCreateReplay.Replayed = true
	if err != nil || replayedCreate != wantCreateReplay {
		t.Fatalf("replayed CreateManualActivity() = %+v, %v", replayedCreate, err)
	}
	initialFeedEvent := assertPracticeFeedEvent(t, db, entry, 1)
	conflictingCreate := create
	conflictingCreate.Idempotency.RequestHash = bytes.Repeat([]byte{32}, 32)
	if _, err := repository.CreateManualActivity(context.Background(), conflictingCreate); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("conflicting create replay error = %v", err)
	}

	timer, _ := domain.StartTimer("manual-overlap-timer", pathID, participantID, now.Add(-5*time.Second), "Etc/UTC", now)
	if _, err := repository.StartTimer(context.Background(), application.StartTimerCommand{Timer: timer, Idempotency: idempotency(participantID, application.StartTimerOperation, "manual-overlap-start", 33), Audit: timerAudit("manual-overlap-start-audit", participantID, timer.ID, audit.ActivityTimerStarted, now)}); err != nil {
		t.Fatal(err)
	}
	edit := domain.ActivityEdit{StartedAt: now.Add(-15 * time.Second), DurationSeconds: 10, OccurrenceTimeZone: "Etc/UTC", Note: "private after"}
	update := application.UpdateActivityCommand{
		ActivityID: entry.ID, PathID: pathID, ParticipantID: participantID, Edit: edit, UpdatedAt: now,
		Idempotency: idempotency(participantID, application.UpdateActivityOperation, "manual-update-key", 34),
		Audit:       activityAudit("manual-update-audit", participantID, entry.ID, audit.ResourceUpdated, now),
	}
	updated, err := repository.UpdateActivity(context.Background(), update)
	if err != nil || updated.Replayed || updated.Version != 2 || updated.AccumulatedSeconds != 10 || updated.Revision.Activity != entry || updated.Activity.Note != "private after" || updated.Activity.DurationSeconds() != 10 {
		t.Fatalf("UpdateActivity() = %+v, %v", updated, err)
	}
	if running, err := repository.GetRunningTimer(context.Background(), participantID, pathID); err != nil || running != timer {
		t.Fatalf("manual update altered overlapping timer: %+v, %v", running, err)
	}
	replayedUpdate, err := repository.UpdateActivity(context.Background(), update)
	wantUpdateReplay := updated
	wantUpdateReplay.Replayed = true
	if err != nil || replayedUpdate != wantUpdateReplay {
		t.Fatalf("replayed UpdateActivity() = %+v, %v", replayedUpdate, err)
	}
	publicUpdatedAt := updated.Activity.UpdatedAt
	noteOnly := update
	noteOnly.Edit.Note = "private note only"
	noteOnly.UpdatedAt = now.Add(time.Second)
	noteOnly.Idempotency = idempotency(participantID, application.UpdateActivityOperation, "note-only-update-key", 36)
	noteOnly.Audit = activityAudit("note-only-update-audit", participantID, entry.ID, audit.ResourceUpdated, noteOnly.UpdatedAt)
	noteOnlyResult, err := repository.UpdateActivity(context.Background(), noteOnly)
	if err != nil || noteOnlyResult.Version != 3 || noteOnlyResult.Activity.Note != "private note only" {
		t.Fatalf("note-only UpdateActivity() = %+v, %v", noteOnlyResult, err)
	}
	if feedEvent := assertPracticeFeedEvent(t, db, noteOnlyResult.Activity, 1); feedEvent != initialFeedEvent {
		t.Fatalf("activity edits moved or replaced feed event: before=%+v after=%+v", initialFeedEvent, feedEvent)
	}
	var publicEditCount int64
	if err := db.Model(&activityRevisionModel{}).Where("activity_id = ? AND public_changed", entry.ID).Count(&publicEditCount).Error; err != nil || publicEditCount != 1 {
		t.Fatalf("derived public edit evidence = %d, %v; want one public revision", publicEditCount, err)
	}

	ownerView, version, err := repository.GetActivity(context.Background(), participantID, pathID, entry.ID)
	if err != nil || ownerView != noteOnlyResult.Activity || version != 3 {
		t.Fatalf("owner GetActivity() = %+v, %d, %v", ownerView, version, err)
	}
	otherView, otherVersion, err := repository.GetActivity(context.Background(), "another-viewer", pathID, entry.ID)
	if err != nil || otherView.Note != "" || otherVersion != 2 || otherView.ID != entry.ID || otherView.UpdatedAt != publicUpdatedAt {
		t.Fatalf("redacted GetActivity() = %+v, %d, %v", otherView, otherVersion, err)
	}
	page := application.ActivityRevisionPageRequest{Limit: 25, Snapshot: now.Add(time.Minute)}
	ownerHistory, err := repository.ListActivityRevisions(context.Background(), participantID, pathID, entry.ID, page)
	wantLatestOwnerRevision := application.ActivityRevisionRecord{Revision: noteOnlyResult.Revision, Version: 2}
	wantInitialOwnerRevision := application.ActivityRevisionRecord{Revision: updated.Revision, Version: 1}
	if err != nil || len(ownerHistory.Items) != 2 || ownerHistory.Items[0] != wantLatestOwnerRevision || ownerHistory.Items[1] != wantInitialOwnerRevision {
		t.Fatalf("owner revisions = %+v, %v", ownerHistory, err)
	}
	firstRevisionPage, err := repository.ListActivityRevisions(context.Background(), participantID, pathID, entry.ID, application.ActivityRevisionPageRequest{Limit: 1, Snapshot: page.Snapshot})
	if err != nil || len(firstRevisionPage.Items) != 1 || firstRevisionPage.Items[0].Version != 2 || !firstRevisionPage.HasMore {
		t.Fatalf("first revision page = %+v, %v", firstRevisionPage, err)
	}
	secondRevisionPage, err := repository.ListActivityRevisions(context.Background(), participantID, pathID, entry.ID, application.ActivityRevisionPageRequest{BeforeVersion: firstRevisionPage.Items[0].Version, Limit: 1, Snapshot: page.Snapshot})
	if err != nil || len(secondRevisionPage.Items) != 1 || secondRevisionPage.Items[0].Version != 1 || secondRevisionPage.HasMore {
		t.Fatalf("second revision page = %+v, %v", secondRevisionPage, err)
	}
	otherHistory, err := repository.ListActivityRevisions(context.Background(), "another-viewer", pathID, entry.ID, page)
	redactedInitialRevision := updated.Revision
	redactedInitialRevision.Activity.Note = ""
	wantPublicHistory := application.ActivityRevisionRecord{Revision: redactedInitialRevision, Version: 1}
	if err != nil || len(otherHistory.Items) != 1 || otherHistory.Items[0] != wantPublicHistory {
		t.Fatalf("redacted revisions = %+v, %v", otherHistory, err)
	}
	activityPage := application.ActivityPageRequest{Limit: 25, Snapshot: now.Add(time.Minute)}
	ownerList, err := repository.ListActivities(context.Background(), participantID, pathID, activityPage)
	if err != nil || len(ownerList.Items) != 1 || ownerList.Items[0].Version != 3 || ownerList.Items[0].Activity != noteOnlyResult.Activity {
		t.Fatalf("owner activity list = %+v, %v", ownerList, err)
	}
	otherList, err := repository.ListActivities(context.Background(), "another-viewer", pathID, activityPage)
	if err != nil || len(otherList.Items) != 1 || otherList.Items[0].Version != 2 || otherList.Items[0].Activity.Note != "" || otherList.Items[0].Activity.UpdatedAt != publicUpdatedAt {
		t.Fatalf("redacted activity list = %+v, %v", otherList, err)
	}
	denied := update
	denied.ParticipantID = "another-participant"
	denied.Idempotency = idempotency("another-participant", application.UpdateActivityOperation, "denied-update-key", 35)
	denied.Audit = activityAudit("denied-update-audit", "another-participant", entry.ID, audit.ResourceUpdated, now)
	if _, err := repository.UpdateActivity(context.Background(), denied); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("cross-owner update error = %v", err)
	}
	unchanged, unchangedVersion, err := repository.GetActivity(context.Background(), participantID, pathID, entry.ID)
	if err != nil || unchanged != noteOnlyResult.Activity || unchangedVersion != 3 {
		t.Fatalf("denied update changed activity: %+v, %d, %v", unchanged, unchangedVersion, err)
	}

	newerStartedAt := now.Add(-10 * time.Second)
	newerLow, err := domain.RecordManualActivity(domain.ManualActivity{ID: "page-activity-a", PathID: pathID, ParticipantID: participantID, StartedAt: newerStartedAt, DurationSeconds: 1, OccurrenceTimeZone: "Etc/UTC"}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	newerHigh, err := domain.RecordManualActivity(domain.ManualActivity{ID: "page-activity-z", PathID: pathID, ParticipantID: participantID, StartedAt: newerStartedAt, DurationSeconds: 1, OccurrenceTimeZone: "Etc/UTC"}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	historicalLateCreated, err := domain.RecordManualActivity(domain.ManualActivity{ID: "page-historical-late-created", PathID: pathID, ParticipantID: participantID, StartedAt: now.Add(-48 * time.Hour), DurationSeconds: 1, OccurrenceTimeZone: "Etc/UTC"}, now.Add(1500*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create([]*activityModel{fromActivity(newerLow), fromActivity(newerHigh), fromActivity(historicalLateCreated)}).Error; err != nil {
		t.Fatal(err)
	}
	snapshot := now.Add(2 * time.Second)
	firstActivityPage, err := repository.ListActivities(context.Background(), participantID, pathID, application.ActivityPageRequest{Limit: 1, Snapshot: snapshot})
	if err != nil || len(firstActivityPage.Items) != 1 || firstActivityPage.Items[0].Activity.ID != newerHigh.ID || !firstActivityPage.HasMore {
		t.Fatalf("first activity page = %+v, %v", firstActivityPage, err)
	}
	if _, err := repository.UpdateActivity(context.Background(), application.UpdateActivityCommand{
		ActivityID: newerHigh.ID, PathID: pathID, ParticipantID: participantID,
		Edit: domain.ActivityEdit{StartedAt: now.Add(-24 * time.Hour), DurationSeconds: 1, OccurrenceTimeZone: "Etc/UTC"}, UpdatedAt: now.Add(3 * time.Second),
		Idempotency: idempotency(participantID, application.UpdateActivityOperation, "page-stability-update-key", 37),
		Audit:       activityAudit("page-stability-update-audit", participantID, newerHigh.ID, audit.ResourceUpdated, now.Add(3*time.Second)),
	}); err != nil {
		t.Fatalf("edit after pagination snapshot: %v", err)
	}
	secondActivityPage, err := repository.ListActivities(context.Background(), participantID, pathID, application.ActivityPageRequest{AfterID: newerHigh.ID, AfterStartedAt: newerHigh.StartedAt, Limit: 1, Snapshot: snapshot})
	if err != nil || len(secondActivityPage.Items) != 1 || secondActivityPage.Items[0].Activity.ID != newerLow.ID || !secondActivityPage.HasMore {
		t.Fatalf("second activity page = %+v, %v", secondActivityPage, err)
	}
}

func TestPostgresConcurrentActivityUpdatesWithSharedIdempotencyKeyReturnConflict(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	participantID, pathID := "shared-update-user-"+suffix, "shared-update-path-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedParticipantAndPath(t, migrationDB, participantID, pathID, now)
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationDB.Table("user_models").Where("id = ?", participantID).Delete(&struct{ ID string }{})
	})

	activities := make([]domain.RecordedActivity, 2)
	for index := range activities {
		activity, recordErr := domain.RecordManualActivity(domain.ManualActivity{
			ID: fmt.Sprintf("shared-update-activity-%d-%s", index, suffix), PathID: pathID, ParticipantID: participantID,
			StartedAt: now.Add(time.Duration(-10-index) * time.Minute), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC",
		}, now.Add(-time.Minute))
		if recordErr != nil {
			t.Fatal(recordErr)
		}
		activities[index] = activity
		if createErr := migrationDB.Create(fromActivity(activity)).Error; createErr != nil {
			t.Fatal(createErr)
		}
	}

	progressTx := runtimeDB.Begin()
	if progressTx.Error != nil {
		t.Fatal(progressTx.Error)
	}
	t.Cleanup(func() { progressTx.Rollback() })
	if err := progresslock.Lock(progressTx, participantID, pathID); err != nil {
		t.Fatal(err)
	}

	type updateResult struct{ err error }
	results := make(chan updateResult, 2)
	started := make(chan struct{}, 2)
	sharedKey := "shared-update-key-" + suffix
	for index, activity := range activities {
		index, activity := index, activity
		go func() {
			started <- struct{}{}
			_, updateErr := New(runtimeDB).UpdateActivity(context.Background(), application.UpdateActivityCommand{
				ActivityID: activity.ID, PathID: pathID, ParticipantID: participantID,
				Edit:        domain.ActivityEdit{StartedAt: activity.StartedAt, DurationSeconds: int64(61 + index), OccurrenceTimeZone: activity.OccurrenceTimeZone},
				UpdatedAt:   now,
				Idempotency: idempotency(participantID, application.UpdateActivityOperation, sharedKey, byte(80+index)),
				Audit:       activityAudit(fmt.Sprintf("shared-update-audit-%d-%s", index, suffix), participantID, activity.ID, audit.ResourceUpdated, now),
			})
			results <- updateResult{err: updateErr}
		}()
	}
	for range activities {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent updates were not both admitted")
		}
	}
	select {
	case result := <-results:
		t.Fatalf("concurrent update escaped the canonical progress lock: %v", result.err)
	case <-time.After(250 * time.Millisecond):
	}
	if err := progressTx.Commit().Error; err != nil {
		t.Fatal(err)
	}

	var succeeded, conflicted int
	for range activities {
		var result updateResult
		select {
		case result = <-results:
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent update did not complete after releasing the progress lock")
		}
		switch {
		case result.err == nil:
			succeeded++
		case errors.Is(result.err, ports.ErrIdempotencyConflict):
			conflicted++
		default:
			t.Fatalf("concurrent update returned raw database error: %v", result.err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent updates = %d succeeded, %d conflicted; want one of each", succeeded, conflicted)
	}
}

func TestPostgresSubsecondStopReplaysWithoutActivity(t *testing.T) {
	db := postgresDB(t, false)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "subsecond-participant", "subsecond-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	repository := New(db)
	priorTimer, _ := domain.StartTimer("prior-timer", pathID, participantID, now.Add(-2*time.Minute), "Etc/UTC", now)
	priorActivity, saved, err := priorTimer.Stop("prior-activity", now.Add(-time.Minute), now.Add(-time.Minute))
	if err != nil || !saved {
		t.Fatalf("prior Stop() = %+v, %v, %v", priorActivity, saved, err)
	}
	if err := db.Create(fromActivity(priorActivity)).Error; err != nil {
		t.Fatal(err)
	}
	timer, _ := domain.StartTimer("subsecond-timer", pathID, participantID, now, "Etc/UTC", now)
	_, err = repository.StartTimer(context.Background(), application.StartTimerCommand{Timer: timer, Idempotency: idempotency(participantID, application.StartTimerOperation, "subsecond-start", 4), Audit: timerAudit("subsecond-start-audit", participantID, timer.ID, audit.ActivityTimerStarted, now)})
	if err != nil {
		t.Fatal(err)
	}
	command := application.StopTimerCommand{TimerID: timer.ID, PathID: pathID, ParticipantID: participantID, StoppedAt: now.Add(time.Second - time.Microsecond), RecordedAt: now.Add(time.Second), Idempotency: idempotency(participantID, application.StopTimerOperation, "subsecond-stop", 5), Audit: timerAudit("subsecond-stop-audit", participantID, timer.ID, audit.ActivityTimerStopped, now.Add(time.Second))}
	for attempt := 0; attempt < 2; attempt++ {
		result, err := repository.StopTimer(context.Background(), command)
		if err != nil || result.Saved || result.Activity != (domain.RecordedActivity{}) || result.AccumulatedSeconds != 60 || result.CurrentTimer != nil || result.Replayed != (attempt == 1) {
			t.Fatalf("attempt %d StopTimer() = %+v, %v", attempt, result, err)
		}
	}
	var count int64
	if err := db.Model(&timerModel{}).Where("participant_id = ? AND path_id = ?", participantID, pathID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("subsecond timer count = %d, %v", count, err)
	}
	if err := db.Model(&activityModel{}).Where("participant_id = ? AND path_id = ?", participantID, pathID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("subsecond activity count = %d, %v", count, err)
	}
	var mutation mutationModel
	if err := db.Where("participant_id = ? AND operation = ? AND key = ?", participantID, application.StopTimerOperation, "subsecond-stop").First(&mutation).Error; err != nil {
		t.Fatal(err)
	}
	if mutation.ResultActivitySaved || mutation.ResultActivityID != nil || mutation.ResultEndedAt != nil || mutation.ResultCreatedAt != nil || mutation.ResultUpdatedAt != nil {
		t.Fatalf("subsecond stop mutation retained activity projection: %+v", mutation)
	}
	if err := db.Model(&auditModel{}).Where("id = ?", "subsecond-stop-audit").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("subsecond stop audit count = %d, %v", count, err)
	}
}

func TestPostgresFailedStopRetainsTimerAndDoesNotReserveReplay(t *testing.T) {
	db := postgresDB(t, true)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "atomic-participant", "atomic-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	repository := New(db)
	timer, _ := domain.StartTimer("atomic-timer", pathID, participantID, now, "Etc/UTC", now)
	_, err := repository.StartTimer(context.Background(), application.StartTimerCommand{Timer: timer, Idempotency: idempotency(participantID, application.StartTimerOperation, "atomic-start", 6), Audit: timerAudit("atomic-start-audit", participantID, timer.ID, audit.ActivityTimerStarted, now)})
	if err != nil {
		t.Fatal(err)
	}
	existing, _, _ := timer.Stop("duplicate-activity", now.Add(time.Second), now.Add(time.Second))
	if err := db.Create(fromActivity(existing)).Error; err != nil {
		t.Fatal(err)
	}
	command := application.StopTimerCommand{TimerID: timer.ID, PathID: pathID, ParticipantID: participantID, ActivityID: existing.ID, StoppedAt: now.Add(2 * time.Second), RecordedAt: now.Add(2 * time.Second), Idempotency: idempotency(participantID, application.StopTimerOperation, "atomic-stop", 7), Audit: timerAudit("atomic-stop-audit", participantID, timer.ID, audit.ActivityTimerStopped, now.Add(2*time.Second))}
	if _, err := repository.StopTimer(context.Background(), command); err == nil {
		t.Fatal("StopTimer() succeeded despite duplicate activity identity")
	}
	if _, err := repository.GetRunningTimer(context.Background(), participantID, pathID); err != nil {
		t.Fatalf("failed stop removed timer: %v", err)
	}
	if err := db.Delete(fromActivity(existing)).Error; err != nil {
		t.Fatal(err)
	}
	if result, err := repository.StopTimer(context.Background(), command); err != nil || !result.Saved || result.Replayed {
		t.Fatalf("retry after rollback = %+v, %v", result, err)
	}
}

func TestPostgresAuditFailureRollsBackTimerAndReplayReservation(t *testing.T) {
	db := postgresDB(t, false)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "audit-rollback-participant", "audit-rollback-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	duplicate := timerAudit("duplicate-audit", participantID, "unrelated-timer", audit.ActivityTimerStarted, now)
	if err := db.Create(fromAudit(duplicate)).Error; err != nil {
		t.Fatal(err)
	}
	timer, _ := domain.StartTimer("audit-rollback-timer", pathID, participantID, now, "Etc/UTC", now)
	command := application.StartTimerCommand{Timer: timer, Idempotency: idempotency(participantID, application.StartTimerOperation, "audit-rollback-key", 8), Audit: timerAudit(duplicate.ID, participantID, timer.ID, audit.ActivityTimerStarted, now)}
	repository := New(db)
	if _, err := repository.StartTimer(context.Background(), command); err == nil {
		t.Fatal("StartTimer() succeeded despite duplicate audit identity")
	}
	if _, err := repository.GetRunningTimer(context.Background(), participantID, pathID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("audit failure retained timer: %v", err)
	}
	command.Audit.ID = "audit-rollback-retry"
	if result, err := repository.StartTimer(context.Background(), command); err != nil || result.Replayed {
		t.Fatalf("retry after audit rollback = %+v, %v", result, err)
	}
}

func TestPostgresReplayProjectionReturnsCurrentTimerAndCompletedTotalTogether(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	participantID, pathID := "replay-lock-user-"+suffix, "replay-lock-path-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedParticipantAndPath(t, migrationDB, participantID, pathID, now)
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationDB.Table("user_models").Where("id = ?", participantID).Delete(&struct{ ID string }{})
	})
	timer, _ := domain.StartTimer("replay-lock-timer-"+suffix, pathID, participantID, now, "Etc/UTC", now)
	if err := migrationDB.Create(fromTimer(timer)).Error; err != nil {
		t.Fatal(err)
	}
	entry, saved, err := timer.Stop("replay-lock-activity-"+suffix, now.Add(time.Second), now.Add(time.Second))
	if err != nil || !saved {
		t.Fatalf("Stop() = %+v, %v, %v", entry, saved, err)
	}

	if projection, err := currentProjection(runtimeDB, participantID, pathID, nil); err != nil || projection.Timer == nil || *projection.Timer != timer || projection.AccumulatedSeconds != 0 {
		t.Fatalf("running currentProjection() = %+v, %v", projection, err)
	}

	if err := runtimeDB.Transaction(func(stop *gorm.DB) error {
		deleted := stop.Where("id = ? AND participant_id = ? AND path_id = ?", timer.ID, participantID, pathID).Delete(&timerModel{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return fmt.Errorf("stop deleted %d timers", deleted.RowsAffected)
		}
		return stop.Create(fromActivity(entry)).Error
	}); err != nil {
		t.Fatal(err)
	}
	if projection, err := currentProjection(runtimeDB, participantID, pathID, nil); err != nil || projection.Timer != nil || projection.AccumulatedSeconds != 1 {
		t.Fatalf("stopped currentProjection() = %+v, %v", projection, err)
	}
}

func postgresDB(t *testing.T, migrationOwner bool) *gorm.DB {
	t.Helper()
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	dsn := *databaseDSN
	if migrationOwner {
		dsn = *migrationDatabaseDSN
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil && !errors.Is(err, gorm.ErrInvalidTransaction) {
			t.Errorf("rollback PostgreSQL activity fixture: %v", err)
		}
	})
	return tx
}

func idempotency(participantID, operation, key string, fill byte) ports.Idempotency {
	return ports.Idempotency{PrincipalID: participantID, Operation: operation, Key: key, RequestHash: bytes.Repeat([]byte{fill}, 32)}
}

func timerAudit(id, participantID, timerID string, action audit.Action, occurredAt time.Time) audit.Event {
	return audit.Event{ID: id, OwnerUserID: participantID, ActorUserID: participantID, Action: action, TargetType: "timer", TargetID: timerID, Outcome: audit.Succeeded, CorrelationID: "activity-request", OccurredAt: occurredAt}
}

func activityAudit(id, participantID, activityID string, action audit.Action, occurredAt time.Time) audit.Event {
	return audit.Event{ID: id, OwnerUserID: participantID, ActorUserID: participantID, Action: action, TargetType: "activity", TargetID: activityID, Outcome: audit.Succeeded, CorrelationID: "activity-request", OccurredAt: occurredAt}
}

func assertPracticeFeedEvent(t *testing.T, db *gorm.DB, entry domain.RecordedActivity, wantCount int64) practiceFeedEventModel {
	t.Helper()
	var count int64
	if err := db.Model(&practiceFeedEventModel{}).Where("source_activity_id = ?", entry.ID).Count(&count).Error; err != nil || count != wantCount {
		t.Fatalf("practice feed event count for %q = %d, %v; want %d", entry.ID, count, err, wantCount)
	}
	if wantCount == 0 {
		return practiceFeedEventModel{}
	}
	var row practiceFeedEventModel
	if err := db.Where("source_activity_id = ?", entry.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ID != "practice:"+entry.ID || row.ParticipantUserID != entry.ParticipantID || row.PathID != entry.PathID || !row.PublishedAt.Equal(entry.CreatedAt) {
		t.Fatalf("practice feed event = %+v, want stable source attribution and publication %v", row, entry.CreatedAt)
	}
	return row
}

func seedParticipantAndPath(t *testing.T, db *gorm.DB, participantID, pathID string, now time.Time) {
	seedParticipantAndPathCreatedAt(t, db, participantID, pathID, now, now.AddDate(0, 0, -7))
}

func seedParticipantAndPathCreatedAt(t *testing.T, db *gorm.DB, participantID, pathID string, now, pathCreatedAt time.Time) {
	t.Helper()
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Email, DisplayName   string
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	if err := db.Table("user_models").Create(&userRow{ID: participantID, Status: identity.StatusActive, CreatedAt: pathCreatedAt, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	seedPath(t, db, participantID, pathID, pathCreatedAt)
}

func seedPath(t *testing.T, db *gorm.DB, participantID, pathID string, now time.Time) {
	t.Helper()
	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		CreatedAt, UpdatedAt              time.Time
	}
	type membershipRow struct {
		PathID   string `gorm:"primaryKey"`
		UserID   string `gorm:"primaryKey"`
		Role     string
		JoinedAt time.Time
	}
	if err := db.Table("path_models").Create(&pathRow{ID: pathID, OwnerUserID: participantID, Name: "Activity path", Visibility: "private", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: participantID, Role: "participant", JoinedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
}
