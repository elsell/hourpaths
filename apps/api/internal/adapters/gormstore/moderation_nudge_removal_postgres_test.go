package gormstore

import (
	"bytes"
	"context"
	"errors"
	moderationapp "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"testing"
	"time"

	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	moderationdomain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresModeratedNudgeHidesNotificationAndReplay(t *testing.T) {
	t.Run("retained", func(t *testing.T) { testModeratedNudge(t, false) })
	t.Run("deleted", func(t *testing.T) { testModeratedNudge(t, true) })
}
func testModeratedNudge(t *testing.T, deleted bool) {
	f := newNudgeFixture(t, "removednudge", false)
	ctx := context.Background()
	push, err := NewPushRepository(f.runtime.DB, bytes.Repeat([]byte{0x72}, 32), pushTestClock{f.now})
	if err != nil {
		t.Fatal(err)
	}
	installationID := newTestID()
	if err := push.UpsertPushInstallation(ctx, ports.PushInstallation{ID: installationID, OwnerUserID: f.recipient.ID, Provider: "expo", Platform: "ios", Locale: "en", Token: "ExponentPushToken[nudge-removal-fixture]", CreatedAt: f.now, UpdatedAt: f.now}, audit.Event{ID: newTestID(), OwnerUserID: f.recipient.ID, ActorUserID: f.recipient.ID, Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: f.now}); err != nil {
		t.Fatal(err)
	}

	command := nudgeSendTestCommand("nudge-"+newTestID(), f.sender.ID, f.recipient.ID, f.pathID, "moderated-nudge-send-01", socialdomain.NudgeLetsGo, f.now)
	if _, err := f.repository.SendNudge(ctx, command); err != nil {
		t.Fatal(err)
	}
	notificationID := "nudge:" + command.Nudge.ID
	notifications := pathstore.New(f.runtime.DB)
	page := pathapp.NotificationPageRequest{Snapshot: f.now.Add(time.Minute), Limit: 20}
	before, err := notifications.ListNotifications(ctx, f.recipient.ID, page)
	if err != nil || len(before.Items) != 1 || before.UnreadCount != 1 {
		t.Fatalf("before=%+v %v", before, err)
	}
	caseID, noticeID := newTestID(), newTestID()
	if err := f.migration.DB.Exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'nudge',?,'spam_or_scam','','{}',?)`, caseID, f.recipient.ID, f.sender.ID, notificationID, f.now).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.DB.Exec(`SELECT moderation_remove_nudge(?,?,?)`, caseID, noticeID, "Policy reason").Error; err == nil {
		t.Fatal("runtime admitted enforcement")
	}

	claims, err := push.ClaimPushDeliveries(ctx, "nudge-removal-worker", time.Minute, 20)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claims=%+v %v", claims, err)
	}
	tx := f.migration.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Exec(`SELECT moderation_remove_nudge(?,?,?)`, caseID, noticeID, "Policy reason").Error; err != nil {
		t.Fatal(err)
	}
	var backend int
	if err := tx.Raw(`SELECT pg_backend_pid()`).Scan(&backend).Error; err != nil {
		t.Fatal(err)
	}
	type handoffResult struct {
		called bool
		err    error
	}
	done := make(chan handoffResult, 1)
	go func() {
		called := false
		_, handed, err := push.HandoffPushDelivery(ctx, "nudge-removal-worker", notificationID, installationID, func() ports.PushTicket { called = true; return ports.PushTicket{State: ports.PushDelivered} })
		done <- handoffResult{called || handed, err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case result := <-done:
			t.Fatalf("handoff escaped pending removal: %+v", result)
		default:
		}
		var waiting bool
		if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE ?=ANY(pg_blocking_pids(pid)))`, backend).Scan(&waiting).Error; err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("handoff did not serialize with removal")
		}
		time.Sleep(time.Millisecond)
	}
	if err := tx.Commit().Error; err != nil {
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

	hidden, err := notifications.ListNotifications(ctx, f.recipient.ID, page)
	if err != nil || len(hidden.Items) != 0 || hidden.UnreadCount != 0 {
		t.Fatalf("hidden=%+v %v", hidden, err)
	}
	if _, err := notifications.GetNotification(ctx, f.recipient.ID, notificationID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("removed notification=%v", err)
	}
	if _, err := NewModerationRepository(f.runtime.DB).ResolveReportTarget(ctx, f.recipient.ID, moderationdomain.Target{Kind: moderationdomain.Nudge, ID: notificationID}, f.now); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("removed report target=%v", err)
	}
	if _, err := f.repository.SendNudge(ctx, command); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("removed replay=%v", err)
	}
	eligibility, err := f.repository.GetNudgeEligibility(ctx, f.sender.ID, f.recipient.ID, f.pathID, f.now.Add(time.Minute))
	if err != nil || eligibility.Eligible {
		t.Fatalf("removal reset rate limit: %+v %v", eligibility, err)
	}
	if err := f.migration.DB.Exec(`SELECT moderation_remove_nudge(?,?,?)`, caseID, newTestID(), "Another notice").Error; err == nil {
		t.Fatal("duplicate removal admitted")
	}
	at := time.Now().UTC()
	event := audit.Event{ID: newTestID(), OwnerUserID: f.sender.ID, ActorUserID: f.sender.ID, Action: audit.ResourceViewed, TargetType: "enforcement_notice", TargetID: noticeID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}
	moderation := NewModerationRepository(f.runtime.DB)
	notice, err := moderation.ReadNotice(ctx, f.sender.ID, noticeID, event)
	if err != nil || notice.Decision.AffectedNudgeID != command.Nudge.ID || !notice.Decision.AffectedNudgeSentAt.Equal(f.now) {
		t.Fatalf("affected nudge: %+v %v", notice, err)
	}
	appeal, err := notice.Decision.NewAppeal("nudge-removal-appeal-01", "Please review", at)
	if err != nil {
		t.Fatal(err)
	}
	event.ID = newTestID()
	event.Action = audit.ResourceCreated
	event.TargetType = "moderation_appeal"
	if _, err := moderation.SubmitAppeal(ctx, moderationapp.AppealCommand{OwnerID: f.sender.ID, Appeal: appeal, Audit: event}); err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.DB.Exec(`SELECT moderation_decide_nudge_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "Only reviewer").Error; err == nil {
		t.Fatal("runtime admitted appeal decision")
	}
	if err := f.migration.DB.Exec(`SELECT moderation_decide_comment_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "Only reviewer").Error; err == nil {
		t.Fatal("wrong content type admitted appeal")
	}
	if err := f.migration.DB.Exec(`SELECT moderation_decide_nudge_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "").Error; err == nil {
		t.Fatal("same reviewer without justification")
	}
	if deleted {
		if err := f.migration.DB.Table("social_nudge_models").Where("id=?", command.Nudge.ID).Delete(&socialNudgeModel{}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := f.migration.DB.Exec(`SELECT moderation_decide_nudge_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "Only reviewer").Error; err != nil {
		t.Fatal(err)
	}
	restored, err := notifications.ListNotifications(ctx, f.recipient.ID, page)
	expected := 1
	if deleted {
		expected = 0
	}
	if err != nil || len(restored.Items) != expected || restored.UnreadCount != int64(expected) {
		t.Fatalf("restored=%+v %v", restored, err)
	}
	if !deleted {
		if _, err := f.repository.SendNudge(ctx, command); err != nil {
			t.Fatalf("restored replay=%v", err)
		}
	}

	claims, err = push.ClaimPushDeliveries(ctx, "nudge-removal-worker", time.Minute, 20)
	if err != nil || len(claims) != 0 {
		t.Fatalf("appeal resent old push: %+v %v", claims, err)
	}
	if err := f.migration.DB.Exec(`SELECT moderation_decide_nudge_appeal(?,?,?,?)`, noticeID, "upheld", "Changed mind", "Only reviewer").Error; err == nil {
		t.Fatal("replaced final decision")
	}

}
