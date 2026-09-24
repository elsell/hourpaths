package pathstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresVisibilityChangeIsAtomicNotifiesPrivateParticipantsAndReplaysOriginalSnapshot(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, member, administrator, supporter, pathID := "visibility-owner-"+suffix, "visibility-member-"+suffix, "visibility-admin-"+suffix, "visibility-supporter-"+suffix, "visibility-path-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedInvitationUser(t, migrationDB, owner, "owner."+suffix, identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, migrationDB, member, "member."+suffix, identity.ProfileVisibilityPrivate, now)
	seedInvitationUser(t, migrationDB, administrator, "admin."+suffix, identity.ProfileVisibilityPrivate, now)
	seedInvitationUser(t, migrationDB, supporter, "supporter."+suffix, identity.ProfileVisibilityPrivate, now)
	existing := domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Practice", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	seedGoalUpdatePath(t, migrationDB, existing, owner)
	if err := migrationDB.Create(&[]membershipModel{{PathID: pathID, UserID: member, Role: "participant"}, {PathID: pathID, UserID: administrator, Role: "administrator"}, {PathID: pathID, UserID: supporter, Role: "supporter"}}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupInvitationFixture(t, migrationDB, []string{owner, member, administrator, supporter}, []string{pathID})
	})
	sequence := 0
	ids := func() string { sequence++; return fmt.Sprintf("visibility-%s-%d", suffix, sequence) }
	changed, _ := existing.SetVisibility("followers", now)
	command := visibilityCommand(owner, existing.Visibility, changed, "visibility-change-key-0001", bytes.Repeat([]byte{1}, 32), "visibility-audit-"+suffix, now, ids)
	result, err := New(runtimeDB).SetVisibility(context.Background(), command)
	if err != nil || result.Path != changed || result.Replayed || len(result.AuthorizationChanges) != 1 || result.AuthorizationChanges[0].Relation != "followers_owner" || result.AuthorizationChanges[0].Operation != ports.AuthorizationTouch {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var notices, administratorNotices, pushes, audits int64
	if err := migrationDB.Table("notification_models").Where("path_id = ? AND recipient_user_id = ? AND kind = ? AND path_visibility = ?", pathID, member, application.NotificationPathVisibilityChanged, "followers").Count(&notices).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_outbox_models").Where("notification_id IN (SELECT id FROM notification_models WHERE path_id = ? AND kind = ?)", pathID, application.NotificationPathVisibilityChanged).Count(&pushes).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_models").Where("path_id = ? AND recipient_user_id = ? AND kind = ? AND path_visibility = ?", pathID, administrator, application.NotificationPathVisibilityChanged, "followers").Count(&administratorNotices).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("audit_event_models").Where("id = ? AND action = ?", command.Audit.ID, audit.PathVisibilityChanged).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	var supporterNotices int64
	_ = migrationDB.Table("notification_models").Where("path_id = ? AND recipient_user_id = ? AND kind = ?", pathID, supporter, application.NotificationPathVisibilityChanged).Count(&supporterNotices).Error
	if notices != 1 || administratorNotices != 1 || pushes != 2 || audits != 1 || supporterNotices != 0 {
		t.Fatalf("participant notices=%d administrator notices=%d pushes=%d audits=%d supporter=%d", notices, administratorNotices, pushes, audits, supporterNotices)
	}
	notificationPage, err := New(runtimeDB).ListNotifications(context.Background(), member, application.NotificationPageRequest{
		Limit: 10, Snapshot: now.Add(time.Minute),
	})
	if err != nil || notificationPage.UnreadCount != 1 || len(notificationPage.Items) != 1 {
		t.Fatalf("visibility notifications=%+v err=%v", notificationPage, err)
	}
	notification := notificationPage.Items[0]
	if notification.Kind != application.NotificationPathVisibilityChanged ||
		notification.PathID != domain.ID(pathID) || notification.PathName != "Practice" ||
		notification.PathVisibility != "followers" {
		t.Fatalf("visibility notification=%+v", notification)
	}
	later := now.Add(time.Minute)
	public, _ := changed.SetVisibility("public", later)
	second := visibilityCommand(owner, "followers", public, "visibility-change-key-0002", bytes.Repeat([]byte{2}, 32), "visibility-audit-public-"+suffix, later, ids)
	if err := migrationDB.Table("user_models").Where("id = ?", owner).Update("profile_visibility", identity.ProfileVisibilityPrivate).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := New(runtimeDB).SetVisibility(context.Background(), second); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("private owner public transition err=%v", err)
	}
	var retainedVisibility string
	if err := migrationDB.Table("path_models").Select("visibility").Where("id = ?", pathID).Scan(&retainedVisibility).Error; err != nil || retainedVisibility != "followers" {
		t.Fatalf("visibility after rejected profile race=%q err=%v", retainedVisibility, err)
	}
	if err := migrationDB.Table("user_models").Where("id = ?", owner).Update("profile_visibility", identity.ProfileVisibilityPublic).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := New(runtimeDB).SetVisibility(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	replay, err := New(runtimeDB).SetVisibility(context.Background(), command)
	if err != nil || !replay.Replayed || replay.Path != changed {
		t.Fatalf("original replay=%+v err=%v", replay, err)
	}
}

func visibilityCommand(actor, expected string, path domain.Entity, key string, hash []byte, auditID string, changedAt time.Time, newID func() string) application.SetVisibilityCommand {
	return application.SetVisibilityCommand{ActorUserID: actor, ExpectedVisibility: expected, Path: path, ChangedAt: changedAt, Idempotency: ports.Idempotency{PrincipalID: actor, Operation: application.SetVisibilityOperation, Key: key, RequestHash: hash}, Audit: audit.Event{ID: auditID, OwnerUserID: actor, ActorUserID: actor, Action: audit.PathVisibilityChanged, TargetType: "path", TargetID: string(path.ID), Outcome: audit.Succeeded, CorrelationID: auditID, OccurredAt: changedAt}, NewID: newID, AuthorizationWorker: "visibility-test", AuthorizationLease: time.Minute}
}
