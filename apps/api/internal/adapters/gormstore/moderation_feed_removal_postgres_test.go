package gormstore

import (
	"bytes"
	"context"
	"errors"
	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	moderationapp "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	moderationdomain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresModeratedFeedPreservesSourceAndAppealLifecycle(t *testing.T) {
	for _, kind := range []string{"practice", "achievement"} {
		for _, deleted := range []bool{false, true} {
			name := kind + "/retained"
			if deleted {
				name = kind + "/deleted"
			}
			t.Run(name, func(t *testing.T) { testModeratedFeed(t, kind, deleted) })
		}
	}
}
func testModeratedFeed(t *testing.T, kind string, deleted bool) {
	f := newNudgeFixture(t, "removedfeed", false)
	ctx := context.Background()
	sourceID := "moderated-feed-source-" + newTestID()
	eventID := "practice:" + sourceID
	sourceTable := "recorded_activity_models"
	if kind == "practice" {
		if err := f.migration.DB.Table(sourceTable).Create(map[string]any{"id": sourceID, "path_id": f.pathID, "participant_id": f.sender.ID, "occurrence_time_zone": "Etc/UTC", "started_at": f.now.Add(-time.Hour), "ended_at": f.now.Add(-time.Minute), "created_at": f.now, "updated_at": f.now}).Error; err != nil {
			t.Fatal(err)
		}
	} else {
		sourceTable = "social_goal_achievement_models"
		eventID = "achievement:" + sourceID
		if err := f.migration.DB.Table(sourceTable).Create(map[string]any{"id": sourceID, "participant_user_id": f.sender.ID, "path_id": f.pathID, "kind": "overall", "target_seconds": int64(300), "published_at": f.now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.migration.DB.Table("social_feed_event_models").Create(map[string]any{"id": eventID, "achievement_id": sourceID, "participant_user_id": f.sender.ID, "path_id": f.pathID, "published_at": f.now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	target, err := f.repository.ResolvePracticeCommentTarget(ctx, f.recipient.ID, eventID)
	if err != nil {
		t.Fatal(err)
	}
	command := socialCommentTestCommand(f.sender.ID, target, "Retained comment", "", 0, socialapp.CreatePracticeCommentOperation, "moderated-feed-comment-01", f.now)
	comment, err := f.repository.CreatePracticeComment(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	reaction := socialReactionTestCommand(f.sender.ID, socialapp.ReactionTarget{EventID: eventID, PathID: f.pathID, OwnerUserID: f.sender.ID}, socialdomain.ReactionCelebrate, socialapp.SetPracticeReactionOperation, "moderated-feed-reaction-01", f.now)
	if _, err = f.repository.SetPracticeReaction(ctx, reaction); err != nil {
		t.Fatal(err)
	}
	feedRequest := socialapp.FeedPageRequest{Snapshot: f.now.Add(time.Minute), Limit: 20}
	assertFeed := func(want int) {
		t.Helper()
		page, e := f.repository.ListPracticeCandidates(ctx, f.recipient.ID, feedRequest)
		if e != nil || len(page.Items) != want {
			t.Fatalf("Following=%+v %v want %d", page, e, want)
		}
		page, e = f.repository.ListProfilePracticeCandidates(ctx, f.recipient.ID, f.sender.ID, feedRequest)
		if e != nil || len(page.Items) != want {
			t.Fatalf("profile=%+v %v want %d", page, e, want)
		}
	}
	assertFeed(1)
	push, err := NewPushRepository(f.runtime.DB, bytes.Repeat([]byte{0x73}, 32), pushTestClock{f.now})
	if err != nil {
		t.Fatal(err)
	}
	installationID := newTestID()
	if err = push.UpsertPushInstallation(ctx, ports.PushInstallation{ID: installationID, OwnerUserID: f.sender.ID, Provider: "expo", Platform: "ios", Locale: "en", Token: "ExponentPushToken[feed-removal-fixture]", CreatedAt: f.now, UpdatedAt: f.now}, audit.Event{ID: newTestID(), OwnerUserID: f.sender.ID, ActorUserID: f.sender.ID, Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: f.now}); err != nil {
		t.Fatal(err)
	}
	heart := socialCommentHeartTestCommand(f.recipient.ID, target, comment.Comment, true, "moderated-feed-heart-01", f.now)
	if _, err = f.repository.SetPracticeCommentHeart(ctx, heart); err != nil {
		t.Fatal(err)
	}
	claims, err := push.ClaimPushDeliveries(ctx, "feed-removal-worker", time.Minute, 20)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claims=%+v %v", claims, err)
	}
	notifications := pathstore.New(f.runtime.DB)
	notificationPage := pathapp.NotificationPageRequest{Snapshot: f.now.Add(time.Minute), Limit: 20}
	before, err := notifications.ListNotifications(ctx, f.sender.ID, notificationPage)
	if err != nil || len(before.Items) != 1 || before.UnreadCount != 1 {
		t.Fatalf("before=%+v %v", before, err)
	}
	caseID, noticeID := newTestID(), newTestID()
	if err = f.migration.DB.Exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'feed_event',?,'spam_or_scam','','{}',?)`, caseID, f.recipient.ID, f.sender.ID, eventID, f.now).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.runtime.DB.Exec(`SELECT moderation_remove_feed_event(?,?,?)`, caseID, noticeID, "Policy reason").Error; err == nil {
		t.Fatal("runtime admitted removal")
	}
	tx := f.migration.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err = tx.Exec(`SELECT moderation_remove_feed_event(?,?,?)`, caseID, noticeID, "Policy reason").Error; err != nil {
		t.Fatal(err)
	}
	var backend int
	if err = tx.Raw(`SELECT pg_backend_pid()`).Scan(&backend).Error; err != nil {
		t.Fatal(err)
	}
	type handoffResult struct {
		called bool
		err    error
	}
	handoff := make(chan handoffResult, 1)
	mutation := make(chan error, 1)
	go func() {
		called := false
		_, sent, e := push.HandoffPushDelivery(ctx, "feed-removal-worker", claims[0].NotificationID, installationID, func() ports.PushTicket { called = true; return ports.PushTicket{State: ports.PushDelivered} })
		handoff <- handoffResult{called || sent, e}
	}()
	competing := socialCommentTestCommand(f.sender.ID, target, "Must not republish", "", 0, socialapp.CreatePracticeCommentOperation, "moderated-feed-competing-01", f.now)
	go func() { _, e := f.repository.CreatePracticeComment(ctx, competing); mutation <- e }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case result := <-handoff:
			t.Fatalf("push escaped pending removal: %+v", result)
		default:
		}
		select {
		case e := <-mutation:
			t.Fatalf("comment escaped pending removal: %v", e)
		default:
		}
		var waiting int64
		if err = f.migration.DB.Raw(`SELECT count(*) FROM pg_stat_activity WHERE ?=ANY(pg_blocking_pids(pid))`, backend).Scan(&waiting).Error; err != nil {
			t.Fatal(err)
		}
		if waiting >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("push and comment did not serialize with removal")
		}
		time.Sleep(time.Millisecond)
	}
	if err = tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-handoff:
		if result.err != nil || result.called {
			t.Fatalf("removed push escaped: %+v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("push did not resume")
	}
	select {
	case e := <-mutation:
		if !errors.Is(e, ports.ErrNotFound) {
			t.Fatalf("removed post accepted comment: %v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("comment did not resume")
	}
	assertFeed(0)
	unavailable := func(e error) {
		t.Helper()
		if !errors.Is(e, ports.ErrNotFound) {
			t.Fatalf("expected opaque unavailable, got %v", e)
		}
	}
	_, err = f.repository.GetPracticeCandidate(ctx, f.recipient.ID, eventID, f.now)
	unavailable(err)
	_, err = f.repository.ListPracticeComments(ctx, f.recipient.ID, eventID, socialapp.CommentPageRequest{Snapshot: f.now.Add(time.Minute), Limit: 20})
	unavailable(err)
	_, err = f.repository.ResolvePracticeComment(ctx, f.recipient.ID, eventID, comment.Comment.ID)
	unavailable(err)
	_, err = f.repository.CreatePracticeComment(ctx, command)
	unavailable(err)
	_, _, err = f.repository.FindPracticeCommentHeartReplay(ctx, heart.Idempotency)
	unavailable(err)
	_, err = f.repository.SetPracticeCommentHeart(ctx, heart)
	unavailable(err)
	_, err = f.repository.SetPracticeReaction(ctx, reaction)
	unavailable(err)
	_, err = NewModerationRepository(f.runtime.DB).ResolveReportTarget(ctx, f.recipient.ID, moderationdomain.Target{Kind: moderationdomain.FeedEvent, ID: eventID}, f.now)
	unavailable(err)
	hidden, err := notifications.ListNotifications(ctx, f.sender.ID, notificationPage)
	if err != nil || len(hidden.Items) != 0 || hidden.UnreadCount != 0 {
		t.Fatalf("hidden=%+v %v", hidden, err)
	}
	_, err = notifications.GetNotification(ctx, f.sender.ID, claims[0].NotificationID)
	unavailable(err)
	var retained int64
	if err = f.runtime.DB.Table(sourceTable).Where("id=?", sourceID).Count(&retained).Error; err != nil || retained != 1 {
		t.Fatalf("source removed: %d %v", retained, err)
	}
	if kind == "practice" {
		if err = f.runtime.DB.Table(sourceTable).Where("id=? AND participant_id=? AND started_at=? AND ended_at=?", sourceID, f.sender.ID, f.now.Add(-time.Hour), f.now.Add(-time.Minute)).Count(&retained).Error; err != nil || retained != 1 {
			t.Fatalf("recorded time changed: %d %v", retained, err)
		}
		if err = f.migration.DB.Table(sourceTable).Where("id=?", sourceID).Update("started_at", f.now.Add(-2*time.Hour)).Error; err != nil {
			t.Fatal(err)
		}
		assertFeed(0)
	}
	at := time.Now().UTC()
	moderation := NewModerationRepository(f.runtime.DB)
	event := audit.Event{ID: newTestID(), OwnerUserID: f.sender.ID, ActorUserID: f.sender.ID, Action: audit.ResourceViewed, TargetType: "enforcement_notice", TargetID: noticeID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}
	notice, err := moderation.ReadNotice(ctx, f.sender.ID, noticeID, event)
	if err != nil || notice.Decision.AffectedFeedEventID != eventID || notice.Decision.AffectedFeedEventPublishedAt.IsZero() {
		t.Fatalf("notice=%+v %v", notice, err)
	}
	appeal, err := notice.Decision.NewAppeal("feed-removal-appeal-01", "Please review", at)
	if err != nil {
		t.Fatal(err)
	}
	event.ID = newTestID()
	event.Action = audit.ResourceCreated
	event.TargetType = "moderation_appeal"
	if _, err = moderation.SubmitAppeal(ctx, moderationapp.AppealCommand{OwnerID: f.sender.ID, Appeal: appeal, Audit: event}); err != nil {
		t.Fatal(err)
	}
	if err = f.runtime.DB.Exec(`SELECT moderation_decide_feed_event_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "Only reviewer").Error; err == nil {
		t.Fatal("runtime admitted appeal decision")
	}
	if err = f.migration.DB.Exec(`SELECT moderation_decide_comment_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "Only reviewer").Error; err == nil {
		t.Fatal("comment decision accepted feed appeal")
	}
	if err = f.migration.DB.Exec(`SELECT moderation_decide_nudge_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "Only reviewer").Error; err == nil {
		t.Fatal("nudge decision accepted feed appeal")
	}
	if err = f.migration.DB.Exec(`SELECT moderation_decide_feed_event_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "").Error; err == nil {
		t.Fatal("same reviewer omitted justification")
	}
	if deleted {
		if err = f.migration.DB.Table(sourceTable).Where("id=?", sourceID).Delete(&struct{ ID string }{}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = f.migration.DB.Exec(`SELECT moderation_decide_feed_event_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "Only reviewer").Error; err != nil {
		t.Fatal(err)
	}
	expected := 1
	if deleted {
		expected = 0
	}
	assertFeed(expected)
	restored, err := notifications.ListNotifications(ctx, f.sender.ID, notificationPage)
	if err != nil || len(restored.Items) != expected {
		t.Fatalf("restored=%+v %v", restored, err)
	}
	claims, err = push.ClaimPushDeliveries(ctx, "feed-removal-worker", time.Minute, 20)
	if err != nil || len(claims) != 0 {
		t.Fatalf("appeal resent push: %+v %v", claims, err)
	}
	if !deleted {
		if _, err = f.repository.CreatePracticeComment(ctx, command); err != nil {
			t.Fatalf("restored replay=%v", err)
		}
	}
	if err = f.migration.DB.Exec(`SELECT moderation_decide_feed_event_appeal(?,?,?,?)`, noticeID, "upheld", "Changed mind", "Only reviewer").Error; err == nil {
		t.Fatal("replaced final decision")
	}
}

// Disabled interactions retain a safe notification tombstone until the parent
// event is removed. Moderation must also hide this alternate resolution branch.
func TestPostgresModeratedFeedHidesDisabledNotification(t *testing.T) {
	f := newNudgeFixture(t, "removeddisabledfeed", false)
	ctx := context.Background()
	sourceID := "moderated-feed-source-" + newTestID()
	eventID := "practice:" + sourceID
	if err := f.migration.DB.Table("recorded_activity_models").Create(map[string]any{"id": sourceID, "path_id": f.pathID, "participant_id": f.sender.ID, "occurrence_time_zone": "Etc/UTC", "started_at": f.now.Add(-time.Hour), "ended_at": f.now.Add(-time.Minute), "created_at": f.now, "updated_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	target, err := f.repository.ResolvePracticeCommentTarget(ctx, f.recipient.ID, eventID)
	if err != nil {
		t.Fatal(err)
	}
	command := socialCommentTestCommand(f.recipient.ID, target, "Retained comment", "", 0, socialapp.CreatePracticeCommentOperation, "disabled-feed-comment-01", f.now)
	if _, err = f.repository.CreatePracticeComment(ctx, command); err != nil {
		t.Fatal(err)
	}
	notifications := pathstore.New(f.runtime.DB)
	before, err := notifications.ListNotifications(ctx, f.sender.ID, pathapp.NotificationPageRequest{Snapshot: f.now.Add(time.Minute), Limit: 20})
	if err != nil || len(before.Items) != 1 {
		t.Fatalf("before=%+v %v", before, err)
	}
	noticeID := before.Items[0].ID
	if _, err = f.repository.UpdateInteractionSettings(ctx, interactionSettingsTestCommand(f.sender.ID, socialapp.InteractionSettings{CommentsEnabled: false, ReactionsEnabled: false}, "disable-feed-settings-01", f.now.Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	disabled, err := notifications.GetNotification(ctx, f.sender.ID, noticeID)
	if err != nil || string(disabled.InteractionDisabled) != "comments" {
		t.Fatalf("disabled=%+v %v", disabled, err)
	}
	caseID := newTestID()
	if err = f.migration.DB.Exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'feed_event',?,'spam_or_scam','','{}',?)`, caseID, f.recipient.ID, f.sender.ID, eventID, f.now).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.migration.DB.Exec(`SELECT moderation_remove_feed_event(?,?,?)`, caseID, newTestID(), "Policy reason").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = notifications.GetNotification(ctx, f.sender.ID, noticeID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("removed notification exposed: %v", err)
	}
}
