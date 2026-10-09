package gormstore

import (
	"bytes"
	"context"
	"errors"
	moderationapp "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresModeratedCommentIsUnavailableAcrossReadsAndReplay(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		name := "retained"
		if deleted {
			name = "deleted"
		}
		t.Run(name, func(t *testing.T) { testModeratedComment(t, deleted) })
	}
}
func testModeratedComment(t *testing.T, deleteBeforeAppeal bool) {
	f := newNudgeFixture(t, "removedcomment", false)
	ctx := context.Background()
	activityID := "moderated-activity-" + newTestID()
	if err := f.migration.DB.Table("recorded_activity_models").Create(map[string]any{"id": activityID, "path_id": f.pathID, "participant_id": f.sender.ID, "occurrence_time_zone": "Etc/UTC", "started_at": f.now.Add(-time.Hour), "ended_at": f.now.Add(-time.Minute), "created_at": f.now, "updated_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	eventID := "practice:" + activityID
	target, err := f.repository.ResolvePracticeCommentTarget(ctx, f.recipient.ID, eventID)
	if err != nil {
		t.Fatal(err)
	}
	command := socialCommentTestCommand(f.sender.ID, target, "Reported comment", "", 0, socialapp.CreatePracticeCommentOperation, "moderated-comment-create-01", f.now)
	result, err := f.repository.CreatePracticeComment(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.repository.CreatePracticeComment(ctx, socialCommentTestCommand(f.recipient.ID, target, "Unrelated comment", "", 0, socialapp.CreatePracticeCommentOperation, "moderated-comment-create-02", f.now))
	if err != nil {
		t.Fatal(err)
	}
	commentID := result.Comment.ID
	caseID, noticeID := newTestID(), newTestID()
	if err = f.migration.DB.Exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'comment',?,'spam_or_scam','','{}',?)`, caseID, f.recipient.ID, f.sender.ID, commentID, f.now).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.runtime.DB.Exec(`SELECT moderation_remove_comment(?,?,?)`, caseID, noticeID, "Policy reason").Error; err == nil {
		t.Fatal("runtime admitted removal")
	}
	// Admit an actual comment-heart push, then hold removal uncommitted while
	// handoff contends. No provider call may escape a committed removal.
	push, err := NewPushRepository(f.runtime.DB, bytes.Repeat([]byte{0x71}, 32))
	if err != nil {
		t.Fatal(err)
	}
	installationID := newTestID()
	if err = push.UpsertPushInstallation(ctx, ports.PushInstallation{ID: installationID, OwnerUserID: f.sender.ID, Provider: "expo", Platform: "ios", Locale: "en", Token: "ExponentPushToken[moderation-fixture]", CreatedAt: f.now, UpdatedAt: f.now}, audit.Event{ID: newTestID(), OwnerUserID: f.sender.ID, ActorUserID: f.sender.ID, Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: f.now}); err != nil {
		t.Fatal(err)
	}
	heart := socialCommentHeartTestCommand(f.recipient.ID, target, result.Comment, true, "moderated-comment-heart-01", f.now)
	if _, err = f.repository.SetPracticeCommentHeart(ctx, heart); err != nil {
		t.Fatal(err)
	}
	claims, err := push.ClaimPushDeliveries(ctx, "moderation-worker", time.Minute, 20)
	if err != nil || len(claims) != 1 {
		t.Fatalf("push claims: %+v %v", claims, err)
	}
	tx := f.migration.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	expectedText := "Reported comment"
	if !deleteBeforeAppeal {
		// A self-comment edit owns the comment lock before inserting its replay's
		// author FK. Removal must wait without taking a conflicting FK row lock.
		editTx := f.runtime.DB.Begin()
		if editTx.Error != nil {
			t.Fatal(editTx.Error)
		}
		defer editTx.Rollback()
		if err = lockSocialCommentTarget(editTx, commentID); err != nil {
			t.Fatal(err)
		}
		var editBackend int
		if err = editTx.Raw(`SELECT pg_backend_pid()`).Scan(&editBackend).Error; err != nil {
			t.Fatal(err)
		}
		removalDone := make(chan error, 1)
		go func() {
			removalDone <- tx.Exec(`SELECT moderation_remove_comment(?,?,?)`, caseID, noticeID, "Policy reason").Error
		}()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var waiting bool
			if err = editTx.Raw(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE ?=ANY(pg_blocking_pids(pid)))`, editBackend).Scan(&waiting).Error; err != nil {
				t.Fatal(err)
			}
			if waiting {
				break
			}
			select {
			case err := <-removalDone:
				t.Fatalf("removal escaped the editing lock: %v", err)
			default:
			}
			if time.Now().After(deadline) {
				t.Fatal("removal did not wait for edit")
			}
			time.Sleep(time.Millisecond)
		}
		expectedText = "Edited before removal"
		edit := socialCommentTestCommand(f.sender.ID, target, expectedText, commentID, 1, socialapp.EditPracticeCommentOperation, "moderation-competing-edit-01", f.now.Add(time.Second))
		if _, err = NewSocialFeedRepository(editTx).EditPracticeComment(ctx, edit); err != nil {
			t.Fatal(err)
		}
		if err = editTx.Commit().Error; err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-removalDone:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("removal did not resume")
		}
	} else {
		if err = tx.Exec(`SELECT moderation_remove_comment(?,?,?)`, caseID, noticeID, "Policy reason").Error; err != nil {
			t.Fatal(err)
		}
	}
	var backend int
	if err = tx.Raw(`SELECT pg_backend_pid()`).Scan(&backend).Error; err != nil {
		t.Fatal(err)
	}
	type handoffResult struct {
		called bool
		err    error
	}
	done := make(chan handoffResult, 1)
	go func() {
		called := false
		_, handedOff, err := push.HandoffPushDelivery(ctx, "moderation-worker", claims[0].NotificationID, installationID, func() ports.PushTicket { called = true; return ports.PushTicket{State: ports.PushDelivered} })
		done <- handoffResult{called || handedOff, err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case result := <-done:
			t.Fatalf("handoff escaped pending removal: %+v", result)
		default:
		}
		var waiting bool
		if err = tx.Raw(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE ?=ANY(pg_blocking_pids(pid)))`, backend).Scan(&waiting).Error; err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("handoff did not wait for removal")
		}
		time.Sleep(time.Millisecond)
	}
	if err = tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil || result.called {
			t.Fatalf("removed push escaped: %+v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handoff did not finish")
	}
	if _, _, err = f.repository.FindPracticeCommentHeartReplay(ctx, heart.Idempotency); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("removed heart replay: %v", err)
	}
	for _, viewer := range []string{f.sender.ID, f.recipient.ID} {
		page, err := f.repository.ListPracticeComments(ctx, viewer, eventID, socialapp.CommentPageRequest{Snapshot: f.now.Add(time.Second), Limit: 20})
		if err != nil || len(page.Items) != 1 || page.Items[0].Comment.ID != other.Comment.ID {
			t.Fatalf("visible comments: %+v %v", page, err)
		}
		if _, err = f.repository.ResolvePracticeComment(ctx, viewer, eventID, commentID); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("removed direct read: %v", err)
		}
		if _, err = f.repository.ListCommentHistory(ctx, viewer, eventID, commentID, socialapp.CommentHistoryPageRequest{Snapshot: f.now.Add(time.Second), Limit: 20}); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("removed history: %v", err)
		}
	}
	if _, _, err = f.repository.FindPracticeCommentReplay(ctx, command.Idempotency); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("removed replay reveals text: %v", err)
	}
	edit := socialCommentTestCommand(f.sender.ID, target, "Republished", commentID, 1, socialapp.EditPracticeCommentOperation, "moderated-comment-edit-01", f.now.Add(time.Second))
	if _, err = f.repository.EditPracticeComment(ctx, edit); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("removed edit: %v", err)
	}
	var count int64
	if err = f.migration.DB.Table("moderation_enforcement_models").Where("id=? AND action='content_removal' AND subject_user_id=?", noticeID, f.sender.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("missing author notice: %d %v", count, err)
	}
	if err = f.migration.DB.Table("moderation_review_event_models").Where("case_id=? AND operation='content_removal'", caseID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("missing decision evidence: %d %v", count, err)
	}
	// Counts and direct interaction entry points share the same restriction.
	feed, err := f.repository.GetPracticeCandidate(ctx, f.sender.ID, eventID, f.now.Add(time.Second))
	if err != nil || feed.CommentCount != 1 {
		t.Fatalf("feed count: %d %v", feed.CommentCount, err)
	}
	if _, err = f.repository.ListPracticeCommentHearts(ctx, f.recipient.ID, eventID, commentID, socialapp.CommentHeartRosterPageRequest{Snapshot: f.now.Add(time.Second), Limit: 20}); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("removed heart roster: %v", err)
	}
	if err = f.runtime.DB.Exec(`DELETE FROM moderation_removed_comment_models WHERE comment_id=?`, commentID).Error; err == nil {
		t.Fatal("runtime can undo removal")
	}
	// Only the subject receives the affected-comment reference and can appeal.
	at := time.Now().UTC()
	event := audit.Event{ID: newTestID(), OwnerUserID: f.sender.ID, ActorUserID: f.sender.ID, Action: audit.ResourceViewed, TargetType: "enforcement_notice", TargetID: noticeID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}
	moderation := NewModerationRepository(f.runtime.DB)
	notice, err := moderation.ReadNotice(ctx, f.sender.ID, noticeID, event)
	if err != nil || notice.Decision.AffectedCommentID != commentID || !notice.Decision.AffectedCommentCreatedAt.Equal(f.now) {
		t.Fatalf("affected comment notice: %+v %v", notice, err)
	}
	appeal, err := notice.Decision.NewAppeal("comment-removal-appeal-01", "Please review", at)
	if err != nil {
		t.Fatal(err)
	}
	event.ID = newTestID()
	event.Action = audit.ResourceCreated
	event.TargetType = "moderation_appeal"
	if _, err = moderation.SubmitAppeal(ctx, moderationapp.AppealCommand{OwnerID: f.sender.ID, Appeal: appeal, Audit: event}); err != nil {
		t.Fatal(err)
	}
	if err = f.runtime.DB.Exec(`SELECT moderation_decide_comment_appeal(?,?,?,?)`, noticeID, "reversed", "Context reviewed", "Only available reviewer").Error; err == nil {
		t.Fatal("runtime admitted appeal decision")
	}
	if err = f.migration.DB.Exec(`SELECT moderation_decide_comment_appeal(?,?,?,?)`, noticeID, "reversed", "Context reviewed", "").Error; err == nil {
		t.Fatal("same reviewer without reason")
	}
	if deleteBeforeAppeal {
		if err = f.migration.DB.Table("recorded_activity_models").Where("id=?", activityID).Delete(&struct{}{}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = f.migration.DB.Exec(`SELECT moderation_decide_comment_appeal(?,?,?,?)`, noticeID, "reversed", "Context reviewed", "Only available reviewer").Error; err != nil {
		t.Fatal(err)
	}
	restored, err := f.repository.ResolvePracticeComment(ctx, f.recipient.ID, eventID, commentID)
	if deleteBeforeAppeal {
		if !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("appeal recreated deleted content: %+v %v", restored, err)
		}
	} else if err != nil || restored.Comment.Text != expectedText {
		t.Fatalf("appeal reversal: %+v %v", restored, err)
	}
	var notificationCount int64
	if err = f.migration.DB.Table("notification_models").Where("comment_id=?", commentID).Count(&notificationCount).Error; err != nil || notificationCount != 0 {
		t.Fatalf("old notification recreated: %d %v", notificationCount, err)
	}
	if err = f.migration.DB.Exec(`SELECT moderation_decide_comment_appeal(?,?,?,?)`, noticeID, "upheld", "Changed mind", "Only available reviewer").Error; err == nil {
		t.Fatal("final decision replaced")
	}

}
