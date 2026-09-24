package activitystore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	activitydomain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var achievementTestInstant = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

func TestOverlappingIntervalWindowsUsesCurrentParticipantCalendarAcrossEveryOccurrence(t *testing.T) {
	goal := pathdomain.IntervalGoal{Present: true, TargetSeconds: 30, Recurrence: pathdomain.RecurrenceDaily, Alignment: pathdomain.GoalAlignment{Hour: 0}}
	started := time.Date(2026, 7, 28, 3, 59, 0, 0, time.UTC)
	windows, err := overlappingIntervalWindows(goal, "America/New_York", started, started.Add(2*time.Minute))
	if err != nil || len(windows) != 2 || windows[0].EndedAt != time.Date(2026, 7, 28, 4, 0, 0, 0, time.UTC) || windows[1].StartedAt != windows[0].EndedAt {
		t.Fatalf("overlapping windows=%+v err=%v", windows, err)
	}
}

func TestPostgresManualActivityPublishesAndInvalidatesGoalAchievementsAtomically(t *testing.T) {
	db := postgresDB(t, false)
	now := achievementTestInstant
	participantID, pathID := "achievement-participant", "achievement-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	seedAchievementPreference(t, db, participantID, "Etc/UTC", now)
	configureAchievementGoals(t, db, pathID, 60, 60)
	goals, err := currentAchievementGoals(db, pathID)
	if err != nil || goals.IntervalGoal.TargetSeconds == nil || *goals.IntervalGoal.TargetSeconds != 60 ||
		goals.IntervalGoal.Recurrence == nil || *goals.IntervalGoal.Recurrence != "daily" ||
		goals.IntervalGoal.StartHour == nil || *goals.IntervalGoal.StartHour != 0 ||
		goals.OverallTargetSeconds == nil || *goals.OverallTargetSeconds != 60 {
		t.Fatalf("current achievement goals=%+v err=%v", goals, err)
	}
	repository := New(db)

	if got := loadGoalAchievements(t, db, participantID, pathID); len(got) != 0 {
		t.Fatalf("goal configuration published achievements: %+v", got)
	}
	first := manualAchievementActivity(t, "achievement-first", participantID, pathID, now.Add(-2*time.Minute), 60, now)
	createFirst := application.CreateManualActivityCommand{Activity: first, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "achievement-first-create", 71), Audit: activityAudit("achievement-first-audit", participantID, first.ID, audit.ResourceCreated, now)}
	if _, err := repository.CreateManualActivity(context.Background(), createFirst); err != nil {
		t.Fatal(err)
	}
	earned := loadGoalAchievements(t, db, participantID, pathID)
	assertEarnedGoalShapes(t, db, earned, first, 60)
	originalIDs := map[string]string{earned[0].Kind: earned[0].ID, earned[1].Kind: earned[1].ID}
	if _, err := repository.CreateManualActivity(context.Background(), createFirst); err != nil || len(loadGoalAchievements(t, db, participantID, pathID)) != 2 {
		t.Fatalf("idempotent replay duplicated achievements: %v", err)
	}

	additional := manualAchievementActivity(t, "achievement-additional", participantID, pathID, now.Add(-30*time.Second), 10, now.Add(time.Second))
	if _, err := repository.CreateManualActivity(context.Background(), application.CreateManualActivityCommand{Activity: additional, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "achievement-additional-create", 72), Audit: activityAudit("achievement-additional-audit", participantID, additional.ID, audit.ResourceCreated, additional.CreatedAt)}); err != nil || len(loadGoalAchievements(t, db, participantID, pathID)) != 2 {
		t.Fatalf("additional above-target time duplicated achievements: %v", err)
	}

	feedID := "achievement:" + earned[0].ID
	if err := db.Table("social_practice_reaction_models").Create(map[string]any{"social_feed_event_id": feedID, "actor_user_id": participantID, "reaction_type": "heart", "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	commentID := "achievement-comment"
	if err := db.Table("social_practice_comment_models").Create(map[string]any{
		"id": commentID, "social_feed_event_id": feedID, "author_user_id": participantID,
		"body": "Supported achievement", "version": int64(1), "created_at": now, "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("social_practice_comment_revision_models").Create(map[string]any{
		"comment_id": commentID, "version": int64(1), "body": "Supported achievement", "changed_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("social_practice_comment_heart_models").Create(map[string]any{"comment_id": commentID, "actor_user_id": participantID, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	for _, notification := range []map[string]any{
		{"id": "achievement-reaction-notification", "recipient_user_id": participantID, "actor_user_id": participantID, "path_id": pathID, "social_feed_event_id": feedID, "reaction_type": "heart", "kind": "practice_reaction", "presentation_class": "informational", "channel": "reactions", "created_at": now},
		{"id": "achievement-comment-notification", "recipient_user_id": participantID, "actor_user_id": participantID, "path_id": pathID, "social_feed_event_id": feedID, "comment_id": commentID, "kind": "practice_comment", "presentation_class": "informational", "channel": "comments", "created_at": now},
		{"id": "achievement-heart-notification", "recipient_user_id": participantID, "actor_user_id": participantID, "path_id": pathID, "social_feed_event_id": feedID, "comment_id": commentID, "kind": "comment_heart", "presentation_class": "informational", "channel": "comment_hearts", "created_at": now},
	} {
		if err := db.Table("notification_models").Create(notification).Error; err != nil {
			t.Fatal(err)
		}
	}
	editAt := now.Add(2 * time.Second)
	if _, err := repository.UpdateActivity(context.Background(), application.UpdateActivityCommand{ActivityID: first.ID, PathID: pathID, ParticipantID: participantID, Edit: activitydomain.ActivityEdit{StartedAt: first.StartedAt, DurationSeconds: 30, OccurrenceTimeZone: first.OccurrenceTimeZone}, UpdatedAt: editAt, Idempotency: idempotency(participantID, application.UpdateActivityOperation, "achievement-first-edit", 73), Audit: activityAudit("achievement-first-edit-audit", participantID, first.ID, audit.ResourceUpdated, editAt)}); err != nil {
		t.Fatal(err)
	}
	if got := loadGoalAchievements(t, db, participantID, pathID); len(got) != 0 {
		t.Fatalf("unsupported achievements survived edit: %+v", got)
	}
	for _, table := range []string{"social_practice_reaction_models", "social_practice_comment_models", "social_practice_comment_revision_models", "social_practice_comment_heart_models", "notification_models"} {
		var count int64
		query := db.Table(table)
		switch table {
		case "social_practice_comment_revision_models", "social_practice_comment_heart_models":
			query = query.Where("comment_id = ?", commentID)
		default:
			query = query.Where("social_feed_event_id = ?", feedID)
		}
		if err := query.Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("invalidated achievement %s count=%d err=%v", table, count, err)
		}
	}

	recross := manualAchievementActivity(t, "achievement-recross", participantID, pathID, now.Add(-20*time.Second), 20, now.Add(3*time.Second))
	if _, err := repository.CreateManualActivity(context.Background(), application.CreateManualActivityCommand{Activity: recross, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "achievement-recross-create", 74), Audit: activityAudit("achievement-recross-audit", participantID, recross.ID, audit.ResourceCreated, recross.CreatedAt)}); err != nil {
		t.Fatal(err)
	}
	reearned := loadGoalAchievements(t, db, participantID, pathID)
	if len(reearned) != 2 {
		t.Fatalf("genuine recross achievements=%+v", reearned)
	}
	for _, row := range reearned {
		if originalIDs[row.Kind] == row.ID {
			t.Fatalf("recross restored old identity for %s: %s", row.Kind, row.ID)
		}
	}

	configureAchievementGoals(t, db, pathID, 30, 30)
	if got := loadGoalAchievements(t, db, participantID, pathID); len(got) != 2 {
		t.Fatalf("goal reconfiguration erased or created achievement rows: %+v", got)
	}
	afterLowering := manualAchievementActivity(t, "achievement-after-lowering", participantID, pathID, now.Add(-5*time.Second), 1, now.Add(4*time.Second))
	if _, err := repository.CreateManualActivity(context.Background(), application.CreateManualActivityCommand{Activity: afterLowering, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "achievement-after-lowering-create", 75), Audit: activityAudit("achievement-after-lowering-audit", participantID, afterLowering.ID, audit.ResourceCreated, afterLowering.CreatedAt)}); err != nil || len(loadGoalAchievements(t, db, participantID, pathID)) != 2 {
		t.Fatalf("already-above lowered target published achievement: %v", err)
	}
	if _, err := repository.DeleteActivity(context.Background(), application.DeleteActivityCommand{ActivityID: recross.ID, PathID: pathID, ParticipantID: participantID, Idempotency: idempotency(participantID, application.DeleteActivityOperation, "achievement-recross-delete", 79), Audit: activityAudit("achievement-recross-delete-audit", participantID, recross.ID, audit.ResourceDeleted, now.Add(5*time.Second))}); err != nil {
		t.Fatal(err)
	}
	if got := loadGoalAchievements(t, db, participantID, pathID); len(got) != 0 {
		t.Fatalf("unsupported stored-target achievements survived deletion: %+v", got)
	}
}

func TestPostgresStoppedTimerCanPublishBothGoalAchievements(t *testing.T) {
	db := postgresDB(t, false)
	now := achievementTestInstant
	participantID, pathID := "timer-achievement-participant", "timer-achievement-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	seedAchievementPreference(t, db, participantID, "Etc/UTC", now)
	configureAchievementGoals(t, db, pathID, 5, 5)
	repository := New(db)
	timer, err := activitydomain.StartTimer("timer-achievement", pathID, participantID, now.Add(-5*time.Second), "Etc/UTC", now.Add(-5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.StartTimer(context.Background(), application.StartTimerCommand{Timer: timer, Idempotency: idempotency(participantID, application.StartTimerOperation, "timer-achievement-start", 76), Audit: timerAudit("timer-achievement-start-audit", participantID, timer.ID, audit.ActivityTimerStarted, timer.StartedAt)}); err != nil {
		t.Fatal(err)
	}
	result, err := repository.StopTimer(context.Background(), application.StopTimerCommand{TimerID: timer.ID, PathID: pathID, ParticipantID: participantID, ActivityID: "timer-achievement-activity", StoppedAt: now, RecordedAt: now, Idempotency: idempotency(participantID, application.StopTimerOperation, "timer-achievement-stop", 77), Audit: timerAudit("timer-achievement-stop-audit", participantID, timer.ID, audit.ActivityTimerStopped, now)})
	if err != nil || !result.Saved {
		t.Fatalf("StopTimer()=%+v err=%v", result, err)
	}
	assertEarnedGoalShapes(t, db, loadGoalAchievements(t, db, participantID, pathID), result.Activity, 5)
}

func TestPostgresLongBackdatedActivityPublishesEveryCurrentProfileIntervalCrossing(t *testing.T) {
	db := postgresDB(t, false)
	now := achievementTestInstant
	participantID, pathID := "multi-interval-participant", "multi-interval-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	seedAchievementPreference(t, db, participantID, "America/New_York", now)
	configureAchievementGoals(t, db, pathID, 30, 10_000)
	started := time.Date(2026, 7, 28, 3, 59, 0, 0, time.UTC)
	entry := manualAchievementActivity(t, "multi-interval-activity", participantID, pathID, started, 120, now)
	if _, err := New(db).CreateManualActivity(context.Background(), application.CreateManualActivityCommand{Activity: entry, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "multi-interval-create", 78), Audit: activityAudit("multi-interval-audit", participantID, entry.ID, audit.ResourceCreated, now)}); err != nil {
		t.Fatal(err)
	}
	rows := loadGoalAchievements(t, db, participantID, pathID)
	if len(rows) != 2 || rows[0].Kind != "interval" || rows[1].Kind != "interval" || rows[0].IntervalStartedAt == nil || rows[1].IntervalStartedAt == nil || rows[0].IntervalStartedAt.Equal(*rows[1].IntervalStartedAt) {
		t.Fatalf("multi-interval achievements=%+v", rows)
	}
}

func TestPostgresConcurrentActivityCrossingsBothCommitOneSupportedAchievementPerKind(t *testing.T) {
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
	participantID, pathID := "achievement-concurrent-user-"+suffix, "achievement-concurrent-path-"+suffix
	now := achievementTestInstant
	seedParticipantAndPath(t, migrationDB, participantID, pathID, now)
	seedAchievementPreference(t, migrationDB, participantID, "Etc/UTC", now)
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationDB.Table("user_models").Where("id = ?", participantID).Delete(&struct{ ID string }{})
	})
	prior := manualAchievementActivity(t, "achievement-concurrent-prior-"+suffix, participantID, pathID, now.Add(-3*time.Minute), 80, now)
	if err := migrationDB.Create(fromActivity(prior)).Error; err != nil {
		t.Fatal(err)
	}
	configureAchievementGoals(t, migrationDB, pathID, 100, 100)

	progressTx := runtimeDB.Begin()
	if progressTx.Error != nil {
		t.Fatal(progressTx.Error)
	}
	t.Cleanup(func() { progressTx.Rollback() })
	if err := progresslock.Lock(progressTx, participantID, pathID); err != nil {
		t.Fatal(err)
	}

	results := make(chan error, 2)
	started := make(chan struct{}, 2)
	entries := []activitydomain.RecordedActivity{
		manualAchievementActivity(t, "achievement-concurrent-0-"+suffix, participantID, pathID, now.Add(-2*time.Minute), 20, now),
		manualAchievementActivity(t, "achievement-concurrent-1-"+suffix, participantID, pathID, now.Add(-time.Minute), 20, now.Add(time.Second)),
	}
	for index := 0; index < 2; index++ {
		index := index
		go func() {
			started <- struct{}{}
			entry := entries[index]
			_, createErr := New(runtimeDB).CreateManualActivity(context.Background(), application.CreateManualActivityCommand{Activity: entry, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, fmt.Sprintf("achievement-concurrent-key-%d-%s", index, suffix), byte(90+index)), Audit: activityAudit(fmt.Sprintf("achievement-concurrent-audit-%d-%s", index, suffix), participantID, entry.ID, audit.ResourceCreated, entry.CreatedAt)})
			results <- createErr
		}()
	}
	for index := 0; index < 2; index++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent crossings were not both admitted")
		}
	}
	select {
	case createErr := <-results:
		t.Fatalf("concurrent crossing escaped the canonical progress lock: %v", createErr)
	case <-time.After(250 * time.Millisecond):
	}
	if err := progressTx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		var createErr error
		select {
		case createErr = <-results:
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent activity did not complete after releasing the progress lock")
		}
		if createErr != nil {
			t.Fatalf("concurrent activity %d failed: %v", index, createErr)
		}
	}
	rows := loadGoalAchievements(t, migrationDB, participantID, pathID)
	if len(rows) != 2 || rows[0].Kind != "interval" || rows[1].Kind != "overall" {
		t.Fatalf("concurrent supported achievements=%+v, want one per kind", rows)
	}
	var activityCount int64
	if err := migrationDB.Model(&activityModel{}).Where("participant_id = ? AND path_id = ?", participantID, pathID).Count(&activityCount).Error; err != nil || activityCount != 3 {
		t.Fatalf("concurrent committed activities=%d err=%v, want three including prior", activityCount, err)
	}
}

func configureAchievementGoals(t *testing.T, db *gorm.DB, pathID string, intervalTarget, overallTarget int64) {
	t.Helper()
	if err := db.Table("path_models").Where("id = ?", pathID).Updates(map[string]any{"interval_goal_target_seconds": intervalTarget, "interval_goal_recurrence": "daily", "interval_goal_start_hour": 0, "overall_target_seconds": overallTarget}).Error; err != nil {
		t.Fatal(err)
	}
}

func seedAchievementPreference(t *testing.T, db *gorm.DB, participantID, timeZone string, now time.Time) {
	t.Helper()
	if err := db.Table("user_preference_models").Create(map[string]any{"user_id": participantID, "first_day_of_week": 1, "current_time_zone": timeZone, "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
}

func manualAchievementActivity(t *testing.T, id, participantID, pathID string, startedAt time.Time, duration int64, createdAt time.Time) activitydomain.RecordedActivity {
	t.Helper()
	entry, err := activitydomain.RecordManualActivity(activitydomain.ManualActivity{ID: id, PathID: pathID, ParticipantID: participantID, StartedAt: startedAt, DurationSeconds: duration, OccurrenceTimeZone: "Etc/UTC"}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func loadGoalAchievements(t *testing.T, db *gorm.DB, participantID, pathID string) []goalAchievementModel {
	t.Helper()
	var rows []goalAchievementModel
	if err := db.Where("participant_user_id = ? AND path_id = ?", participantID, pathID).Order("kind").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func assertEarnedGoalShapes(t *testing.T, db *gorm.DB, rows []goalAchievementModel, source activitydomain.RecordedActivity, target int64) {
	t.Helper()
	if len(rows) != 2 {
		t.Fatalf("achievements=%+v, want interval and overall", rows)
	}
	for _, row := range rows {
		if row.ParticipantUserID != source.ParticipantID || row.PathID != source.PathID || row.TargetSeconds != target || !row.PublishedAt.Equal(source.CreatedAt) {
			t.Fatalf("achievement attribution=%+v", row)
		}
		if row.Kind == "interval" {
			if row.IntervalStartedAt == nil || row.IntervalEndedAt == nil || !row.IntervalEndedAt.After(*row.IntervalStartedAt) {
				t.Fatalf("interval bounds=%+v", row)
			}
		} else if row.Kind != "overall" || row.IntervalStartedAt != nil || row.IntervalEndedAt != nil {
			t.Fatalf("achievement kind shape=%+v", row)
		}
		var feedCount int64
		if err := db.Table("social_feed_event_models").Where("achievement_id = ? AND id = ?", row.ID, "achievement:"+row.ID).Count(&feedCount).Error; err != nil || feedCount != 1 {
			t.Fatalf("achievement feed count=%d err=%v row=%+v", feedCount, err, row)
		}
	}
}
