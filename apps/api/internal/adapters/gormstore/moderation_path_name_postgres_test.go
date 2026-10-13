package gormstore

import (
	"bytes"
	"context"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"

	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	moderationapp "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestPostgresModeratedPathNamePreservesTrackingAndLaterChanges(t *testing.T) {
	for _, after := range []string{"unchanged", "goals", "renamed", "renamed_back", "transferred", "transferred_back", "deleted", "upheld"} {
		t.Run(after, func(t *testing.T) {
			f := newNudgeFixture(t, "pathremoval", true)
			ctx := context.Background()
			caseID, noticeID, timerID, activityID := newTestID(), newTestID(), newTestID(), newTestID()
			exec := func(query string, args ...any) {
				t.Helper()
				if err := f.migration.DB.Exec(query, args...).Error; err != nil {
					t.Fatal(err)
				}
			}
			exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'path',?,'spam_or_scam','','{"title":"Piano"}',?)`, caseID, f.recipient.ID, f.sender.ID, f.pathID, f.now)
			exec(`INSERT INTO running_timer_models(id,path_id,participant_id,started_at,occurrence_time_zone) VALUES(?,?,?,?,'Etc/UTC')`, timerID, f.pathID, f.recipient.ID, f.now)
			exec(`INSERT INTO recorded_activity_models(id,path_id,participant_id,started_at,ended_at,occurrence_time_zone,created_at,updated_at) VALUES(?,?,?,?,?,'Etc/UTC',?,?)`, activityID, f.pathID, f.recipient.ID, f.now.Add(-time.Hour), f.now.Add(-time.Minute), f.now, f.now)
			if err := f.runtime.DB.Exec(`SELECT moderation_remove_path_name(?,?,?,?)`, caseID, noticeID, "Policy reason", "Name removed").Error; err == nil {
				t.Fatal("runtime admitted removal")
			}
			exec(`SELECT moderation_remove_path_name(?,?,?,?)`, caseID, noticeID, "Policy reason", "Name removed")
			if err := f.migration.DB.Exec(`SELECT moderation_remove_path_name(?,?,?,?)`, caseID, newTestID(), "Policy reason", "Name removed").Error; err == nil {
				t.Fatal("duplicate removal admitted")
			}
			repo := pathstore.New(f.runtime.DB)
			p, err := repo.Get(ctx, f.sender.ID, pathdomain.ID(f.pathID))
			if err != nil || p.Name != "Name removed" {
				t.Fatalf("path: %+v %v", p, err)
			}
			for table, id := range map[string]string{"running_timer_models": timerID, "recorded_activity_models": activityID} {
				var count int64
				err = f.runtime.DB.Table(table).Where("id=? AND path_id=? AND participant_id=?", id, f.pathID, f.recipient.ID).Count(&count).Error
				if err != nil || count != 1 {
					t.Fatalf("lost %s: %d %v", table, count, err)
				}
			}
			var memberships int64
			if err = f.runtime.DB.Table("path_membership_models").Where("path_id=? AND user_id=?", f.pathID, f.recipient.ID).Count(&memberships).Error; err != nil || memberships != 1 {
				t.Fatalf("membership changed: %d %v", memberships, err)
			}
			at := time.Now().UTC()
			moderation := NewModerationRepository(f.runtime.DB)
			event := audit.Event{ID: newTestID(), OwnerUserID: f.sender.ID, ActorUserID: f.sender.ID, Action: audit.ResourceViewed, TargetType: "enforcement_notice", TargetID: noticeID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}
			notice, err := moderation.ReadNotice(ctx, f.sender.ID, noticeID, event)
			if err != nil || notice.Decision.AffectedPathID != f.pathID {
				t.Fatalf("notice: %+v %v", notice, err)
			}
			appeal, err := notice.Decision.NewAppeal(newTestID(), "Please review", at)
			if err != nil {
				t.Fatal(err)
			}
			event.ID = newTestID()
			event.Action = audit.ResourceCreated
			event.TargetType = "moderation_appeal"
			if _, err = moderation.SubmitAppeal(ctx, moderationapp.AppealCommand{OwnerID: f.sender.ID, Appeal: appeal, Audit: event}); err != nil {
				t.Fatal(err)
			}
			for _, fn := range []string{"moderation_decide_comment_appeal", "moderation_decide_nudge_appeal", "moderation_decide_feed_event_appeal"} {
				if err = f.migration.DB.Exec("SELECT "+fn+"(?,?,?,?)", noticeID, "reversed", "Reviewed", "Only reviewer").Error; err == nil {
					t.Fatalf("%s accepted wrong kind", fn)
				}
			}
			switch after {
			case "goals":
				exec(`UPDATE path_models SET overall_target_seconds=3600,updated_at=clock_timestamp() WHERE id=?`, f.pathID)
			case "renamed", "renamed_back":
				exec(`UPDATE path_models SET name='New name',updated_at=clock_timestamp() WHERE id=?`, f.pathID)
				if after == "renamed_back" {
					exec(`UPDATE path_models SET name='Name removed',updated_at=clock_timestamp() WHERE id=?`, f.pathID)
				}
			case "transferred", "transferred_back":
				exec(`UPDATE path_models SET owner_user_id=? WHERE id=?`, f.recipient.ID, f.pathID)
				if after == "transferred_back" {
					exec(`UPDATE path_models SET owner_user_id=? WHERE id=?`, f.sender.ID, f.pathID)
				}
			case "deleted":
				exec(`DELETE FROM path_models WHERE id=?`, f.pathID)
			}
			if err = f.runtime.DB.Exec(`SELECT moderation_decide_path_name_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "Only reviewer").Error; err == nil {
				t.Fatal("runtime admitted appeal")
			}
			if err = f.migration.DB.Exec(`SELECT moderation_decide_path_name_appeal(?,?,?,?)`, noticeID, "reversed", "Reviewed", "").Error; err == nil {
				t.Fatal("same reviewer omitted reason")
			}
			outcome := "reversed"
			if after == "upheld" {
				outcome = "upheld"
			}
			exec(`SELECT moderation_decide_path_name_appeal(?,?,?,?)`, noticeID, outcome, "Reviewed", "Only reviewer")
			if err = f.migration.DB.Exec(`SELECT moderation_decide_path_name_appeal(?,?,?,?)`, noticeID, "reversed", "Again", "Only reviewer").Error; err == nil {
				t.Fatal("final decision overwritten")
			}
			var rows []struct{ Name string }
			if err = f.runtime.DB.Table("path_models").Select("name").Where("id=?", f.pathID).Scan(&rows).Error; err != nil {
				t.Fatal(err)
			}
			if after == "deleted" {
				if len(rows) != 0 {
					t.Fatal("deleted Path recreated")
				}
				return
			}
			want := "Piano"
			if after == "renamed" {
				want = "New name"
			}
			if after == "renamed_back" || after == "transferred" || after == "transferred_back" || after == "upheld" {
				want = "Name removed"
			}
			if len(rows) != 1 || rows[0].Name != want {
				t.Fatalf("after %s: %+v want %s", after, rows, want)
			}
		})
	}
}

func TestPostgresModeratedPathNameSerializesPushHandoff(t *testing.T) {
	f := newNudgeFixture(t, "namepush", false)
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
	if err := f.migration.DB.Exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'path',?,'spam_or_scam','','{"title":"Piano"}',?)`, caseID, f.recipient.ID, f.sender.ID, f.pathID, f.now).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.DB.Exec(`SELECT moderation_remove_path_name(?,?,?,?)`, caseID, noticeID, "Policy reason", "Name removed").Error; err == nil {
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
	if err := tx.Exec(`SELECT moderation_remove_path_name(?,?,?,?)`, caseID, noticeID, "Policy reason", "Name removed").Error; err != nil {
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

	after, err := notifications.ListNotifications(ctx, f.recipient.ID, page)
	if err != nil || len(after.Items) != 1 || after.Items[0].PathName != "Name removed" {
		t.Fatalf("history should remain with new name: %+v %v", after, err)
	}
}

func TestPostgresModeratedPathNameRejectsStaleCaseAndRestrictedRoles(t *testing.T) {
	f := newNudgeFixture(t, "namestale", false)
	caseID := newTestID()
	if err := f.migration.DB.Exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'path',?,'spam_or_scam','','{"title":"Old name"}',?)`, caseID, f.recipient.ID, f.sender.ID, f.pathID, f.now).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Exec(`SELECT moderation_remove_path_name(?,?,?,?)`, caseID, newTestID(), "Reviewed", "Name removed").Error; err == nil {
		t.Fatal("stale report removed new name")
	}
	for _, role := range []string{"app", "app_audit_retention"} {
		var allowed bool
		if err := f.migration.DB.Raw(`SELECT has_function_privilege(?, 'moderation_remove_path_name(text,text,text,text)', 'EXECUTE') OR has_function_privilege(?, 'moderation_decide_path_name_appeal(text,text,text,text)', 'EXECUTE') OR has_table_privilege(?, 'moderation_removed_path_name_models', 'SELECT')`, role, role, role).Scan(&allowed).Error; err != nil || allowed {
			t.Fatalf("restricted role %s: %v %v", role, allowed, err)
		}
	}
	var notices int64
	if err := f.runtime.DB.Table("moderation_enforcement_models").Where("case_id=?", caseID).Count(&notices).Error; err != nil || notices != 0 {
		t.Fatalf("partial decision: %d %v", notices, err)
	}
}
