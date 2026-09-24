package pathstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPostgresPathDeletionAtomicallyDiscardsSharedDataAndCreatesStandaloneNotices(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 16, 0, 0, 0, time.UTC)
	owner, participant, supporter := "path08-delete-owner", "path08-delete-participant", "path08-delete-supporter"
	pathID, otherPathID := "path08-delete-path", "path08-delete-other"
	users, paths := []string{owner, participant, supporter}, []string{pathID, otherPathID}
	cleanupDeletionFixture(migrationDB, users, paths)
	t.Cleanup(func() { cleanupDeletionFixture(migrationDB, users, paths) })
	seedGoalUpdateUsers(t, migrationDB, now, users...)
	for index, userID := range users {
		username := fmt.Sprintf("Path08.Delete.User.%d", index)
		if err := migrationDB.Table("user_models").Where("id = ?", userID).Updates(map[string]any{"username": username, "display_name": "Delete User"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	path := domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Shared Guitar", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	seedGoalUpdatePath(t, migrationDB, path, owner)
	if err := migrationDB.Create(&[]membershipModel{{PathID: pathID, UserID: participant, Role: "participant"}, {PathID: pathID, UserID: supporter, Role: "supporter"}}).Error; err != nil {
		t.Fatal(err)
	}
	other := domain.Entity{ID: domain.ID(otherPathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Other", Visibility: "private"}, CreatedAt: path.CreatedAt, UpdatedAt: path.UpdatedAt}
	seedGoalUpdatePath(t, migrationDB, other, owner)
	if err := migrationDB.Create(&[]archiveTimerRow{
		{ID: "path08-delete-running", PathID: pathID, ParticipantID: participant, StartedAt: now.Add(-time.Minute), OccurrenceTimeZone: "Etc/UTC"},
		{ID: "path08-delete-other-running", PathID: otherPathID, ParticipantID: owner, StartedAt: now.Add(-time.Minute), OccurrenceTimeZone: "Etc/UTC"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	activity := goalUpdateActivityRow{ID: "path08-delete-activity", PathID: pathID, ParticipantID: participant, StartedAt: now.Add(-10 * time.Minute), EndedAt: now.Add(-5 * time.Minute), OccurrenceTimeZone: "Etc/UTC", CreatedAt: now.Add(-5 * time.Minute), UpdatedAt: now.Add(-5 * time.Minute)}
	if err := migrationDB.Create(&activity).Error; err != nil {
		t.Fatal(err)
	}

	counter := 0
	newID := func() string { counter++; return fmt.Sprintf("path08-delete-generated-%03d", counter) }
	command := application.DeletePathCommand{
		ActorUserID: owner, PathID: path.ID, ExpectedName: path.Name, DeletedAt: now,
		Idempotency: ports.Idempotency{PrincipalID: owner, Operation: application.DeletePathOperation, Key: "path08-delete-path-key-0001", RequestHash: bytes.Repeat([]byte{1}, 32)},
		Audit:       audit.Event{ID: "path08-delete-audit", OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceDeleted, TargetType: "path", TargetID: pathID, Outcome: audit.Succeeded, CorrelationID: "path08-delete", OccurredAt: now},
		NewID:       newID, AuthorizationWorker: "path08-delete-worker", AuthorizationLease: time.Minute,
	}
	result, err := New(runtimeDB).DeletePath(ctx, command)
	if err != nil || !result.Deleted || result.PathID != path.ID || result.Replayed || len(result.AuthorizationChanges) != 12 {
		t.Fatalf("DeletePath() = %+v, %v", result, err)
	}
	for table, column := range map[string]string{"path_models": "id", "path_membership_models": "path_id", "running_timer_models": "path_id", "recorded_activity_models": "path_id"} {
		var count int64
		if err := migrationDB.Table(table).Where(column+" = ?", pathID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s retained %d rows: %v", table, count, err)
		}
	}
	var otherTimers int64
	if err := migrationDB.Table("running_timer_models").Where("path_id = ?", otherPathID).Count(&otherTimers).Error; err != nil || otherTimers != 1 {
		t.Fatalf("unrelated timers=%d, %v", otherTimers, err)
	}
	var notices []notificationModel
	if err := migrationDB.Where("kind = ? AND recipient_user_id IN ?", application.NotificationPathDeleted, []string{participant, supporter}).Order("recipient_user_id").Find(&notices).Error; err != nil {
		t.Fatal(err)
	}
	if len(notices) != 2 {
		t.Fatalf("notices=%+v", notices)
	}
	for _, notice := range notices {
		if notice.PathID != "" || notice.PathNameSnapshot == nil || *notice.PathNameSnapshot != path.Name || notice.ActorUsernameSnapshot == nil || notice.ActorDisplayNameSnapshot == nil {
			t.Fatalf("standalone notice=%+v", notice)
		}
	}
	replay, err := New(runtimeDB).DeletePath(ctx, command)
	if err != nil || !replay.Replayed || !replay.Deleted || replay.PathID != path.ID {
		t.Fatalf("replay=%+v, %v", replay, err)
	}
	var noticeCount int64
	if err := migrationDB.Model(&notificationModel{}).Where("kind = ? AND recipient_user_id IN ?", application.NotificationPathDeleted, []string{participant, supporter}).Count(&noticeCount).Error; err != nil || noticeCount != 2 {
		t.Fatalf("replay notices=%d, %v", noticeCount, err)
	}

	stale := command
	stale.Idempotency.Key, stale.Idempotency.RequestHash, stale.ExpectedName, stale.Audit.ID = "path08-delete-path-key-stale", bytes.Repeat([]byte{2}, 32), "Renamed", "path08-delete-stale-audit"
	stale.PathID = other.ID
	stale.Audit.TargetID = otherPathID
	if _, err := New(runtimeDB).DeletePath(ctx, stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale name error=%v", err)
	}
}

func TestPostgresPathDeletionEmitsVisibilityAuthorizationCleanup(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	now := time.Date(2026, 7, 27, 17, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, visibility, relation, subject string
	}{
		{name: "public wildcard", visibility: "public", relation: "public_viewer", subject: "*"},
		{name: "followers owner", visibility: "followers", relation: "followers_owner", subject: "owner"},
	} {
		t.Run(test.name, func(t *testing.T) {
			suffix := strings.ReplaceAll(test.visibility, "_", "-")
			owner, pathID := "path08-visibility-owner-"+suffix, "path08-visibility-path-"+suffix
			if test.subject == "owner" {
				test.subject = owner
			}
			cleanupDeletionFixture(migrationDB, []string{owner}, []string{pathID})
			t.Cleanup(func() { cleanupDeletionFixture(migrationDB, []string{owner}, []string{pathID}) })
			seedGoalUpdateUsers(t, migrationDB, now, owner)
			if err := migrationDB.Table("user_models").Where("id = ?", owner).Updates(map[string]any{
				"username": "Path08.Visibility." + suffix, "display_name": "Visibility Owner",
			}).Error; err != nil {
				t.Fatal(err)
			}
			path := domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Visibility cleanup", Visibility: test.visibility}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
			seedGoalUpdatePath(t, migrationDB, path, owner)
			counter := 0
			command := application.DeletePathCommand{
				ActorUserID: owner, PathID: path.ID, ExpectedName: path.Name, DeletedAt: now,
				Idempotency: ports.Idempotency{PrincipalID: owner, Operation: application.DeletePathOperation, Key: "path08-delete-visibility-key-" + suffix, RequestHash: bytes.Repeat([]byte{3}, 32)},
				Audit:       audit.Event{ID: "path08-delete-visibility-audit-" + suffix, OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceDeleted, TargetType: "path", TargetID: pathID, Outcome: audit.Succeeded, CorrelationID: "path08-delete-visibility-" + suffix, OccurredAt: now},
				NewID: func() string {
					counter++
					return fmt.Sprintf("path08-delete-visibility-%s-%03d", suffix, counter)
				},
				AuthorizationWorker: "path08-delete-worker", AuthorizationLease: time.Minute,
			}
			result, err := New(runtimeDB).DeletePath(context.Background(), command)
			if err != nil || len(result.AuthorizationChanges) != 5 {
				t.Fatalf("DeletePath() = %+v, %v", result, err)
			}
			visibilityDelete := result.AuthorizationChanges[4]
			if visibilityDelete.Relation != test.relation || visibilityDelete.SubjectType != "user" || visibilityDelete.SubjectID != test.subject || visibilityDelete.Operation != ports.AuthorizationDelete {
				t.Fatalf("visibility cleanup = %+v", visibilityDelete)
			}
		})
	}
}

func cleanupDeletionFixture(db *gorm.DB, users, paths []string) {
	if len(paths) > 0 {
		_ = db.Table("authorization_outbox_models").Where("resource_type = ? AND resource_id IN ?", "path", paths).Delete(&authorizationOutboxModel{}).Error
		_ = db.Table("path_models").Where("id IN ?", paths).Delete(&model{}).Error
	}
	if len(users) > 0 {
		_ = db.Table("audit_event_models").Where("owner_user_id IN ?", users).Delete(&auditModel{}).Error
		_ = db.Table("idempotency_models").Where("principal_id IN ?", users).Delete(&idempotencyModel{}).Error
		_ = db.Table("user_models").Where("id IN ?", users).Delete(map[string]any{}).Error
	}
}
