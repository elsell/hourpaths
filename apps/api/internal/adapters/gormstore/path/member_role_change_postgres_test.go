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
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPostgresChangeMemberRoleAtomicallyDeletesParticipantDataNotifiesAndReconcilesRelationships(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, administrator, member := "role-owner-"+suffix, "role-admin-"+suffix, "role-member-"+suffix
	pathID := "role-path-" + suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedGoalUpdateUsers(t, migrationDB, now, owner, administrator, member)
	seedGoalUpdatePath(t, migrationDB, domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Role change", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}, owner)
	if err := migrationDB.Create(&[]membershipModel{{PathID: pathID, UserID: administrator, Role: "administrator"}, {PathID: pathID, UserID: member, Role: "participant"}}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupDeletionFixture(migrationDB, []string{owner, administrator, member}, []string{pathID}) })
	if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{"id": "role-activity-" + suffix, "path_id": pathID, "participant_id": member, "started_at": now.Add(-time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("running_timer_models").Create(map[string]any{"id": "role-timer-" + suffix, "path_id": pathID, "participant_id": member, "started_at": now, "occurrence_time_zone": "Etc/UTC"}).Error; err != nil {
		t.Fatal(err)
	}

	sequence := 0
	command := application.ChangeMemberRoleCommand{
		ActorUserID: administrator, TargetUserID: member, PathID: domain.ID(pathID), ExpectedRole: domain.RoleParticipant, Role: domain.RoleSupporter, ChangedAt: now,
		Idempotency:  ports.Idempotency{PrincipalID: administrator, Operation: application.ChangeMemberRoleOperation, Key: "change-member-role-0001", RequestHash: bytes.Repeat([]byte{4}, 32)},
		Audit:        audit.Event{ID: "role-audit-" + suffix, OwnerUserID: administrator, ActorUserID: administrator, Action: audit.ResourceUpdated, TargetType: "path_member", TargetID: pathID + ":" + member, Outcome: audit.Succeeded, CorrelationID: "role-" + suffix, OccurredAt: now},
		Notification: application.MemberAccessNotification{ID: "role-notification-" + suffix, RecipientUserID: member, ActorUserID: administrator, PathID: domain.ID(pathID), Kind: application.NotificationPathMemberRoleChanged, Role: domain.RoleSupporter, CreatedAt: now},
		NewID:        func() string { sequence++; return fmt.Sprintf("role-auth-%s-%d", suffix, sequence) }, AuthorizationWorker: "role-test", AuthorizationLease: time.Minute,
	}
	failure := command
	failure.Idempotency.Key = "change-member-rollback-0001"
	failure.Idempotency.RequestHash = bytes.Repeat([]byte{7}, 32)
	failure.Audit.ID = "role-rollback-audit-" + suffix
	failure.Notification.ID = "role-rollback-notification-" + suffix
	if err := migrationDB.Create(fromAudit(failure.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := New(runtimeDB).ChangeMemberRole(context.Background(), failure); err == nil {
		t.Fatal("injected post-notification audit collision succeeded")
	}
	var rollbackMembership membershipModel
	if err := migrationDB.Where("path_id = ? AND user_id = ?", pathID, member).Take(&rollbackMembership).Error; err != nil || rollbackMembership.Role != "participant" {
		t.Fatalf("rollback membership=%+v err=%v", rollbackMembership, err)
	}
	var rollbackActivities, rollbackTimers, rollbackNotices int64
	if err := migrationDB.Table("recorded_activity_models").Where("path_id = ? AND participant_id = ?", pathID, member).Count(&rollbackActivities).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("running_timer_models").Where("path_id = ? AND participant_id = ?", pathID, member).Count(&rollbackTimers).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_models").Where("id = ?", failure.Notification.ID).Count(&rollbackNotices).Error; err != nil {
		t.Fatal(err)
	}
	if rollbackActivities != 1 || rollbackTimers != 1 || rollbackNotices != 0 {
		t.Fatalf("rollback activity=%d timer=%d notices=%d", rollbackActivities, rollbackTimers, rollbackNotices)
	}
	result, err := New(runtimeDB).ChangeMemberRole(context.Background(), command)
	if err != nil || result.Role != domain.RoleSupporter || !result.ActivityDeleted || len(result.AuthorizationChanges) != 2 {
		t.Fatalf("ChangeMemberRole()=%+v err=%v", result, err)
	}
	var membership membershipModel
	if err := migrationDB.Where("path_id = ? AND user_id = ?", pathID, member).Take(&membership).Error; err != nil || membership.Role != "supporter" {
		t.Fatalf("membership=%+v err=%v", membership, err)
	}
	for _, table := range []string{"recorded_activity_models", "running_timer_models"} {
		var count int64
		if err := migrationDB.Table(table).Where("path_id = ? AND participant_id = ?", pathID, member).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s rows=%d err=%v", table, count, err)
		}
	}
	var notices, pushes, audits, outbox int64
	if err := migrationDB.Table("notification_models").Where("id = ? AND recipient_user_id = ? AND actor_user_id = ? AND kind = ? AND presentation_class = ? AND channel = ? AND offered_role = ?", command.Notification.ID, member, administrator, application.NotificationPathMemberRoleChanged, application.NotificationInformational, "path_access", domain.RoleSupporter).Count(&notices).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_outbox_models").Where("notification_id = ?", command.Notification.ID).Count(&pushes).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("audit_event_models").Where("id = ?", command.Audit.ID).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("authorization_outbox_models").Where("resource_id = ? AND subject_id = ? AND relation IN ?", pathID, member, []string{"participant", "supporter"}).Count(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if notices != 1 || pushes != 1 || audits != 1 || outbox != 2 {
		t.Fatalf("notices=%d pushes=%d audits=%d outbox=%d", notices, pushes, audits, outbox)
	}
	replay, err := New(runtimeDB).ChangeMemberRole(context.Background(), command)
	if err != nil || !replay.Replayed {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
}

func TestPostgresChangeSupporterToParticipantPreservesContinuousMembershipAndRetainedData(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, supporter, pathID := "promote-owner-"+suffix, "promote-member-"+suffix, "promote-path-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedGoalUpdateUsers(t, migrationDB, now, owner, supporter)
	seedGoalUpdatePath(t, migrationDB, domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Promote", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}, owner)
	joinedAt := now.Add(-30 * time.Minute)
	if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: supporter, Role: "supporter", JoinedAt: joinedAt}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupDeletionFixture(migrationDB, []string{owner, supporter}, []string{pathID}) })
	if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{"id": "retained-" + suffix, "path_id": pathID, "participant_id": supporter, "started_at": now.Add(-time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	sequence := 0
	command := application.ChangeMemberRoleCommand{ActorUserID: owner, TargetUserID: supporter, PathID: domain.ID(pathID), ExpectedRole: domain.RoleSupporter, Role: domain.RoleParticipant, ChangedAt: now, Idempotency: ports.Idempotency{PrincipalID: owner, Operation: application.ChangeMemberRoleOperation, Key: "promote-member-role-0001", RequestHash: bytes.Repeat([]byte{5}, 32)}, Audit: audit.Event{ID: "promote-audit-" + suffix, OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceUpdated, TargetType: "path_member", TargetID: pathID + ":" + supporter, Outcome: audit.Succeeded, CorrelationID: "promote-" + suffix, OccurredAt: now}, Notification: application.MemberAccessNotification{ID: "promote-notification-" + suffix, RecipientUserID: supporter, ActorUserID: owner, PathID: domain.ID(pathID), Kind: application.NotificationPathMemberRoleChanged, Role: domain.RoleParticipant, CreatedAt: now}, NewID: func() string { sequence++; return fmt.Sprintf("promote-auth-%s-%d", suffix, sequence) }, AuthorizationWorker: "role-test", AuthorizationLease: time.Minute}
	result, err := New(runtimeDB).ChangeMemberRole(context.Background(), command)
	if err != nil || result.ActivityDeleted {
		t.Fatalf("ChangeMemberRole()=%+v err=%v", result, err)
	}
	var membership membershipModel
	if err := migrationDB.Where("path_id = ? AND user_id = ?", pathID, supporter).Take(&membership).Error; err != nil || membership.Role != "participant" || !membership.JoinedAt.Equal(joinedAt) {
		t.Fatalf("membership=%+v err=%v", membership, err)
	}
	var activities int64
	if err := migrationDB.Table("recorded_activity_models").Where("path_id = ? AND participant_id = ?", pathID, supporter).Count(&activities).Error; err != nil || activities != 1 {
		t.Fatalf("activities=%d err=%v", activities, err)
	}
}

func TestPostgresAdministratorGrantRevokeAndSelfStepDownPreserveParticipantDataAndEnforceHierarchy(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, administrator, participant := "admin-owner-"+suffix, "admin-manager-"+suffix, "admin-participant-"+suffix
	pathID := "admin-path-" + suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedGoalUpdateUsers(t, migrationDB, now, owner, administrator, participant)
	seedGoalUpdatePath(t, migrationDB, domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Administrator lifecycle", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}, owner)
	if err := migrationDB.Create(&[]membershipModel{{PathID: pathID, UserID: administrator, Role: "administrator", JoinedAt: now.Add(-time.Hour)}, {PathID: pathID, UserID: participant, Role: "participant", JoinedAt: now.Add(-time.Hour)}}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupDeletionFixture(migrationDB, []string{owner, administrator, participant}, []string{pathID})
	})
	if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{"id": "admin-activity-" + suffix, "path_id": pathID, "participant_id": participant, "started_at": now.Add(-time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}

	sequence := 0
	command := func(actor, target string, from, to domain.MembershipRole, key string) application.ChangeMemberRoleCommand {
		sequence++
		stamp := fmt.Sprintf("%s-%d", suffix, sequence)
		return application.ChangeMemberRoleCommand{
			ActorUserID: actor, TargetUserID: target, PathID: domain.ID(pathID), ExpectedRole: from, Role: to, ChangedAt: now,
			Idempotency:  ports.Idempotency{PrincipalID: actor, Operation: application.ChangeMemberRoleOperation, Key: key, RequestHash: bytes.Repeat([]byte{byte(sequence)}, 32)},
			Audit:        audit.Event{ID: "admin-audit-" + stamp, OwnerUserID: actor, ActorUserID: actor, Action: audit.ResourceUpdated, TargetType: "path_member", TargetID: pathID + ":" + target, Outcome: audit.Succeeded, CorrelationID: "admin-" + stamp, OccurredAt: now},
			Notification: application.MemberAccessNotification{ID: "admin-notification-" + stamp, RecipientUserID: target, ActorUserID: actor, PathID: domain.ID(pathID), Kind: application.NotificationPathMemberRoleChanged, Role: to, CreatedAt: now},
			NewID:        func() string { sequence++; return fmt.Sprintf("admin-auth-%s-%d", suffix, sequence) }, AuthorizationWorker: "admin-role-test", AuthorizationLease: time.Minute,
		}
	}
	repository := New(runtimeDB)

	grant := command(owner, participant, domain.RoleParticipant, domain.RoleAdministrator, "administrator-grant-0001")
	granted, err := repository.ChangeMemberRole(context.Background(), grant)
	if err != nil || granted.Role != domain.RoleAdministrator || granted.ActivityDeleted {
		t.Fatalf("grant=%+v err=%v", granted, err)
	}
	assertMembershipRoleAndActivityCount(t, migrationDB, pathID, participant, "administrator", 1)
	var grantNotices int64
	if err := migrationDB.Table("notification_models").Where("id = ? AND recipient_user_id = ? AND offered_role = ?", grant.Notification.ID, participant, domain.RoleAdministrator).Count(&grantNotices).Error; err != nil || grantNotices != 1 {
		t.Fatalf("grant notices=%d err=%v", grantNotices, err)
	}

	unauthorized := command(administrator, participant, domain.RoleAdministrator, domain.RoleParticipant, "administrator-cross-revoke-0001")
	if _, err := repository.ChangeMemberRole(context.Background(), unauthorized); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("administrator revoked another administrator: %v", err)
	}
	assertMembershipRoleAndActivityCount(t, migrationDB, pathID, participant, "administrator", 1)

	revoke := command(owner, participant, domain.RoleAdministrator, domain.RoleParticipant, "administrator-revoke-0001")
	revoked, err := repository.ChangeMemberRole(context.Background(), revoke)
	if err != nil || revoked.Role != domain.RoleParticipant || revoked.ActivityDeleted {
		t.Fatalf("revoke=%+v err=%v", revoked, err)
	}
	assertMembershipRoleAndActivityCount(t, migrationDB, pathID, participant, "participant", 1)

	grantAgain := command(owner, participant, domain.RoleParticipant, domain.RoleAdministrator, "administrator-grant-again-0001")
	if _, err := repository.ChangeMemberRole(context.Background(), grantAgain); err != nil {
		t.Fatal(err)
	}
	stepDown := command(participant, participant, domain.RoleAdministrator, domain.RoleParticipant, "administrator-step-down-0001")
	steppedDown, err := repository.ChangeMemberRole(context.Background(), stepDown)
	if err != nil || steppedDown.Role != domain.RoleParticipant || steppedDown.ActivityDeleted {
		t.Fatalf("step down=%+v err=%v", steppedDown, err)
	}
	assertMembershipRoleAndActivityCount(t, migrationDB, pathID, participant, "participant", 1)
	var selfNotices int64
	if err := migrationDB.Table("notification_models").Where("id = ? AND recipient_user_id = actor_user_id AND offered_role = ?", stepDown.Notification.ID, domain.RoleParticipant).Count(&selfNotices).Error; err != nil || selfNotices != 1 {
		t.Fatalf("self-step-down notices=%d err=%v", selfNotices, err)
	}

	creatorMutation := command(owner, owner, domain.RoleParticipant, domain.RoleAdministrator, "administrator-creator-target-0001")
	if _, err := repository.ChangeMemberRole(context.Background(), creatorMutation); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("creator role mutation error=%v", err)
	}
}

func assertMembershipRoleAndActivityCount(t *testing.T, db *gorm.DB, pathID, userID, role string, activityCount int64) {
	t.Helper()
	var membership membershipModel
	if err := db.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, userID).Take(&membership).Error; err != nil || membership.Role != role {
		t.Fatalf("membership=%+v err=%v want role=%s", membership, err, role)
	}
	var activities int64
	if err := db.Table("recorded_activity_models").Where("path_id = ? AND participant_id = ?", pathID, userID).Count(&activities).Error; err != nil || activities != activityCount {
		t.Fatalf("activities=%d err=%v want=%d", activities, err, activityCount)
	}
}
