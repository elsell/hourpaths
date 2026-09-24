package activitystore

import (
	"context"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

func TestPostgresMutationsReturnAtomicCurrentIntervalProjection(t *testing.T) {
	db := postgresDB(t, true)
	ctx := context.Background()
	window := application.IntervalWindow{
		StartedAt: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 7, 22, 13, 0, 0, 0, time.UTC),
	}
	request := &application.IntervalProgressRequest{TargetSeconds: 600, Window: window}
	recordedAt := window.EndedAt.Add(time.Hour)
	participantID, pathID := "mutation-interval-participant", "mutation-interval-path"
	seedParticipantAndPath(t, db, participantID, pathID, recordedAt)
	repository := New(db)

	seeded := recordIntervalMutationActivity(t, "mutation-interval-seeded", participantID, pathID, window.StartedAt.Add(5*time.Minute), 100, recordedAt)
	if err := db.Create(fromActivity(seeded)).Error; err != nil {
		t.Fatalf("persist seeded activity: %v", err)
	}

	assertProgress := func(name string, got *application.IntervalProgress, want int64) {
		t.Helper()
		if got == nil {
			t.Fatalf("%s interval progress = nil; want %d seconds for %+v", name, want, *request)
		}
		if got.TargetSeconds != request.TargetSeconds || got.Window != request.Window || got.AccumulatedSeconds != want {
			t.Fatalf("%s interval progress = %+v; want target=%d accumulated=%d window=%+v", name, got, request.TargetSeconds, want, request.Window)
		}
	}

	timer, err := domain.StartTimer("mutation-interval-timer", pathID, participantID, window.StartedAt.Add(10*time.Minute), "Etc/UTC", window.StartedAt.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	started, err := repository.StartTimer(ctx, application.StartTimerCommand{
		Timer: timer, IntervalProgress: request,
		Idempotency: idempotency(participantID, application.StartTimerOperation, "mutation-interval-start", 81),
		Audit:       timerAudit("mutation-interval-start-audit", participantID, timer.ID, audit.ActivityTimerStarted, timer.StartedAt),
	})
	if err != nil || started.Replayed || started.Timer != timer {
		t.Fatalf("StartTimer() = %+v, %v", started, err)
	}
	assertProgress("StartTimer()", started.IntervalProgress, 100)

	stoppedAt := timer.StartedAt.Add(time.Minute)
	stopped, err := repository.StopTimer(ctx, application.StopTimerCommand{
		TimerID: timer.ID, PathID: pathID, ParticipantID: participantID,
		ActivityID: "mutation-interval-stopped", StoppedAt: stoppedAt, RecordedAt: stoppedAt,
		IntervalProgress: request,
		Idempotency:      idempotency(participantID, application.StopTimerOperation, "mutation-interval-stop", 82),
		Audit:            timerAudit("mutation-interval-stop-audit", participantID, timer.ID, audit.ActivityTimerStopped, stoppedAt),
	})
	if err != nil || stopped.Replayed || !stopped.Saved || stopped.Activity.DurationSeconds() != 60 {
		t.Fatalf("StopTimer() = %+v, %v", stopped, err)
	}
	assertProgress("StopTimer()", stopped.IntervalProgress, 160)

	manual := recordIntervalMutationActivity(t, "mutation-interval-manual", participantID, pathID, window.StartedAt.Add(20*time.Minute), 40, recordedAt)
	created, err := repository.CreateManualActivity(ctx, application.CreateManualActivityCommand{
		Activity: manual, IntervalProgress: request,
		Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "mutation-interval-create", 83),
		Audit:       activityAudit("mutation-interval-create-audit", participantID, manual.ID, audit.ResourceCreated, recordedAt),
	})
	if err != nil || created.Replayed || created.Activity != manual || created.Version != 1 {
		t.Fatalf("CreateManualActivity() = %+v, %v", created, err)
	}
	assertProgress("CreateManualActivity()", created.IntervalProgress, 200)

	updatedAt := recordedAt.Add(time.Minute)
	updated, err := repository.UpdateActivity(ctx, application.UpdateActivityCommand{
		ActivityID: manual.ID, PathID: pathID, ParticipantID: participantID,
		Edit: domain.ActivityEdit{
			StartedAt: manual.StartedAt.Add(5 * time.Minute), DurationSeconds: 20,
			OccurrenceTimeZone: manual.OccurrenceTimeZone, Note: manual.Note,
		},
		UpdatedAt: updatedAt, IntervalProgress: request,
		Idempotency: idempotency(participantID, application.UpdateActivityOperation, "mutation-interval-update", 84),
		Audit:       activityAudit("mutation-interval-update-audit", participantID, manual.ID, audit.ResourceUpdated, updatedAt),
	})
	if err != nil || updated.Replayed || updated.Activity.DurationSeconds() != 20 || updated.Version != 2 {
		t.Fatalf("UpdateActivity() = %+v, %v", updated, err)
	}
	assertProgress("UpdateActivity()", updated.IntervalProgress, 180)

	deleted, err := repository.DeleteActivity(ctx, application.DeleteActivityCommand{
		ActivityID: seeded.ID, PathID: pathID, ParticipantID: participantID,
		IntervalProgress: request,
		Idempotency:      idempotency(participantID, application.DeleteActivityOperation, "mutation-interval-delete", 85),
		Audit:            activityAudit("mutation-interval-delete-audit", participantID, seeded.ID, audit.ResourceDeleted, recordedAt.Add(2*time.Minute)),
	})
	if err != nil || deleted.Replayed {
		t.Fatalf("DeleteActivity() = %+v, %v", deleted, err)
	}
	assertProgress("DeleteActivity()", deleted.IntervalProgress, 80)

	later := recordIntervalMutationActivity(t, "mutation-interval-later", participantID, pathID, window.StartedAt.Add(30*time.Minute), 10, recordedAt.Add(3*time.Minute))
	if err := db.Create(fromActivity(later)).Error; err != nil {
		t.Fatalf("persist later canonical activity: %v", err)
	}
	deleteReplayCommand := application.DeleteActivityCommand{
		ActivityID: seeded.ID, PathID: pathID, ParticipantID: participantID,
		IntervalProgress: request,
		Idempotency:      idempotency(participantID, application.DeleteActivityOperation, "mutation-interval-delete", 85),
		Audit:            activityAudit("mutation-interval-delete-audit", participantID, seeded.ID, audit.ResourceDeleted, recordedAt.Add(2*time.Minute)),
	}
	replayed, err := repository.DeleteActivity(ctx, deleteReplayCommand)
	if err != nil || !replayed.Replayed {
		t.Fatalf("replayed DeleteActivity() = %+v, %v", replayed, err)
	}
	if replayed.AccumulatedSeconds != deleted.AccumulatedSeconds {
		t.Fatalf("replayed DeleteActivity() accumulated seconds = %d; want original authoritative total %d", replayed.AccumulatedSeconds, deleted.AccumulatedSeconds)
	}
	assertProgress("replayed DeleteActivity()", replayed.IntervalProgress, 90)
}

func TestPostgresMutationWithoutIntervalGoalOmitsProjection(t *testing.T) {
	db := postgresDB(t, true)
	now := time.Date(2026, 7, 22, 14, 0, 0, 0, time.UTC)
	participantID, pathID := "mutation-no-goal-participant", "mutation-no-goal-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	entry := recordIntervalMutationActivity(t, "mutation-no-goal-activity", participantID, pathID, now.Add(-time.Minute), 30, now)

	result, err := New(db).CreateManualActivity(context.Background(), application.CreateManualActivityCommand{
		Activity:    entry,
		Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "mutation-no-goal-create", 86),
		Audit:       activityAudit("mutation-no-goal-create-audit", participantID, entry.ID, audit.ResourceCreated, now),
	})
	if err != nil {
		t.Fatalf("CreateManualActivity() error = %v", err)
	}
	if result.IntervalProgress != nil {
		t.Fatalf("CreateManualActivity() interval progress = %+v; want nil without interval goal request", result.IntervalProgress)
	}
}

func recordIntervalMutationActivity(t *testing.T, id, participantID, pathID string, startedAt time.Time, durationSeconds int64, recordedAt time.Time) domain.RecordedActivity {
	t.Helper()
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: id, PathID: pathID, ParticipantID: participantID, StartedAt: startedAt,
		DurationSeconds: durationSeconds, OccurrenceTimeZone: "Etc/UTC",
	}, recordedAt)
	if err != nil {
		t.Fatalf("RecordManualActivity(%q): %v", id, err)
	}
	return entry
}
