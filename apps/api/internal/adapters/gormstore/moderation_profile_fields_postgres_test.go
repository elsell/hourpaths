package gormstore

import (
	"bytes"
	"context"
	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	moderationapp "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresModeratedProfileFields(t *testing.T) {
	for _, after := range []string{"unchanged", "text_changed", "picture_changed", "upheld"} {
		t.Run(after, func(t *testing.T) {
			f := newNudgeFixture(t, "profremoval", false)
			exec := func(q string, args ...any) {
				t.Helper()
				if err := f.migration.DB.Exec(q, args...).Error; err != nil {
					t.Fatal(err)
				}
			}
			caseID, noticeID, assetID := newTestID(), newTestID(), newTestID()
			exec(`UPDATE user_models SET display_name='Reported name',description='Reported bio',profile_picture_url='https://example.test/avatar' WHERE id=?`, f.sender.ID)
			exec(`INSERT INTO user_profile_picture_models(id,user_id,jpeg,created_at) VALUES(?,?,decode('ffd8ffd9','hex'),?)`, assetID, f.sender.ID, f.now)
			exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,evidence_jpeg,created_at) VALUES(?,?,?,'profile',?,'spam_or_scam','','{"title":"Reported name","text":"Reported bio"}',decode('ffd8ffd9','hex'),?)`, caseID, f.recipient.ID, f.sender.ID, f.sender.ID, f.now)
			// hourpaths-direct-sql: test runtime denial of restricted profile moderation capability
			if err := f.runtime.DB.Exec(`SELECT moderation_remove_profile_fields(?,?,?,ARRAY['display_name','description','picture'],'Name removed')`, caseID, noticeID, "Profile name, bio and picture removed for policy violation").Error; err == nil {
				t.Fatal("runtime removal allowed")
			}
			exec(`SELECT moderation_remove_profile_fields(?,?,?,ARRAY['display_name','description','picture'],'Name removed')`, caseID, noticeID, "Profile name, bio and picture removed for policy violation")
			profile, err := f.runtime.GetOwnProfile(context.Background(), f.sender.ID)
			if err != nil || profile.Text.DisplayName != "Name removed" || profile.Text.Description != "" {
				t.Fatalf("profile %+v %v", profile, err)
			}
			if _, err = f.runtime.ReadPublicPicture(context.Background(), assetID); err == nil {
				t.Fatal("removed photo still public")
			}
			var paths int64
			if err = f.runtime.DB.Table("path_models").Where("id=? AND owner_user_id=?", f.pathID, f.sender.ID).Count(&paths).Error; err != nil || paths != 1 {
				t.Fatal("profile Path lost", err)
			}
			at := time.Now().UTC()
			event := audit.Event{ID: newTestID(), OwnerUserID: f.sender.ID, ActorUserID: f.sender.ID, Action: audit.ResourceViewed, TargetType: "enforcement_notice", TargetID: noticeID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}
			repo := NewModerationRepository(f.runtime.DB)
			notice, err := repo.ReadNotice(context.Background(), f.sender.ID, noticeID, event)
			if err != nil {
				t.Fatal(err)
			}
			appeal, err := notice.Decision.NewAppeal(newTestID(), "Please review", at)
			if err != nil {
				t.Fatal(err)
			}
			event.ID = newTestID()
			event.Action = audit.ResourceCreated
			event.TargetType = "moderation_appeal"
			if _, err = repo.SubmitAppeal(context.Background(), moderationapp.AppealCommand{OwnerID: f.sender.ID, Appeal: appeal, Audit: event}); err != nil {
				t.Fatal(err)
			}
			switch after {
			case "text_changed":
				exec(`UPDATE user_models SET display_name='New name',profile_revision=profile_revision+1 WHERE id=?`, f.sender.ID)
				exec(`UPDATE user_models SET display_name='Name removed',profile_revision=profile_revision+1 WHERE id=?`, f.sender.ID)
			case "picture_changed":
				exec(`UPDATE user_models SET picture_revision=picture_revision+1 WHERE id=?`, f.sender.ID)
			}
			outcome := "reversed"
			if after == "upheld" {
				outcome = "upheld"
			}
			exec(`SELECT moderation_decide_profile_fields_appeal(?,?,?,?)`, noticeID, outcome, "Reviewed", "Only reviewer")
			// hourpaths-direct-sql: verify final appeal immutability through the restricted boundary
			if err = f.migration.DB.Exec(`SELECT moderation_decide_profile_fields_appeal(?,?,?,?)`, noticeID, "reversed", "Again", "Only reviewer").Error; err == nil {
				t.Fatal("duplicate decision admitted")
			}
			profile, err = f.runtime.GetOwnProfile(context.Background(), f.sender.ID)
			want := "Reported name"
			if after == "text_changed" || after == "upheld" {
				want = "Name removed"
			}
			if err != nil || profile.Text.DisplayName != want {
				t.Fatalf("after %+v %v", profile, err)
			}
			_, err = f.runtime.ReadPublicPicture(context.Background(), assetID)
			wantPicture := after != "picture_changed" && after != "upheld"
			if (err == nil) != wantPicture {
				t.Fatalf("restoration photo=%v err=%v", wantPicture, err)
			}
		})
	}
}

func TestPostgresModeratedProfileSelectionAndPermissions(t *testing.T) {
	f := newNudgeFixture(t, "profselect", false)
	exec := func(q string, args ...any) {
		t.Helper()
		if err := f.migration.DB.Exec(q, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	caseID, noticeID := newTestID(), newTestID()
	exec(`UPDATE user_models SET display_name='Unchanged name',description='Reported bio' WHERE id=?`, f.sender.ID)
	exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'profile',?,'spam_or_scam','','{"title":"Stale name","text":"Reported bio"}',?)`, caseID, f.recipient.ID, f.sender.ID, f.sender.ID, f.now)
	for _, fields := range []string{"ARRAY['display_name']", "ARRAY['description','description']", "ARRAY[NULL]::text[]", "ARRAY['username']", "ARRAY[]::text[]", "NULL"} {
		// hourpaths-direct-sql: fixed adversarial field allowlist and stale-evidence calls
		if err := f.migration.DB.Exec(`SELECT moderation_remove_profile_fields(?,?,?,`+fields+`,'Name removed')`, caseID, noticeID, "Profile fields removed").Error; err == nil {
			t.Fatalf("invalid selection accepted %s", fields)
		}
	}
	exec(`SELECT moderation_remove_profile_fields(?,?,?,ARRAY['description'],NULL)`, caseID, noticeID, "Profile bio removed for policy violation")
	profile, err := f.runtime.GetOwnProfile(context.Background(), f.sender.ID)
	if err != nil || profile.Text.DisplayName != "Unchanged name" || profile.Text.Description != "" || profile.Text.Username != *f.sender.Username {
		t.Fatalf("unselected identity changed: %+v %v", profile, err)
	}
	// hourpaths-direct-sql: verify duplicate removals cannot create another private notice
	if err = f.migration.DB.Exec(`SELECT moderation_remove_profile_fields(?,?,?,ARRAY['description'],NULL)`, caseID, newTestID(), "Again").Error; err == nil {
		t.Fatal("duplicate removal admitted")
	}
	for _, role := range []string{"app", "app_audit_retention"} {
		var allowed bool
		// hourpaths-direct-sql: real database privilege inspection for restricted operational capabilities
		if err = f.migration.DB.Raw(`SELECT has_function_privilege(?, 'moderation_remove_profile_fields(text,text,text,text[],text)', 'EXECUTE') OR has_function_privilege(?, 'moderation_decide_profile_fields_appeal(text,text,text,text)', 'EXECUTE') OR has_table_privilege(?, 'moderation_removed_profile_field_models', 'SELECT')`, role, role, role).Scan(&allowed).Error; err != nil || allowed {
			t.Fatalf("%s privilege leak: %v %v", role, allowed, err)
		}
	}
}

func TestPostgresModeratedProfileSerializesPushHandoff(t *testing.T) {
	f := newNudgeFixture(t, "profilepush", false)
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
	if err := f.migration.DB.Exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) SELECT ?,?,u.id,'profile',u.id,'spam_or_scam','',jsonb_build_object('title',u.display_name),? FROM user_models u WHERE u.id=?`, caseID, f.recipient.ID, f.now, f.sender.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.DB.Exec(`SELECT moderation_remove_profile_fields(?,?,?,ARRAY['display_name'],?)`, caseID, noticeID, "Policy reason", "Name removed").Error; err == nil {
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
	if err := tx.Exec(`SELECT moderation_remove_profile_fields(?,?,?,ARRAY['display_name'],?)`, caseID, noticeID, "Policy reason", "Name removed").Error; err != nil {
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
	if err != nil || len(after.Items) != 1 || after.Items[0].PathName != "Piano" {
		t.Fatalf("history should remain: %+v %v", after, err)
	}
}
