package pathstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	sociallock "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPostgresRetainedActivityLeaveRetiresEveryHiddenFeedNotificationAtomically(t *testing.T) {
	fixture := newHiddenFeedLeaveFixture(t, "retained")
	command := hiddenFeedLeaveCommand(fixture.participant, fixture.pathID, fixture.suffix, fixture.now, true)
	rollback := hiddenFeedLeaveCommand(fixture.participant, fixture.pathID, fixture.suffix+"-rollback", fixture.now, true)
	if err := fixture.db.Create(fromAudit(rollback.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.LeavePath(context.Background(), rollback); err == nil {
		t.Fatal("LeavePath() with duplicate audit unexpectedly succeeded")
	}
	fixture.assertTargetState(t, true, true, 0)

	blocker := fixture.db.Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	t.Cleanup(func() { _ = blocker.Rollback().Error })
	if err := progresslock.LockKey(blocker, sociallock.InteractionOwnerKey(fixture.participant)); err != nil {
		t.Fatal(err)
	}
	type leaveResult struct {
		result application.LeavePathResult
		err    error
	}
	completed := make(chan leaveResult, 1)
	go func() {
		result, err := fixture.repository.LeavePath(context.Background(), command)
		completed <- leaveResult{result: result, err: err}
	}()
	waitForHiddenFeedLeaveLock(t, fixture.db, fixture.participant)
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	finished := <-completed
	result, err := finished.result, finished.err
	if err != nil || !result.Left || !result.ActivityRetained || result.Replayed {
		t.Fatalf("LeavePath()=%+v err=%v", result, err)
	}
	fixture.assertTargetState(t, false, true, 2)
	for _, target := range fixture.targets {
		if _, err := fixture.repository.GetNotification(context.Background(), target.recipient, target.id); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("GetNotification(%s) error=%v", target.kind, err)
		}
	}
	participantPage, err := fixture.repository.ListNotifications(context.Background(), fixture.participant, application.NotificationPageRequest{Limit: 25, Snapshot: fixture.now.Add(time.Minute)})
	if err != nil || participantPage.UnreadCount != 2 || len(participantPage.Items) != 2 {
		t.Fatalf("participant notifications=%+v err=%v", participantPage, err)
	}
	commenterPage, err := fixture.repository.ListNotifications(context.Background(), fixture.commenter, application.NotificationPageRequest{Limit: 25, Snapshot: fixture.now.Add(time.Minute)})
	if err != nil || commenterPage.UnreadCount != 0 || len(commenterPage.Items) != 0 {
		t.Fatalf("commenter notifications=%+v err=%v", commenterPage, err)
	}

	replay, err := fixture.repository.LeavePath(context.Background(), command)
	if err != nil || !replay.Replayed || !replay.ActivityRetained {
		t.Fatalf("LeavePath(replay)=%+v err=%v", replay, err)
	}
	fixture.assertTargetState(t, false, true, 2)
	if err := fixture.db.Create(&membershipModel{PathID: fixture.pathID, UserID: fixture.participant, Role: string(domain.RoleParticipant), JoinedAt: fixture.now.Add(time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	for _, target := range fixture.targets {
		if _, err := fixture.repository.GetNotification(context.Background(), target.recipient, target.id); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("rejoin resurrected %s notification: %v", target.kind, err)
		}
	}
}

func TestPostgresDeleteActivityLeaveKeepsCascadeAndUnrelatedNotificationControls(t *testing.T) {
	fixture := newHiddenFeedLeaveFixture(t, "deleted")
	command := hiddenFeedLeaveCommand(fixture.participant, fixture.pathID, fixture.suffix, fixture.now, false)
	result, err := fixture.repository.LeavePath(context.Background(), command)
	if err != nil || !result.Left || result.ActivityRetained {
		t.Fatalf("LeavePath(delete)=%+v err=%v", result, err)
	}
	fixture.assertTargetState(t, false, false, 2)
	var targetRows int64
	ids := make([]string, 0, len(fixture.targets))
	for _, target := range fixture.targets {
		ids = append(ids, target.id)
	}
	if err := fixture.db.Table("notification_models").Where("id IN ?", ids).Count(&targetRows).Error; err != nil || targetRows != 0 {
		t.Fatalf("delete control target rows=%d err=%v", targetRows, err)
	}
}

type hiddenFeedTarget struct{ id, recipient, kind string }

type hiddenFeedLeaveFixture struct {
	repository                                  *Repository
	db                                          *gorm.DB
	suffix, owner, administrator, participant   string
	commenter, reactor, pathID, otherPathID     string
	activityID, eventID, commentID, otherNotice string
	targets                                     []hiddenFeedTarget
	now                                         time.Time
}

func newHiddenFeedLeaveFixture(t *testing.T, label string) hiddenFeedLeaveFixture {
	t.Helper()
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%s-%d", label, time.Now().UnixNano())
	fixture := hiddenFeedLeaveFixture{
		repository: New(runtimeDB), db: migrationDB, suffix: suffix,
		owner: "hidden-owner-" + suffix, administrator: "hidden-admin-" + suffix,
		participant: "hidden-participant-" + suffix, commenter: "hidden-commenter-" + suffix,
		reactor: "hidden-reactor-" + suffix, pathID: "hidden-path-" + suffix,
		otherPathID: "hidden-other-" + suffix, activityID: "hidden-activity-" + suffix,
		commentID: "hidden-comment-" + suffix, otherNotice: "hidden-unrelated-" + suffix,
		now: time.Now().UTC().Truncate(time.Microsecond),
	}
	fixture.eventID = "practice:" + fixture.activityID
	fixture.targets = []hiddenFeedTarget{
		{"hidden-reaction-notice-" + suffix, fixture.participant, "practice_reaction"},
		{"hidden-comment-notice-" + suffix, fixture.participant, "practice_comment"},
		{"hidden-heart-notice-" + suffix, fixture.commenter, "comment_heart"},
	}
	users := []string{fixture.owner, fixture.administrator, fixture.participant, fixture.commenter, fixture.reactor}
	paths := []string{fixture.pathID, fixture.otherPathID}
	seedGoalUpdateUsers(t, migrationDB, fixture.now, users...)
	for index, userID := range users {
		username := fmt.Sprintf("hf%d%d", index, time.Now().UnixNano())
		if err := migrationDB.Table("user_models").Where("id = ?", userID).Updates(map[string]any{"username": username, "display_name": username}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []domain.Entity{
		{ID: domain.ID(fixture.pathID), OwnerUserID: fixture.owner, Attributes: domain.Attributes{Name: "Hidden activity", Visibility: "private"}, CreatedAt: fixture.now.Add(-time.Hour), UpdatedAt: fixture.now.Add(-time.Hour)},
		{ID: domain.ID(fixture.otherPathID), OwnerUserID: fixture.owner, Attributes: domain.Attributes{Name: "Unrelated", Visibility: "private"}, CreatedAt: fixture.now.Add(-time.Hour), UpdatedAt: fixture.now.Add(-time.Hour)},
	} {
		seedGoalUpdatePath(t, migrationDB, path, fixture.owner)
	}
	if err := migrationDB.Create(&[]membershipModel{
		{PathID: fixture.pathID, UserID: fixture.administrator, Role: string(domain.RoleAdministrator), JoinedAt: fixture.now.Add(-time.Hour)},
		{PathID: fixture.pathID, UserID: fixture.participant, Role: string(domain.RoleParticipant), JoinedAt: fixture.now.Add(-time.Hour)},
		{PathID: fixture.pathID, UserID: fixture.commenter, Role: string(domain.RoleParticipant), JoinedAt: fixture.now.Add(-time.Hour)},
		{PathID: fixture.pathID, UserID: fixture.reactor, Role: string(domain.RoleParticipant), JoinedAt: fixture.now.Add(-time.Hour)},
		{PathID: fixture.otherPathID, UserID: fixture.participant, Role: string(domain.RoleParticipant), JoinedAt: fixture.now.Add(-time.Hour)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupDeletionFixture(migrationDB, users, paths) })
	if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{"id": fixture.activityID, "path_id": fixture.pathID, "participant_id": fixture.participant, "started_at": fixture.now.Add(-time.Minute), "ended_at": fixture.now, "occurrence_time_zone": "Etc/UTC", "created_at": fixture.now.Add(-time.Minute), "updated_at": fixture.now.Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("social_practice_reaction_models").Create(map[string]any{"social_feed_event_id": fixture.eventID, "actor_user_id": fixture.reactor, "reaction_type": "heart", "created_at": fixture.now.Add(-30 * time.Second), "updated_at": fixture.now.Add(-30 * time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("social_practice_comment_models").Create(map[string]any{"id": fixture.commentID, "social_feed_event_id": fixture.eventID, "author_user_id": fixture.commenter, "body": "Keep going", "version": 1, "created_at": fixture.now.Add(-20 * time.Second), "updated_at": fixture.now.Add(-20 * time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("social_practice_comment_heart_models").Create(map[string]any{"comment_id": fixture.commentID, "actor_user_id": fixture.reactor, "created_at": fixture.now.Add(-10 * time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	createdAt := fixture.now.Add(-5 * time.Second)
	futureCreatedAt := fixture.now.Add(5 * time.Second)
	disabledAt, disabledReason := fixture.now.Add(-2*time.Second), "comments"
	for _, row := range []map[string]any{
		{"id": fixture.targets[0].id, "recipient_user_id": fixture.participant, "actor_user_id": fixture.reactor, "path_id": fixture.pathID, "social_feed_event_id": fixture.eventID, "reaction_type": "heart", "kind": "practice_reaction", "presentation_class": "informational", "channel": "reactions", "created_at": createdAt},
		{"id": fixture.targets[1].id, "recipient_user_id": fixture.participant, "actor_user_id": fixture.commenter, "path_id": fixture.pathID, "social_feed_event_id": fixture.eventID, "comment_id": fixture.commentID, "kind": "practice_comment", "presentation_class": "informational", "channel": "comments", "created_at": futureCreatedAt},
		{"id": fixture.targets[2].id, "recipient_user_id": fixture.commenter, "actor_user_id": fixture.reactor, "path_id": fixture.pathID, "social_feed_event_id": fixture.eventID, "comment_id": fixture.commentID, "kind": "comment_heart", "presentation_class": "informational", "channel": "comment_hearts", "created_at": createdAt, "deleted_at": disabledAt, "interaction_disabled_reason": disabledReason},
		{"id": "hidden-explanation-" + suffix, "recipient_user_id": fixture.participant, "actor_user_id": fixture.owner, "path_id": fixture.pathID, "path_visibility": "public", "kind": "path_visibility_changed", "presentation_class": "informational", "channel": "path_access", "created_at": createdAt},
	} {
		if err := migrationDB.Table("notification_models").Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, recipient := range []string{fixture.participant, fixture.commenter} {
		hash := sha256.Sum256([]byte("hidden-token-" + recipient))
		if err := migrationDB.Table("push_installation_models").Create(map[string]any{"id": "hidden-installation-" + recipient, "owner_user_id": recipient, "provider": "expo", "platform": "ios", "locale": "en", "token_ciphertext": bytes.Repeat([]byte{7}, 17), "token_nonce": bytes.Repeat([]byte{8}, 12), "token_hash": hash[:], "created_at": createdAt, "updated_at": createdAt}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for index, target := range fixture.targets[:2] {
		pushCreatedAt := createdAt
		if index == 1 {
			pushCreatedAt = futureCreatedAt
		}
		if err := createNotificationPushDelivery(migrationDB, target.id, target.recipient, pushCreatedAt); err != nil {
			t.Fatal(err)
		}
	}
	handedOffAt := fixture.now.Add(-time.Second)
	if err := migrationDB.Table("notification_push_delivery_models").Where("notification_id = ?", fixture.targets[0].id).Updates(map[string]any{"delivered_at": handedOffAt, "token_ciphertext": nil, "token_nonce": nil, "token_hash": nil}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_outbox_models").Where("notification_id = ?", fixture.targets[0].id).Update("delivered_at", handedOffAt).Error; err != nil {
		t.Fatal(err)
	}
	otherNudge := "hidden-other-nudge-" + suffix
	if err := migrationDB.Table("social_nudge_models").Create(map[string]any{"id": otherNudge, "sender_user_id": fixture.owner, "recipient_user_id": fixture.participant, "path_id": fixture.otherPathID, "content_kind": "preset", "preset": "keep_it_going", "sent_at": createdAt}).Error; err != nil {
		t.Fatal(err)
	}
	seedNudgeNotification(t, migrationDB, notificationModel{ID: fixture.otherNotice, RecipientUserID: fixture.participant, ActorUserID: fixture.owner, PathID: fixture.otherPathID, NudgeID: otherNudge, Kind: nudgeReceivedNotificationKind, PresentationClass: "informational", Channel: "nudges", CreatedAt: createdAt})
	return fixture
}

func waitForHiddenFeedLeaveLock(t *testing.T, db *gorm.DB, owner string) {
	t.Helper()
	key := sociallock.InteractionOwnerKey(owner)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiters int64
		err := db.Table("pg_locks").Where(`locktype = 'advisory' AND NOT granted
AND classid = ((hashtextextended(?, 0) >> 32) & 4294967295)::oid
AND objid = (hashtextextended(?, 0) & 4294967295)::oid
AND objsubid = 1`, key, key).Count(&waiters).Error
		if err != nil {
			t.Fatal(err)
		}
		if waiters == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("retain-activity leave did not acquire the shared social interaction owner lock")
}

func hiddenFeedLeaveCommand(participant, pathID, suffix string, now time.Time, retain bool) application.LeavePathCommand {
	sequence := 0
	return application.LeavePathCommand{
		ActorUserID: participant, PathID: domain.ID(pathID), LeftAt: now, RetainActivity: retain,
		Idempotency: ports.Idempotency{PrincipalID: participant, Operation: application.LeavePathOperation, Key: "hidden-leave-key-" + suffix, RequestHash: bytes.Repeat([]byte{3}, 32)},
		Audit:       audit.Event{ID: "hidden-leave-audit-" + suffix, OwnerUserID: participant, ActorUserID: participant, Action: audit.ResourceUpdated, TargetType: "path", TargetID: pathID, Outcome: audit.Succeeded, CorrelationID: "hidden-leave-" + suffix, OccurredAt: now},
		NewID:       func() string { sequence++; return fmt.Sprintf("hidden-leave-id-%s-%d", suffix, sequence) }, AuthorizationWorker: "hidden-leave-test", AuthorizationLease: time.Minute,
	}
}

func (fixture hiddenFeedLeaveFixture) assertTargetState(t *testing.T, active, activityRetained bool, managerNotices int64) {
	t.Helper()
	ids := make([]string, 0, len(fixture.targets))
	for _, target := range fixture.targets {
		ids = append(ids, target.id)
	}
	var targetRows, lifecycleTargets, disabledTargets, futureTombstone, activity, event, comment, reaction, heart, unrelated, explanation, notices int64
	checks := []struct {
		to           *int64
		table, where string
		args         []any
	}{
		{&targetRows, "notification_models", "id IN ?", []any{ids}},
		{&lifecycleTargets, "notification_models", "id IN ? AND (deleted_at IS NULL OR interaction_disabled_reason IS NOT NULL)", []any{ids}},
		{&disabledTargets, "notification_models", "id IN ? AND interaction_disabled_reason IS NOT NULL", []any{ids}},
		{&futureTombstone, "notification_models", "id = ? AND deleted_at = ? AND interaction_disabled_reason IS NULL", []any{fixture.targets[1].id, fixture.now.Add(5 * time.Second)}},
		{&activity, "recorded_activity_models", "id = ?", []any{fixture.activityID}},
		{&event, "social_feed_event_models", "id = ?", []any{fixture.eventID}},
		{&comment, "social_practice_comment_models", "id = ?", []any{fixture.commentID}},
		{&reaction, "social_practice_reaction_models", "social_feed_event_id = ?", []any{fixture.eventID}},
		{&heart, "social_practice_comment_heart_models", "comment_id = ?", []any{fixture.commentID}},
		{&unrelated, "notification_models", "id = ? AND deleted_at IS NULL", []any{fixture.otherNotice}},
		{&explanation, "notification_models", "id = ? AND deleted_at IS NULL", []any{"hidden-explanation-" + fixture.suffix}},
		{&notices, "notification_models", "path_id = ? AND kind = ? AND recipient_user_id IN ?", []any{fixture.pathID, application.NotificationPathMemberLeft, []string{fixture.owner, fixture.administrator}}},
	}
	for _, check := range checks {
		if err := fixture.db.Table(check.table).Where(check.where, check.args...).Count(check.to).Error; err != nil {
			t.Fatal(err)
		}
	}
	wantTargets, wantContent := int64(0), int64(0)
	wantRows, wantFutureTombstone := int64(0), int64(0)
	if active {
		wantTargets = 3
	}
	if activityRetained {
		wantContent = 1
		wantRows = 3
		if !active {
			wantFutureTombstone = 1
		}
	}
	wantDisabled := int64(0)
	if active {
		wantDisabled = 1
	}
	if targetRows != wantRows || lifecycleTargets != wantTargets || disabledTargets != wantDisabled || futureTombstone != wantFutureTombstone || activity != wantContent || event != wantContent || comment != wantContent || reaction != wantContent || heart != wantContent || unrelated != 1 || explanation != 1 || notices != managerNotices {
		t.Fatalf("state rows=%d targets=%d disabled=%d future=%d activity=%d event=%d comment=%d reaction=%d heart=%d unrelated=%d explanation=%d notices=%d", targetRows, lifecycleTargets, disabledTargets, futureTombstone, activity, event, comment, reaction, heart, unrelated, explanation, notices)
	}
	if !active && activityRetained {
		var pendingDeliveriesSuppressed, pendingOutboxesSuppressed, deliveredRetained, deliveredOutboxRetained int64
		pendingIDs := []string{fixture.targets[1].id}
		if err := fixture.db.Table("notification_push_delivery_models").Where("notification_id IN ? AND suppressed_at >= ? AND failure_code = ? AND token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL", pendingIDs, fixture.now, pathAccessRevokedPushFailureCode).Count(&pendingDeliveriesSuppressed).Error; err != nil || pendingDeliveriesSuppressed != 1 {
			t.Fatalf("pending deliveries suppressed=%d err=%v", pendingDeliveriesSuppressed, err)
		}
		if err := fixture.db.Table("notification_push_outbox_models").Where("notification_id IN ? AND suppressed_at >= ? AND failure_code = ?", pendingIDs, fixture.now, pathAccessRevokedPushFailureCode).Count(&pendingOutboxesSuppressed).Error; err != nil || pendingOutboxesSuppressed != 1 {
			t.Fatalf("pending outboxes suppressed=%d err=%v", pendingOutboxesSuppressed, err)
		}
		if err := fixture.db.Table("notification_push_delivery_models").Where("notification_id = ? AND delivered_at IS NOT NULL AND suppressed_at IS NULL", fixture.targets[0].id).Count(&deliveredRetained).Error; err != nil || deliveredRetained != 1 {
			t.Fatalf("handed-off delivery retained=%d err=%v", deliveredRetained, err)
		}
		if err := fixture.db.Table("notification_push_outbox_models").Where("notification_id = ? AND delivered_at IS NOT NULL AND suppressed_at IS NULL", fixture.targets[0].id).Count(&deliveredOutboxRetained).Error; err != nil || deliveredOutboxRetained != 1 {
			t.Fatalf("handed-off outbox retained=%d err=%v", deliveredOutboxRetained, err)
		}
	}
}
