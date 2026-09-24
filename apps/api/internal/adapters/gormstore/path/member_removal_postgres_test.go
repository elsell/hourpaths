package pathstore

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresRemoveMemberAtomicallyDeletesScopedActivityDerivativesAndPreservesOtherPath(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, administrator, member := "remove-owner-"+suffix, "remove-admin-"+suffix, "remove-member-"+suffix
	pathID, otherPathID := "remove-path-"+suffix, "remove-other-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedGoalUpdateUsers(t, migrationDB, now, owner, administrator, member)
	for userID, username := range map[string]string{
		owner:         "ro" + suffix,
		administrator: "ra" + suffix,
		member:        "rm" + suffix,
	} {
		if err := migrationDB.Table("user_models").Where("id = ?", userID).Update("username", username).Error; err != nil {
			t.Fatal(err)
		}
	}
	seedGoalUpdatePath(t, migrationDB, domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Remove", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}, owner)
	seedGoalUpdatePath(t, migrationDB, domain.Entity{ID: domain.ID(otherPathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Keep", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}, owner)
	if err := migrationDB.Create(&[]membershipModel{{PathID: pathID, UserID: administrator, Role: "administrator"}, {PathID: pathID, UserID: member, Role: "participant"}, {PathID: otherPathID, UserID: member, Role: "participant"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("user_models").Where("id = ?", member).Update("status", identity.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	review, err := New(runtimeDB).ReviewMemberRemoval(context.Background(), application.MemberRemovalQuery{ActorUserID: administrator, TargetUserID: member, PathID: domain.ID(pathID)})
	if err != nil || review.UserID != member || review.Username == "" {
		t.Fatalf("disabled retained member review=%+v err=%v", review, err)
	}
	if err := migrationDB.Table("path_models").Where("id = ?", otherPathID).Update("archived_at", now).Error; err != nil {
		t.Fatal(err)
	}
	archivedPage, err := New(runtimeDB).ListMembers(context.Background(), application.MemberListQuery{ActorUserID: owner, PathID: domain.ID(otherPathID)}, application.MemberPageRequest{Limit: 25, Snapshot: now.Add(time.Second)})
	if err != nil || len(archivedPage.Items) != 2 {
		t.Fatalf("archived member roster=%+v err=%v", archivedPage, err)
	}
	t.Cleanup(func() {
		cleanupDeletionFixture(migrationDB, []string{owner, administrator, member}, []string{pathID, otherPathID})
	})
	for _, currentPath := range []string{pathID, otherPathID} {
		activityID := "activity-" + currentPath
		eventID := "practice:" + activityID
		commentID := "comment-" + currentPath
		if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{"id": activityID, "path_id": currentPath, "participant_id": member, "started_at": now.Add(-time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("recorded_activity_revision_models").Create(map[string]any{"activity_id": activityID, "version": 1, "started_at": now.Add(-2 * time.Minute), "ended_at": now.Add(-time.Minute), "occurrence_time_zone": "Etc/UTC", "public_changed": false, "updated_at": now.Add(-time.Minute), "replaced_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_reaction_models").Create(map[string]any{"social_feed_event_id": eventID, "actor_user_id": owner, "reaction_type": "heart", "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_comment_models").Create(map[string]any{"id": commentID, "social_feed_event_id": eventID, "author_user_id": owner, "body": "Keep practicing", "version": 1, "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_comment_heart_models").Create(map[string]any{"comment_id": commentID, "actor_user_id": owner, "created_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("notification_models").Create(map[string]any{"id": "reaction-notice-" + currentPath, "recipient_user_id": member, "actor_user_id": owner, "path_id": currentPath, "social_feed_event_id": eventID, "reaction_type": "heart", "kind": "practice_reaction", "presentation_class": "informational", "channel": "reactions", "created_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("running_timer_models").Create(map[string]any{"id": "timer-" + currentPath, "path_id": currentPath, "participant_id": member, "started_at": now, "occurrence_time_zone": "Etc/UTC"}).Error; err != nil {
			t.Fatal(err)
		}

		achievementID := "achievement-" + currentPath
		achievementEventID := "achievement:" + achievementID
		achievementCommentID := "achievement-comment-" + currentPath
		if err := migrationDB.Table("social_goal_achievement_models").Create(map[string]any{"id": achievementID, "participant_user_id": member, "path_id": currentPath, "kind": "overall", "target_seconds": 60, "published_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_feed_event_models").Create(map[string]any{"id": achievementEventID, "achievement_id": achievementID, "participant_user_id": member, "path_id": currentPath, "published_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_reaction_models").Create(map[string]any{"social_feed_event_id": achievementEventID, "actor_user_id": owner, "reaction_type": "applause", "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_comment_models").Create(map[string]any{"id": achievementCommentID, "social_feed_event_id": achievementEventID, "author_user_id": owner, "body": "Goal reached", "version": 1, "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("social_practice_comment_heart_models").Create(map[string]any{"comment_id": achievementCommentID, "actor_user_id": owner, "created_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationDB.Table("notification_models").Create(map[string]any{"id": "achievement-notice-" + currentPath, "recipient_user_id": member, "actor_user_id": owner, "path_id": currentPath, "social_feed_event_id": achievementEventID, "reaction_type": "applause", "kind": "practice_reaction", "presentation_class": "informational", "channel": "reactions", "created_at": now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrationDB.Table("activity_mutation_models").Create(map[string]any{"participant_id": member, "operation": "activity.timer.start", "key": "queued-remove-key", "request_hash": bytes.Repeat([]byte{7}, 32), "timer_id": "timer-" + pathID, "path_id": pathID, "result_started_at": now, "result_time_zone": "Etc/UTC"}).Error; err != nil {
		t.Fatal(err)
	}
	sequence := 0
	command := application.RemoveMemberCommand{ActorUserID: administrator, TargetUserID: member, PathID: domain.ID(pathID), ExpectedRole: domain.RoleParticipant, RemovedAt: now, Idempotency: ports.Idempotency{PrincipalID: administrator, Operation: application.RemoveMemberOperation, Key: "remove-member-key-0001", RequestHash: bytes.Repeat([]byte{8}, 32)}, Audit: audit.Event{ID: "audit-" + suffix, OwnerUserID: administrator, ActorUserID: administrator, Action: audit.ResourceUpdated, TargetType: "path_member", TargetID: pathID + ":" + member, Outcome: audit.Succeeded, CorrelationID: "remove-" + suffix, OccurredAt: now}, Notification: application.MemberAccessNotification{ID: "remove-notification-" + suffix, RecipientUserID: member, ActorUserID: administrator, PathID: domain.ID(pathID), Kind: application.NotificationPathMemberRemoved, Role: domain.RoleParticipant, CreatedAt: now}, NewID: func() string { sequence++; return fmt.Sprintf("remove-id-%s-%d", suffix, sequence) }, AuthorizationWorker: "remove-test", AuthorizationLease: time.Minute}
	failure := command
	failure.Idempotency.Key = "remove-member-rollback-0001"
	failure.Idempotency.RequestHash = bytes.Repeat([]byte{9}, 32)
	failure.Audit.ID = "rollback-audit-" + suffix
	if err := migrationDB.Create(fromAudit(failure.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := New(runtimeDB).RemoveMember(context.Background(), failure); err == nil {
		t.Fatal("injected post-delete audit collision succeeded")
	}
	var rollbackMemberships, rollbackActivities int64
	if err := migrationDB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, member).Count(&rollbackMemberships).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("recorded_activity_models").Where("path_id = ? AND participant_id = ?", pathID, member).Count(&rollbackActivities).Error; err != nil {
		t.Fatal(err)
	}
	if rollbackMemberships != 1 || rollbackActivities != 1 {
		t.Fatalf("rollback membership=%d activity=%d", rollbackMemberships, rollbackActivities)
	}
	result, err := New(runtimeDB).RemoveMember(context.Background(), command)
	if err != nil || !result.Removed || !result.ActivityDeleted {
		t.Fatalf("RemoveMember()=%+v err=%v", result, err)
	}
	var removalNotices int64
	if err := migrationDB.Table("notification_models").Where("id = ? AND recipient_user_id = ? AND kind = ? AND presentation_class = ? AND offered_role = ?", command.Notification.ID, member, application.NotificationPathMemberRemoved, application.NotificationInformational, domain.RoleParticipant).Count(&removalNotices).Error; err != nil || removalNotices != 1 {
		t.Fatalf("removal notices=%d err=%v", removalNotices, err)
	}
	var persistedAudit auditModel
	if err := migrationDB.Where("id = ?", command.Audit.ID).Take(&persistedAudit).Error; err != nil {
		t.Fatal(err)
	}
	if persistedAudit.OwnerUserID != owner || persistedAudit.ActorUserID != administrator {
		t.Fatalf("persisted removal audit=%+v", persistedAudit)
	}
	for _, table := range []string{"path_membership_models", "running_timer_models", "activity_mutation_models", "recorded_activity_models", "social_goal_achievement_models", "social_feed_event_models"} {
		var count int64
		if err := migrationDB.Table(table).Where("path_id = ? AND "+memberColumn(table)+" = ?", pathID, member).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s scoped rows=%d err=%v", table, count, err)
		}
	}
	for _, table := range []string{"path_membership_models", "running_timer_models", "recorded_activity_models"} {
		var count int64
		if err := migrationDB.Table(table).Where("path_id = ? AND "+memberColumn(table)+" = ?", otherPathID, member).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s other Path rows=%d err=%v", table, count, err)
		}
	}
	var removedRevisions, retainedRevisions int64
	if err := migrationDB.Table("recorded_activity_revision_models").Where("activity_id = ?", "activity-"+pathID).Count(&removedRevisions).Error; err != nil || removedRevisions != 0 {
		t.Fatalf("removed revisions=%d err=%v", removedRevisions, err)
	}
	if err := migrationDB.Table("recorded_activity_revision_models").Where("activity_id = ?", "activity-"+otherPathID).Count(&retainedRevisions).Error; err != nil || retainedRevisions != 1 {
		t.Fatalf("retained revisions=%d err=%v", retainedRevisions, err)
	}
	for _, expected := range []struct {
		table string
		count int64
	}{{"social_goal_achievement_models", 1}, {"social_feed_event_models", 2}} {
		var count int64
		if err := migrationDB.Table(expected.table).Where("path_id = ? AND participant_user_id = ?", otherPathID, member).Count(&count).Error; err != nil || count != expected.count {
			t.Fatalf("%s other Path rows=%d err=%v", expected.table, count, err)
		}
	}
	for _, eventPrefix := range []string{"practice:activity-", "achievement:achievement-"} {
		for _, table := range []string{"social_practice_reaction_models", "social_practice_comment_models", "notification_models"} {
			var removed, retained int64
			if err := migrationDB.Table(table).Where("social_feed_event_id = ?", eventPrefix+pathID).Count(&removed).Error; err != nil || removed != 0 {
				t.Fatalf("%s removed %s rows=%d err=%v", table, eventPrefix, removed, err)
			}
			if err := migrationDB.Table(table).Where("social_feed_event_id = ?", eventPrefix+otherPathID).Count(&retained).Error; err != nil || retained != 1 {
				t.Fatalf("%s retained %s rows=%d err=%v", table, eventPrefix, retained, err)
			}
		}
	}
	for _, commentPrefix := range []string{"comment-", "achievement-comment-"} {
		var removed, retained int64
		if err := migrationDB.Table("social_practice_comment_heart_models").Where("comment_id = ?", commentPrefix+pathID).Count(&removed).Error; err != nil || removed != 0 {
			t.Fatalf("removed hearts %s=%d err=%v", commentPrefix, removed, err)
		}
		if err := migrationDB.Table("social_practice_comment_heart_models").Where("comment_id = ?", commentPrefix+otherPathID).Count(&retained).Error; err != nil || retained != 1 {
			t.Fatalf("retained hearts %s=%d err=%v", commentPrefix, retained, err)
		}
	}
	replay, err := New(runtimeDB).RemoveMember(context.Background(), command)
	if err != nil || !replay.Replayed {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
}

func TestPostgresRemoveSupporterIsAccessOnlyEvenWithUnexpectedRetainedActivity(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, supporter, pathID := "support-owner-"+suffix, "support-member-"+suffix, "support-path-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedGoalUpdateUsers(t, migrationDB, now, owner, supporter)
	seedGoalUpdatePath(t, migrationDB, domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Support", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}, owner)
	if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: supporter, Role: "supporter"}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupDeletionFixture(migrationDB, []string{owner, supporter}, []string{pathID}) })
	if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{"id": "retained-activity-" + suffix, "path_id": pathID, "participant_id": supporter, "started_at": now.Add(-time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("running_timer_models").Create(map[string]any{"id": "retained-timer-" + suffix, "path_id": pathID, "participant_id": supporter, "started_at": now, "occurrence_time_zone": "Etc/UTC"}).Error; err != nil {
		t.Fatal(err)
	}
	sequence := 0
	command := application.RemoveMemberCommand{ActorUserID: owner, TargetUserID: supporter, PathID: domain.ID(pathID), ExpectedRole: domain.RoleSupporter, RemovedAt: now, Idempotency: ports.Idempotency{PrincipalID: owner, Operation: application.RemoveMemberOperation, Key: "remove-supporter-key-0001", RequestHash: bytes.Repeat([]byte{6}, 32)}, Audit: audit.Event{ID: "support-audit-" + suffix, OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceUpdated, TargetType: "path_member", TargetID: pathID + ":" + supporter, Outcome: audit.Succeeded, CorrelationID: "support-remove-" + suffix, OccurredAt: now}, Notification: application.MemberAccessNotification{ID: "support-notification-" + suffix, RecipientUserID: supporter, ActorUserID: owner, PathID: domain.ID(pathID), Kind: application.NotificationPathMemberRemoved, Role: domain.RoleSupporter, CreatedAt: now}, NewID: func() string { sequence++; return fmt.Sprintf("support-id-%s-%d", suffix, sequence) }, AuthorizationWorker: "remove-test", AuthorizationLease: time.Minute}
	result, err := New(runtimeDB).RemoveMember(context.Background(), command)
	if err != nil || !result.Removed || result.ActivityDeleted {
		t.Fatalf("RemoveMember()=%+v err=%v", result, err)
	}
	var memberships, activities, timers int64
	if err := migrationDB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, supporter).Count(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("recorded_activity_models").Where("path_id = ? AND participant_id = ?", pathID, supporter).Count(&activities).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("running_timer_models").Where("path_id = ? AND participant_id = ?", pathID, supporter).Count(&timers).Error; err != nil {
		t.Fatal(err)
	}
	if memberships != 0 || activities != 1 || timers != 1 {
		t.Fatalf("supporter access-only membership=%d activity=%d timer=%d", memberships, activities, timers)
	}
}

func memberColumn(table string) string {
	if table == "social_goal_achievement_models" || table == "social_feed_event_models" {
		return "participant_user_id"
	}
	if table == "path_membership_models" {
		return "user_id"
	}
	return "participant_id"
}
