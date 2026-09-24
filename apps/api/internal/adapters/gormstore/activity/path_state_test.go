package activitystore

import (
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresArchivedPathRejectsEveryActivityMutationWithoutChangingData(t *testing.T) {
	db := postgresDB(t, false)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "archived-write-participant", "archived-write-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	repository := New(db)

	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "archived-existing-activity", PathID: pathID, ParticipantID: participantID,
		StartedAt: now.Add(-time.Minute), DurationSeconds: 30, OccurrenceTimeZone: "Etc/UTC", Note: "retained",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateManualActivity(ctx, application.CreateManualActivityCommand{
		Activity: entry, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "archived-seed-activity", 81),
		Audit: activityAudit("archived-seed-activity-audit", participantID, entry.ID, audit.ResourceCreated, now),
	}); err != nil {
		t.Fatal(err)
	}
	running, err := domain.StartTimer("archived-existing-timer", pathID, participantID, now, "Etc/UTC", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.StartTimer(ctx, application.StartTimerCommand{
		Timer: running, Idempotency: idempotency(participantID, application.StartTimerOperation, "archived-seed-timer", 82),
		Audit: timerAudit("archived-seed-timer-audit", participantID, running.ID, audit.ActivityTimerStarted, now),
	}); err != nil {
		t.Fatal(err)
	}
	archivedAt := now.Add(time.Second)
	if err := db.Table("path_models").Where("id = ?", pathID).Updates(map[string]any{"archived_at": archivedAt, "updated_at": archivedAt}).Error; err != nil {
		t.Fatal(err)
	}

	newTimer, _ := domain.StartTimer("archived-new-timer", pathID, participantID, archivedAt, "Etc/UTC", archivedAt)
	newEntry, _ := domain.RecordManualActivity(domain.ManualActivity{
		ID: "archived-new-activity", PathID: pathID, ParticipantID: participantID,
		StartedAt: archivedAt.Add(-time.Minute), DurationSeconds: 10, OccurrenceTimeZone: "Etc/UTC",
	}, archivedAt)
	mutations := []struct {
		name string
		call func() error
	}{
		{name: "start timer", call: func() error {
			_, err := repository.StartTimer(ctx, application.StartTimerCommand{
				Timer: newTimer, Idempotency: idempotency(participantID, application.StartTimerOperation, "archived-start-key", 83),
				Audit: timerAudit("archived-start-audit", participantID, newTimer.ID, audit.ActivityTimerStarted, archivedAt),
			})
			return err
		}},
		{name: "stop timer", call: func() error {
			_, err := repository.StopTimer(ctx, application.StopTimerCommand{
				TimerID: running.ID, PathID: pathID, ParticipantID: participantID, ActivityID: "archived-stopped-activity",
				StoppedAt: archivedAt.Add(time.Second), RecordedAt: archivedAt.Add(time.Second),
				Idempotency: idempotency(participantID, application.StopTimerOperation, "archived-stop-key", 84),
				Audit:       timerAudit("archived-stop-audit", participantID, running.ID, audit.ActivityTimerStopped, archivedAt.Add(time.Second)),
			})
			return err
		}},
		{name: "create manual activity", call: func() error {
			_, err := repository.CreateManualActivity(ctx, application.CreateManualActivityCommand{
				Activity: newEntry, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "archived-create-key", 85),
				Audit: activityAudit("archived-create-audit", participantID, newEntry.ID, audit.ResourceCreated, archivedAt),
			})
			return err
		}},
		{name: "update activity", call: func() error {
			_, err := repository.UpdateActivity(ctx, application.UpdateActivityCommand{
				ActivityID: entry.ID, PathID: pathID, ParticipantID: participantID,
				Edit:      domain.ActivityEdit{StartedAt: entry.StartedAt, DurationSeconds: 20, OccurrenceTimeZone: "Etc/UTC", Note: "changed"},
				UpdatedAt: archivedAt.Add(time.Second), Idempotency: idempotency(participantID, application.UpdateActivityOperation, "archived-update-key", 86),
				Audit: activityAudit("archived-update-audit", participantID, entry.ID, audit.ResourceUpdated, archivedAt.Add(time.Second)),
			})
			return err
		}},
		{name: "delete activity", call: func() error {
			_, err := repository.DeleteActivity(ctx, application.DeleteActivityCommand{
				ActivityID: entry.ID, PathID: pathID, ParticipantID: participantID,
				Idempotency: idempotency(participantID, application.DeleteActivityOperation, "archived-delete-key", 87),
				Audit:       activityAudit("archived-delete-audit", participantID, entry.ID, audit.ResourceDeleted, archivedAt.Add(time.Second)),
			})
			return err
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if err := mutation.call(); !errors.Is(err, ports.ErrConflict) {
				t.Fatalf("archived mutation error = %v, want conflict", err)
			}
		})
	}
	if retained, err := repository.GetRunningTimer(ctx, participantID, pathID); err != nil || retained != running {
		t.Fatalf("rejected mutations changed timer: %+v, %v", retained, err)
	}
	if retained, version, err := repository.GetActivity(ctx, participantID, pathID, entry.ID); err != nil || retained != entry || version != 1 {
		t.Fatalf("rejected mutations changed activity: %+v version=%d err=%v", retained, version, err)
	}
	var rejectedReservations int64
	if err := db.Model(&mutationModel{}).Where("participant_id = ? AND key LIKE ?", participantID, "archived-%-key").Count(&rejectedReservations).Error; err != nil || rejectedReservations != 0 {
		t.Fatalf("rejected archived mutations retained reservations = %d, %v", rejectedReservations, err)
	}
}
