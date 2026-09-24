package pathstore

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresLeaveAtomicallyStopsAndRetainsTimerActivityThenHidesMembershipScopedData(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC)
	owner, administrator, participant := "path09-owner", "path09-admin", "path09-participant"
	pathID := "path09-path"
	users := []string{owner, administrator, participant}
	cleanupDeletionFixture(migrationDB, users, []string{pathID})
	t.Cleanup(func() { cleanupDeletionFixture(migrationDB, users, []string{pathID}) })
	seedGoalUpdateUsers(t, migrationDB, now, users...)
	path := domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Shared practice", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	seedGoalUpdatePath(t, migrationDB, path, owner)
	if err := migrationDB.Create(&[]membershipModel{{PathID: pathID, UserID: administrator, Role: "administrator"}, {PathID: pathID, UserID: participant, Role: "participant"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Create(&archiveTimerRow{ID: "path09-timer", PathID: pathID, ParticipantID: participant, StartedAt: now.Add(-90 * time.Second), OccurrenceTimeZone: "Etc/UTC"}).Error; err != nil {
		t.Fatal(err)
	}

	sequence := 0
	command := application.LeavePathCommand{ActorUserID: participant, PathID: path.ID, LeftAt: now, RetainActivity: true, Idempotency: ports.Idempotency{PrincipalID: participant, Operation: application.LeavePathOperation, Key: "path09-leave-key-0001", RequestHash: bytes.Repeat([]byte{9}, 32)}, Audit: audit.Event{ID: "path09-audit", OwnerUserID: participant, ActorUserID: participant, Action: audit.ResourceUpdated, TargetType: "path", TargetID: pathID, Outcome: audit.Succeeded, CorrelationID: "path09", OccurredAt: now}, NewID: func() string { sequence++; return fmt.Sprintf("path09-generated-%02d", sequence) }, AuthorizationWorker: "path09-worker", AuthorizationLease: time.Minute}
	result, err := New(runtimeDB).LeavePath(context.Background(), command)
	if err != nil || !result.Left || !result.ActivityRetained || result.Replayed || len(result.AuthorizationChanges) != 1 || result.AuthorizationChanges[0].Relation != "participant" {
		t.Fatalf("LeavePath() = %+v, %v", result, err)
	}

	for table, predicate := range map[string]string{"path_membership_models": "path_id = ? AND user_id = ?", "running_timer_models": "path_id = ? AND participant_id = ?"} {
		var count int64
		if err := migrationDB.Table(table).Where(predicate, pathID, participant).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s count=%d, %v", table, count, err)
		}
	}
	var retained int64
	if err := migrationDB.Table("recorded_activity_models").Where("path_id = ? AND participant_id = ?", pathID, participant).Count(&retained).Error; err != nil || retained != 1 {
		t.Fatalf("retained activity=%d, %v", retained, err)
	}
	if _, err := New(runtimeDB).Get(context.Background(), participant, path.ID); err == nil {
		t.Fatal("departed participant retained private Path access")
	}
	var notices, pushes int64
	if err := migrationDB.Table("notification_models").Where("path_id = ? AND kind = ? AND recipient_user_id IN ?", pathID, application.NotificationPathMemberLeft, []string{owner, administrator}).Count(&notices).Error; err != nil || notices != 2 {
		t.Fatalf("notices=%d, %v", notices, err)
	}
	if err := migrationDB.Table("notification_push_outbox_models AS push").Joins("JOIN notification_models AS notification ON notification.id = push.notification_id").Where("notification.path_id = ? AND notification.kind = ?", pathID, application.NotificationPathMemberLeft).Count(&pushes).Error; err != nil || pushes != 2 {
		t.Fatalf("pushes=%d, %v", pushes, err)
	}

	replay, err := New(runtimeDB).LeavePath(context.Background(), command)
	if err != nil || !replay.Replayed || !replay.Left {
		t.Fatalf("replay = %+v, %v", replay, err)
	}
	if err := migrationDB.Table("notification_models").Where("path_id = ? AND kind = ?", pathID, application.NotificationPathMemberLeft).Count(&notices).Error; err != nil || notices != 2 {
		t.Fatalf("replay notices=%d, %v", notices, err)
	}

	if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: participant, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	secondLeave := command
	secondLeave.Idempotency.Key = "path09-leave-key-0002"
	secondLeave.Idempotency.RequestHash = bytes.Repeat([]byte{8}, 32)
	secondLeave.Audit.ID = "path09-audit-second"
	second, err := New(runtimeDB).LeavePath(context.Background(), secondLeave)
	if err != nil || !second.Left || second.Replayed {
		t.Fatalf("second LeavePath() after rejoin = %+v, %v", second, err)
	}
	if err := migrationDB.Table("notification_models").Where("path_id = ? AND kind = ?", pathID, application.NotificationPathMemberLeft).Count(&notices).Error; err != nil || notices != 4 {
		t.Fatalf("second leave notices=%d, %v", notices, err)
	}
}

func TestPostgresLeaveWithDeletionAtomicallyDiscardsTimerAndDeletesOnlyOwnedPathData(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	var legacyRemoval struct{ Removed bool }
	legacyErr := runtimeDB.Table("public.leave_path_membership(?, ?) AS removal(removed)", "legacy-path", "legacy-member").Select("removed").Take(&legacyRemoval).Error
	if legacyErr == nil || !strings.Contains(legacyErr.Error(), "does not exist") {
		t.Fatalf("obsolete privileged leave function remains executable by app: %v", legacyErr)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, administrator, participant := "leave-delete-owner-"+suffix, "leave-delete-admin-"+suffix, "leave-delete-member-"+suffix
	pathID, otherPathID := "leave-delete-path-"+suffix, "leave-delete-other-"+suffix
	users := []string{owner, administrator, participant}
	t.Cleanup(func() { cleanupDeletionFixture(migrationDB, users, []string{pathID, otherPathID}) })
	seedGoalUpdateUsers(t, migrationDB, now, users...)
	for _, path := range []domain.Entity{
		{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Delete on leave", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
		{ID: domain.ID(otherPathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Keep", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
	} {
		seedGoalUpdatePath(t, migrationDB, path, owner)
	}
	if err := migrationDB.Create(&[]membershipModel{
		{PathID: pathID, UserID: administrator, Role: "administrator"},
		{PathID: pathID, UserID: participant, Role: "participant"},
		{PathID: otherPathID, UserID: participant, Role: "participant"},
	}).Error; err != nil {
		t.Fatal(err)
	}

	for _, currentPath := range []string{pathID, otherPathID} {
		activityID := "leave-delete-activity-" + currentPath
		eventID := "practice:" + activityID
		commentID := "leave-delete-comment-" + currentPath
		if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{"id": activityID, "path_id": currentPath, "participant_id": participant, "started_at": now.Add(-2 * time.Minute), "ended_at": now.Add(-time.Minute), "occurrence_time_zone": "Etc/UTC", "created_at": now.Add(-time.Minute), "updated_at": now.Add(-time.Minute)}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("recorded_activity_revision_models").Create(map[string]any{"activity_id": activityID, "version": 1, "started_at": now.Add(-3 * time.Minute), "ended_at": now.Add(-2 * time.Minute), "occurrence_time_zone": "Etc/UTC", "public_changed": false, "updated_at": now.Add(-2 * time.Minute), "replaced_at": now.Add(-time.Minute)}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_comment_models").Create(map[string]any{"id": commentID, "social_feed_event_id": eventID, "author_user_id": owner, "body": "Practice", "version": 1, "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_comment_replay_models").Create(map[string]any{"actor_user_id": owner, "operation": "social.practice_comment.create", "idempotency_key": "comment-replay-" + currentPath, "request_hash": bytes.Repeat([]byte{4}, 32), "comment_id": commentID, "social_feed_event_id": eventID, "author_user_id": owner, "result_body": "Practice", "result_version": 1, "result_created_at": now, "result_updated_at": now, "result_deleted": false, "created_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_comment_heart_models").Create(map[string]any{"comment_id": commentID, "actor_user_id": administrator, "created_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_comment_heart_replay_models").Create(map[string]any{"actor_user_id": administrator, "operation": "social.practice_comment_heart.set", "idempotency_key": "heart-replay-" + currentPath, "request_hash": bytes.Repeat([]byte{5}, 32), "social_feed_event_id": eventID, "comment_id": commentID, "result_hearted": true, "result_heart_count": 1, "created_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("running_timer_models").Create(map[string]any{"id": "leave-delete-timer-" + currentPath, "path_id": currentPath, "participant_id": participant, "started_at": now.Add(-30 * time.Second), "occurrence_time_zone": "Etc/UTC"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrationDB.Table("activity_mutation_models").Create(map[string]any{"participant_id": participant, "operation": "activity.timer.start", "key": "leave-delete-mutation", "request_hash": bytes.Repeat([]byte{6}, 32), "timer_id": "leave-delete-timer-" + pathID, "path_id": pathID, "result_started_at": now.Add(-30 * time.Second), "result_time_zone": "Etc/UTC"}).Error; err != nil {
		t.Fatal(err)
	}
	achievementID := "leave-delete-achievement-" + pathID
	if err := migrationDB.Table("social_goal_achievement_models").Create(map[string]any{"id": achievementID, "participant_user_id": participant, "path_id": pathID, "kind": "overall", "target_seconds": 60, "published_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("social_feed_event_models").Create(map[string]any{"id": "achievement:" + achievementID, "achievement_id": achievementID, "participant_user_id": participant, "path_id": pathID, "published_at": now}).Error; err != nil {
		t.Fatal(err)
	}

	sequence := 0
	command := application.LeavePathCommand{ActorUserID: participant, PathID: domain.ID(pathID), LeftAt: now, RetainActivity: false, Idempotency: ports.Idempotency{PrincipalID: participant, Operation: application.LeavePathOperation, Key: "leave-delete-key-0001", RequestHash: bytes.Repeat([]byte{7}, 32)}, Audit: audit.Event{ID: "leave-delete-audit-" + suffix, OwnerUserID: participant, ActorUserID: participant, Action: audit.ResourceUpdated, TargetType: "path", TargetID: pathID, Outcome: audit.Succeeded, CorrelationID: "leave-delete-" + suffix, OccurredAt: now}, NewID: func() string { sequence++; return fmt.Sprintf("leave-delete-id-%s-%d", suffix, sequence) }, AuthorizationWorker: "leave-delete-worker", AuthorizationLease: time.Minute}
	rollback := command
	rollback.Idempotency.Key = "leave-delete-rollback-0001"
	rollback.Idempotency.RequestHash = bytes.Repeat([]byte{8}, 32)
	rollback.Audit.ID = "leave-delete-rollback-audit-" + suffix
	if err := migrationDB.Create(fromAudit(rollback.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := New(runtimeDB).LeavePath(context.Background(), rollback); err == nil {
		t.Fatal("injected post-delete audit collision succeeded")
	}
	for table, predicate := range map[string]string{"path_membership_models": "path_id = ? AND user_id = ?", "running_timer_models": "path_id = ? AND participant_id = ?", "recorded_activity_models": "path_id = ? AND participant_id = ?"} {
		var count int64
		if err := migrationDB.Table(table).Where(predicate, pathID, participant).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("rollback %s count=%d err=%v", table, count, err)
		}
	}

	result, err := New(runtimeDB).LeavePath(context.Background(), command)
	if err != nil || !result.Left || result.ActivityRetained || result.Replayed || len(result.AuthorizationChanges) != 1 || result.AuthorizationChanges[0].Relation != "participant" {
		t.Fatalf("LeavePath()=%+v err=%v", result, err)
	}
	for table, predicate := range map[string]string{
		"path_membership_models":         "path_id = ? AND user_id = ?",
		"running_timer_models":           "path_id = ? AND participant_id = ?",
		"activity_mutation_models":       "path_id = ? AND participant_id = ?",
		"recorded_activity_models":       "path_id = ? AND participant_id = ?",
		"social_goal_achievement_models": "path_id = ? AND participant_user_id = ?",
		"social_feed_event_models":       "path_id = ? AND participant_user_id = ?",
	} {
		var count int64
		if err := migrationDB.Table(table).Where(predicate, pathID, participant).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("deleted %s count=%d err=%v", table, count, err)
		}
	}
	for _, table := range []string{"social_practice_comment_replay_models", "social_practice_comment_heart_replay_models"} {
		var removed, retained int64
		if err := migrationDB.Table(table).Where("social_feed_event_id = ?", "practice:leave-delete-activity-"+pathID).Count(&removed).Error; err != nil || removed != 0 {
			t.Fatalf("%s removed replay count=%d err=%v", table, removed, err)
		}
		if err := migrationDB.Table(table).Where("social_feed_event_id = ?", "practice:leave-delete-activity-"+otherPathID).Count(&retained).Error; err != nil || retained != 1 {
			t.Fatalf("%s retained replay count=%d err=%v", table, retained, err)
		}
	}
	for table, predicate := range map[string]string{"path_membership_models": "path_id = ? AND user_id = ?", "running_timer_models": "path_id = ? AND participant_id = ?", "recorded_activity_models": "path_id = ? AND participant_id = ?"} {
		var count int64
		if err := migrationDB.Table(table).Where(predicate, otherPathID, participant).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("other Path %s count=%d err=%v", table, count, err)
		}
	}
	var notices, audits, outbox int64
	if err := migrationDB.Table("notification_models").Where("path_id = ? AND kind = ?", pathID, application.NotificationPathMemberLeft).Count(&notices).Error; err != nil || notices != 2 {
		t.Fatalf("notices=%d err=%v", notices, err)
	}
	if err := migrationDB.Table("audit_event_models").Where("id = ?", command.Audit.ID).Count(&audits).Error; err != nil || audits != 1 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
	if err := migrationDB.Table("authorization_outbox_models").Where("resource_id = ? AND subject_id = ? AND operation = ?", pathID, participant, ports.AuthorizationDelete).Count(&outbox).Error; err != nil || outbox != 1 {
		t.Fatalf("outbox=%d err=%v", outbox, err)
	}
	replay, err := New(runtimeDB).LeavePath(context.Background(), command)
	if err != nil || !replay.Replayed || replay.ActivityRetained {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	conflict := command
	conflict.Idempotency.RequestHash = bytes.Repeat([]byte{9}, 32)
	if _, err := New(runtimeDB).LeavePath(context.Background(), conflict); err != ports.ErrIdempotencyConflict {
		t.Fatalf("changed-choice replay error=%v want idempotency conflict", err)
	}
}
