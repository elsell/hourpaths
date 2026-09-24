package activitystore

import (
	"context"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	activitydomain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

func TestPostgresPracticeFeedPublicationCommitsAtomicallyWithActivityCreation(t *testing.T) {
	db := postgresDB(t, false)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "feed-atomic-participant", "feed-atomic-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	repository := New(db)

	t.Run("manual activity", func(t *testing.T) {
		entry, err := activitydomain.RecordManualActivity(activitydomain.ManualActivity{
			ID: "feed-atomic-manual", PathID: pathID, ParticipantID: participantID,
			StartedAt: now.Add(-time.Minute), DurationSeconds: 30, OccurrenceTimeZone: "Etc/UTC",
		}, now)
		if err != nil {
			t.Fatal(err)
		}
		duplicateAudit := activityAudit("feed-atomic-manual-audit", participantID, "other", audit.ResourceCreated, now)
		if err := db.Create(fromAudit(duplicateAudit)).Error; err != nil {
			t.Fatal(err)
		}
		command := application.CreateManualActivityCommand{
			Activity: entry,
			Idempotency: idempotency(participantID, application.CreateManualActivityOperation,
				"feed-atomic-manual-key", 91),
			Audit: activityAudit(duplicateAudit.ID, participantID, entry.ID, audit.ResourceCreated, now),
		}
		if _, err := repository.CreateManualActivity(context.Background(), command); err == nil {
			t.Fatal("manual activity succeeded after audit persistence failed")
		}
		assertPracticeFeedEvent(t, db, entry, 0)
		var activityCount int64
		if err := db.Model(&activityModel{}).Where("id = ?", entry.ID).Count(&activityCount).Error; err != nil || activityCount != 0 {
			t.Fatalf("failed manual creation retained %d activities: %v", activityCount, err)
		}
	})

	t.Run("completed timer", func(t *testing.T) {
		timer, err := activitydomain.StartTimer("feed-atomic-timer", pathID, participantID, now, "Etc/UTC", now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repository.StartTimer(context.Background(), application.StartTimerCommand{
			Timer: timer,
			Idempotency: idempotency(participantID, application.StartTimerOperation,
				"feed-atomic-timer-start", 92),
			Audit: timerAudit("feed-atomic-timer-start-audit", participantID, timer.ID, audit.ActivityTimerStarted, now),
		}); err != nil {
			t.Fatal(err)
		}
		duplicateAudit := timerAudit("feed-atomic-timer-stop-audit", participantID, "other", audit.ActivityTimerStopped, now.Add(time.Minute))
		if err := db.Create(fromAudit(duplicateAudit)).Error; err != nil {
			t.Fatal(err)
		}
		command := application.StopTimerCommand{
			TimerID: timer.ID, PathID: pathID, ParticipantID: participantID,
			ActivityID: "feed-atomic-timer-activity", StoppedAt: now.Add(time.Minute), RecordedAt: now.Add(time.Minute),
			Idempotency: idempotency(participantID, application.StopTimerOperation,
				"feed-atomic-timer-stop", 93),
			Audit: timerAudit(duplicateAudit.ID, participantID, timer.ID, audit.ActivityTimerStopped, now.Add(time.Minute)),
		}
		if _, err := repository.StopTimer(context.Background(), command); err == nil {
			t.Fatal("timer stop succeeded after audit persistence failed")
		}
		if _, err := repository.GetRunningTimer(context.Background(), participantID, pathID); err != nil {
			t.Fatalf("failed timer stop removed running timer: %v", err)
		}
		var feedCount int64
		if err := db.Model(&practiceFeedEventModel{}).Where("source_activity_id = ?", command.ActivityID).Count(&feedCount).Error; err != nil || feedCount != 0 {
			t.Fatalf("failed timer stop retained %d feed events: %v", feedCount, err)
		}
	})
}
