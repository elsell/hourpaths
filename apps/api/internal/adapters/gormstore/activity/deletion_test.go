package activitystore

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPostgresDeleteActivityTombstonesReplayAndPreservesUnrelatedState(t *testing.T) {
	db := postgresDB(t, false)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "delete-participant", "delete-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	repository := New(db)

	record := func(id string, startedAt time.Time, duration int64, note, key string, fill byte) domain.RecordedActivity {
		t.Helper()
		entry, err := domain.RecordManualActivity(domain.ManualActivity{
			ID: id, PathID: pathID, ParticipantID: participantID, StartedAt: startedAt,
			DurationSeconds: duration, OccurrenceTimeZone: "Etc/UTC", Note: note,
		}, now)
		if err != nil {
			t.Fatal(err)
		}
		result, err := repository.CreateManualActivity(context.Background(), application.CreateManualActivityCommand{
			Activity:    entry,
			Idempotency: idempotency(participantID, application.CreateManualActivityOperation, key, fill),
			Audit:       activityAudit(key+"-audit", participantID, id, audit.ResourceCreated, now),
		})
		if err != nil || result.Activity != entry {
			t.Fatalf("CreateManualActivity(%s) = %+v, %v", id, result, err)
		}
		return entry
	}

	target := record("delete-target", now.Add(-40*time.Second), 10, "private original", "delete-create-target", 41)
	unrelated := record("delete-unrelated", now.Add(-30*time.Second), 20, "unrelated private", "delete-create-unrelated", 42)
	updatedAt := now.Add(time.Second)
	updated, err := repository.UpdateActivity(context.Background(), application.UpdateActivityCommand{
		ActivityID: target.ID, PathID: pathID, ParticipantID: participantID,
		Edit:        domain.ActivityEdit{StartedAt: target.StartedAt, DurationSeconds: 10, OccurrenceTimeZone: target.OccurrenceTimeZone, Note: "private updated"},
		UpdatedAt:   updatedAt,
		Idempotency: idempotency(participantID, application.UpdateActivityOperation, "delete-update-target", 43),
		Audit:       activityAudit("delete-update-target-audit", participantID, target.ID, audit.ResourceUpdated, updatedAt),
	})
	if err != nil || updated.Version != 2 {
		t.Fatalf("UpdateActivity() = %+v, %v", updated, err)
	}

	denied := application.DeleteActivityCommand{
		ActivityID: target.ID, PathID: pathID, ParticipantID: "another-participant",
		Idempotency: idempotency("another-participant", application.DeleteActivityOperation, "delete-denied-key", 44),
		Audit:       activityAudit("delete-denied-audit", "another-participant", target.ID, audit.ResourceDeleted, now.Add(2*time.Second)),
	}
	if _, err := repository.DeleteActivity(context.Background(), denied); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("cross-owner DeleteActivity() error = %v", err)
	}
	if retained, _, err := repository.GetActivity(context.Background(), participantID, pathID, target.ID); err != nil || retained != updated.Activity {
		t.Fatalf("denied deletion changed target: %+v, %v", retained, err)
	}

	command := application.DeleteActivityCommand{
		ActivityID: target.ID, PathID: pathID, ParticipantID: participantID,
		Idempotency: idempotency(participantID, application.DeleteActivityOperation, "delete-owned-key", 45),
		Audit:       activityAudit("delete-owned-audit", participantID, target.ID, audit.ResourceDeleted, now.Add(3*time.Second)),
	}
	deleted, err := repository.DeleteActivity(context.Background(), command)
	if err != nil || deleted.Replayed || deleted.AccumulatedSeconds != unrelated.DurationSeconds() || deleted.SessionCount != 1 || deleted.UnreadNotificationCount != 0 || !slices.Equal(deleted.RemovedFeedEventIDs, []string{"practice:" + target.ID}) {
		t.Fatalf("DeleteActivity() = %+v, %v", deleted, err)
	}
	if _, _, err := repository.GetActivity(context.Background(), participantID, pathID, target.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("deleted GetActivity() error = %v", err)
	}
	assertPracticeFeedEvent(t, db, target, 0)
	assertPracticeFeedEvent(t, db, unrelated, 1)
	if _, err := repository.ListActivityRevisions(context.Background(), participantID, pathID, target.ID, application.ActivityRevisionPageRequest{Limit: 25, Snapshot: now.Add(time.Minute)}); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("deleted ListActivityRevisions() error = %v", err)
	}
	if retained, _, err := repository.GetActivity(context.Background(), participantID, pathID, unrelated.ID); err != nil || retained != unrelated {
		t.Fatalf("unrelated activity changed: %+v, %v", retained, err)
	}

	record("delete-later", now.Add(-5*time.Second), 5, "later private", "delete-create-later", 46)
	replayed, err := repository.DeleteActivity(context.Background(), command)
	if err != nil || replayed.AccumulatedSeconds != 20 || replayed.SessionCount != 1 || replayed.UnreadNotificationCount != 0 || !slices.Equal(replayed.RemovedFeedEventIDs, []string{"practice:" + target.ID}) || !replayed.Replayed {
		t.Fatalf("replayed DeleteActivity() = %+v, %v", replayed, err)
	}
	conflict := command
	conflict.ActivityID = unrelated.ID
	conflict.Idempotency.RequestHash = bytes.Repeat([]byte{47}, 32)
	conflict.Audit.TargetID = unrelated.ID
	if _, err := repository.DeleteActivity(context.Background(), conflict); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("conflicting delete replay error = %v", err)
	}

	var tombstones []mutationModel
	if err := db.Where("participant_id = ? AND result_activity_id = ?", participantID, target.ID).Order("operation, key").Find(&tombstones).Error; err != nil {
		t.Fatal(err)
	}
	if len(tombstones) != 3 {
		t.Fatalf("target mutation tombstones = %+v", tombstones)
	}
	for _, row := range tombstones {
		if !row.ResultActivityDeleted || row.ResultActivitySaved || row.ResultStartedAt != nil || row.ResultEndedAt != nil || row.ResultTimeZone != "" || row.ResultCreatedAt != nil || row.ResultUpdatedAt != nil || row.ResultNote != nil || row.ResultVersion != nil {
			t.Fatalf("mutation retained deleted activity snapshot: %+v", row)
		}
		if row.Operation == application.DeleteActivityOperation {
			if row.ResultAccumulatedSeconds == nil || *row.ResultAccumulatedSeconds != 20 || row.ResultSessionCount == nil || *row.ResultSessionCount != 1 || row.ResultUnreadNotificationCount == nil || *row.ResultUnreadNotificationCount != 0 || !slices.Equal([]string(row.ResultRemovedFeedEventIDs), []string{"practice:" + target.ID}) {
				t.Fatalf("delete tombstone total = %+v", row.ResultAccumulatedSeconds)
			}
		} else if row.ResultAccumulatedSeconds != nil {
			t.Fatalf("prior mutation retained deletion total: %+v", row)
		}
	}
	var deletionAudits int64
	if err := db.Model(&auditModel{}).Where("id = ? AND action = ?", command.Audit.ID, audit.ResourceDeleted).Count(&deletionAudits).Error; err != nil || deletionAudits != 1 {
		t.Fatalf("deletion audits = %d, %v", deletionAudits, err)
	}
}

func TestPostgresDeleteActivityCascadesEverySocialDerivativeAndKeepsDeletionTerminal(t *testing.T) {
	db := postgresDB(t, false)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "delete-social-participant", "delete-social-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	seedAchievementPreference(t, db, participantID, "Etc/UTC", now)
	configureAchievementGoals(t, db, pathID, 60, 60)
	repository := New(db)

	unrelated := manualAchievementActivity(t, "delete-social-unrelated", participantID, pathID, now.Add(-4*time.Minute), 10, now)
	unrelatedCreate := application.CreateManualActivityCommand{
		Activity: unrelated, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "delete-social-unrelated-create", 91),
		Audit: activityAudit("delete-social-unrelated-audit", participantID, unrelated.ID, audit.ResourceCreated, now),
	}
	if _, err := repository.CreateManualActivity(context.Background(), unrelatedCreate); err != nil {
		t.Fatal(err)
	}
	target := manualAchievementActivity(t, "delete-social-target", participantID, pathID, now.Add(-2*time.Minute), 50, now.Add(time.Second))
	targetCreate := application.CreateManualActivityCommand{
		Activity: target, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, "delete-social-target-create", 92),
		Audit: activityAudit("delete-social-target-create-audit", participantID, target.ID, audit.ResourceCreated, target.CreatedAt),
	}
	if _, err := repository.CreateManualActivity(context.Background(), targetCreate); err != nil {
		t.Fatal(err)
	}
	targetEdit := application.UpdateActivityCommand{
		ActivityID: target.ID, PathID: pathID, ParticipantID: participantID,
		Edit:      domain.ActivityEdit{StartedAt: target.StartedAt, DurationSeconds: 50, OccurrenceTimeZone: target.OccurrenceTimeZone, Note: "private edit"},
		UpdatedAt: now.Add(2 * time.Second), Idempotency: idempotency(participantID, application.UpdateActivityOperation, "delete-social-target-edit", 93),
		Audit: activityAudit("delete-social-target-edit-audit", participantID, target.ID, audit.ResourceUpdated, now.Add(2*time.Second)),
	}
	if _, err := repository.UpdateActivity(context.Background(), targetEdit); err != nil {
		t.Fatal(err)
	}

	targetEventID, unrelatedEventID := "practice:"+target.ID, "practice:"+unrelated.ID
	seedActivityDeletionSocialDerivatives(t, db, participantID, pathID, targetEventID, "target", now)
	seedActivityDeletionSocialDerivatives(t, db, participantID, pathID, unrelatedEventID, "unrelated", now)
	hiddenActorID := "delete-social-hidden-actor"
	seedParticipantAndPath(t, db, hiddenActorID, "delete-social-hidden-path", now)
	if err := db.Table("notification_models").Create(map[string]any{
		"id": "delete-social-hidden-notification", "recipient_user_id": participantID, "actor_user_id": hiddenActorID,
		"follow_subject_user_id": hiddenActorID, "kind": "new_follower",
		"presentation_class": "informational", "channel": "following", "created_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("block_models").Create(map[string]any{"blocker_user_id": participantID, "blocked_user_id": hiddenActorID, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	achievements := loadGoalAchievements(t, db, participantID, pathID)
	if len(achievements) != 2 {
		t.Fatalf("achievements before deletion=%+v want interval and overall", achievements)
	}
	achievementEventIDs := make([]string, 0, len(achievements))
	for index, achievement := range achievements {
		eventID := "achievement:" + achievement.ID
		achievementEventIDs = append(achievementEventIDs, eventID)
		seedActivityDeletionSocialDerivatives(t, db, participantID, pathID, eventID, "achievement-"+string(rune('a'+index)), now)
	}

	deleted, err := repository.DeleteActivity(context.Background(), application.DeleteActivityCommand{
		ActivityID: target.ID, PathID: pathID, ParticipantID: participantID,
		Idempotency: idempotency(participantID, application.DeleteActivityOperation, "delete-social-target-delete", 94),
		Audit:       activityAudit("delete-social-target-delete-audit", participantID, target.ID, audit.ResourceDeleted, now.Add(3*time.Second)),
	})
	wantRemovedEventIDs := append([]string(nil), achievementEventIDs...)
	wantRemovedEventIDs = append(wantRemovedEventIDs, targetEventID)
	slices.Sort(wantRemovedEventIDs)
	if err != nil || deleted.AccumulatedSeconds != unrelated.DurationSeconds() || deleted.SessionCount != 1 || deleted.UnreadNotificationCount != 3 || !slices.Equal(deleted.RemovedFeedEventIDs, wantRemovedEventIDs) {
		t.Fatalf("DeleteActivity()=%+v err=%v", deleted, err)
	}

	assertActivityDeletionSocialDerivativeCounts(t, db, targetEventID, "target", 0)
	for index, eventID := range achievementEventIDs {
		assertActivityDeletionSocialDerivativeCounts(t, db, eventID, "achievement-"+string(rune('a'+index)), 0)
	}
	assertActivityDeletionSocialDerivativeCounts(t, db, unrelatedEventID, "unrelated", 1)
	if got := loadGoalAchievements(t, db, participantID, pathID); len(got) != 0 {
		t.Fatalf("unsupported achievements survived deletion: %+v", got)
	}
	var notificationCount int64
	if err := db.Table("notification_models").Where("recipient_user_id = ?", participantID).Count(&notificationCount).Error; err != nil || notificationCount != 4 {
		t.Fatalf("remaining notifications=%d err=%v want three visible and one blocked notification", notificationCount, err)
	}
	if _, err := repository.CreateManualActivity(context.Background(), targetCreate); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("deleted create replay error=%v want not found", err)
	}
	if _, err := repository.UpdateActivity(context.Background(), targetEdit); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("deleted edit replay error=%v want not found", err)
	}
}

func seedActivityDeletionSocialDerivatives(t *testing.T, db *gorm.DB, participantID, pathID, eventID, suffix string, now time.Time) {
	t.Helper()
	commentID := "delete-social-comment-" + suffix
	if err := db.Table("social_practice_reaction_models").Create(map[string]any{"social_feed_event_id": eventID, "actor_user_id": participantID, "reaction_type": "heart", "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("social_practice_reaction_replay_models").Create(map[string]any{"actor_user_id": participantID, "operation": "social.practice_reaction.set", "idempotency_key": "delete-social-reaction-" + suffix, "request_hash": bytes.Repeat([]byte{101}, 32), "social_feed_event_id": eventID, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("social_practice_comment_models").Create(map[string]any{"id": commentID, "social_feed_event_id": eventID, "author_user_id": participantID, "body": "delete me", "version": 1, "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("social_practice_comment_revision_models").Create(map[string]any{"comment_id": commentID, "version": 1, "body": "delete me", "changed_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("social_practice_comment_heart_models").Create(map[string]any{"comment_id": commentID, "actor_user_id": participantID, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("social_practice_comment_replay_models").Create(map[string]any{"actor_user_id": participantID, "operation": "social.practice_comment.create", "idempotency_key": "delete-social-comment-" + suffix, "request_hash": bytes.Repeat([]byte{102}, 32), "comment_id": commentID, "social_feed_event_id": eventID, "author_user_id": participantID, "result_body": "delete me", "result_version": 1, "result_created_at": now, "result_updated_at": now, "result_deleted": false, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("social_practice_comment_heart_replay_models").Create(map[string]any{"actor_user_id": participantID, "operation": "social.practice_comment_heart.set", "idempotency_key": "delete-social-heart-" + suffix, "request_hash": bytes.Repeat([]byte{103}, 32), "social_feed_event_id": eventID, "comment_id": commentID, "result_hearted": true, "result_heart_count": 1, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	for _, notification := range []struct{ id, kind, channel string }{{"reaction", "practice_reaction", "reactions"}, {"comment", "practice_comment", "comments"}, {"heart", "comment_heart", "comment_hearts"}} {
		row := map[string]any{"id": "delete-social-" + notification.id + "-" + suffix, "recipient_user_id": participantID, "actor_user_id": participantID, "path_id": pathID, "social_feed_event_id": eventID, "kind": notification.kind, "presentation_class": "informational", "channel": notification.channel, "created_at": now}
		if notification.kind == "practice_reaction" {
			row["reaction_type"] = "heart"
		} else {
			row["comment_id"] = commentID
		}
		if err := db.Table("notification_models").Create(row).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Table("notification_push_outbox_models").Create(map[string]any{"notification_id": row["id"], "created_at": now}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func assertActivityDeletionSocialDerivativeCounts(t *testing.T, db *gorm.DB, eventID, suffix string, want int64) {
	t.Helper()
	for _, table := range []string{"social_feed_event_models", "social_practice_reaction_models", "social_practice_reaction_replay_models", "social_practice_comment_models", "social_practice_comment_replay_models", "social_practice_comment_heart_replay_models", "notification_models"} {
		var count int64
		eventColumn := "social_feed_event_id"
		if table == "social_feed_event_models" {
			eventColumn = "id"
		}
		if err := db.Table(table).Where(eventColumn+" = ?", eventID).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		expected := want
		if table == "notification_models" {
			expected *= 3
		}
		if count != expected {
			t.Fatalf("%s rows for %s=%d want %d", table, eventID, count, expected)
		}
	}
	commentID := "delete-social-comment-" + suffix
	for _, table := range []string{"social_practice_comment_revision_models", "social_practice_comment_heart_models"} {
		var count int64
		if err := db.Table(table).Where("comment_id = ?", commentID).Count(&count).Error; err != nil || count != want {
			t.Fatalf("%s rows for %s=%d err=%v want %d", table, eventID, count, err, want)
		}
	}
	var outboxCount int64
	notificationIDs := []string{"delete-social-reaction-" + suffix, "delete-social-comment-" + suffix, "delete-social-heart-" + suffix}
	if err := db.Table("notification_push_outbox_models").Where("notification_id IN ?", notificationIDs).Count(&outboxCount).Error; err != nil || outboxCount != want*3 {
		t.Fatalf("notification outbox rows for %s=%d err=%v want %d", eventID, outboxCount, err, want*3)
	}
}

func TestPostgresDeleteActivityRollsBackWhenAuditCannotPersist(t *testing.T) {
	db := postgresDB(t, false)
	now := time.Now().UTC().Truncate(time.Microsecond)
	participantID, pathID := "delete-rollback-participant", "delete-rollback-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	repository := New(db)
	record := func(id, key, note string, startedAt time.Time, fill byte) domain.RecordedActivity {
		t.Helper()
		entry, err := domain.RecordManualActivity(domain.ManualActivity{ID: id, PathID: pathID, ParticipantID: participantID, StartedAt: startedAt, DurationSeconds: 5, OccurrenceTimeZone: "Etc/UTC", Note: note}, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repository.CreateManualActivity(context.Background(), application.CreateManualActivityCommand{Activity: entry, Idempotency: idempotency(participantID, application.CreateManualActivityOperation, key, fill), Audit: activityAudit(key+"-audit", participantID, entry.ID, audit.ResourceCreated, now)}); err != nil {
			t.Fatal(err)
		}
		return entry
	}
	entry := record("delete-rollback-target", "delete-rollback-create", "must survive", now.Add(-10*time.Second), 48)
	unrelated := record("delete-rollback-unrelated", "delete-rollback-unrelated-create", "must remain unchanged", now.Add(-20*time.Second), 50)
	assertPracticeFeedEvent(t, db, entry, 1)
	assertPracticeFeedEvent(t, db, unrelated, 1)
	duplicateAudit := activityAudit("delete-rollback-audit", participantID, "other-target", audit.ResourceDeleted, now)
	if err := db.Create(fromAudit(duplicateAudit)).Error; err != nil {
		t.Fatal(err)
	}
	command := application.DeleteActivityCommand{ActivityID: entry.ID, PathID: pathID, ParticipantID: participantID, Idempotency: idempotency(participantID, application.DeleteActivityOperation, "delete-rollback-key", 49), Audit: activityAudit(duplicateAudit.ID, participantID, entry.ID, audit.ResourceDeleted, now.Add(time.Second))}
	if _, err := repository.DeleteActivity(context.Background(), command); err == nil {
		t.Fatal("DeleteActivity() succeeded despite duplicate audit ID")
	}
	if retained, _, err := repository.GetActivity(context.Background(), participantID, pathID, entry.ID); err != nil || retained != entry {
		t.Fatalf("failed deletion did not roll back target: %+v, %v", retained, err)
	}
	if retained, _, err := repository.GetActivity(context.Background(), participantID, pathID, unrelated.ID); err != nil || retained != unrelated {
		t.Fatalf("failed deletion changed unrelated activity: %+v, %v", retained, err)
	}
	assertPracticeFeedEvent(t, db, entry, 1)
	assertPracticeFeedEvent(t, db, unrelated, 1)
	var reservationCount int64
	if err := db.Model(&mutationModel{}).Where("participant_id = ? AND operation = ? AND key = ?", participantID, application.DeleteActivityOperation, command.Idempotency.Key).Count(&reservationCount).Error; err != nil || reservationCount != 0 {
		t.Fatalf("failed deletion retained %d reservations: %v", reservationCount, err)
	}
}
