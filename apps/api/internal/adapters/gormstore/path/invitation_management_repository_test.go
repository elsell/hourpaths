package pathstore

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresManagerListsAndAtomicallyCancelsAnotherManagersInvitation(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 8, 13, 0, 0, 123456000, time.UTC)
	owner, administrator, recipient, pathID := "cancel-owner", "cancel-admin", "cancel-recipient", "cancel-path"
	users := []string{owner, administrator, recipient}
	paths := []string{pathID}
	cleanupInvitationFixture(t, migrationDB, users, paths)
	t.Cleanup(func() { cleanupInvitationFixture(t, migrationDB, users, paths) })
	seedInvitationUser(t, migrationDB, owner, "Cancel.Owner", identity.ProfileVisibilityPrivate, now)
	seedInvitationUser(t, migrationDB, administrator, "Cancel.Admin", identity.ProfileVisibilityPrivate, now)
	seedInvitationUser(t, migrationDB, recipient, "Cancel.Reader", identity.ProfileVisibilityPrivate, now)
	if err := migrationDB.Table("push_installation_models").Create(map[string]any{
		"id": "cancel-installation", "owner_user_id": recipient, "provider": "expo", "platform": "ios", "locale": "en",
		"token_ciphertext": bytes.Repeat([]byte{7}, 17), "token_nonce": bytes.Repeat([]byte{8}, 12), "token_hash": bytes.Repeat([]byte{9}, 32),
		"created_at": now, "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	entity := domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Cancel invitation", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	seedGoalUpdatePath(t, migrationDB, entity, owner)
	if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: administrator, Role: string(domain.RoleAdministrator), JoinedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := New(runtimeDB)
	invitation, err := domain.NewInvitation("cancel-invitation", entity.ID, owner, recipient, domain.RoleSupporter, now)
	if err != nil {
		t.Fatal(err)
	}
	send := sendInvitationCommand(invitation, "cancel-notification", "cancel-send-key-0001", 1, "cancel-send-audit", owner)
	send.ExpectedRecipientUsername = "Cancel.Reader"
	if _, err := repository.Send(ctx, send); err != nil {
		t.Fatalf("Send() error=%v", err)
	}
	page, err := repository.ListManagedPending(ctx, administrator, entity.ID, application.InvitationPageRequest{Limit: 25, Snapshot: now.Add(time.Minute)})
	if err != nil || len(page.Items) != 1 || page.Items[0].Invitation != invitation || page.Items[0].Inviter.UserID != owner || page.Items[0].Recipient.UserID != recipient {
		t.Fatalf("ListManagedPending()=%+v %v", page, err)
	}
	idempotency := ports.Idempotency{PrincipalID: administrator, Operation: application.CancelInvitationOperation, Key: "cancel-key-000001", RequestHash: bytes.Repeat([]byte{2}, 32)}
	decision, err := repository.CancellationDecision(ctx, administrator, entity.ID, invitation.ID, idempotency)
	if err != nil || decision.Invitation != invitation || decision.Path.ID != entity.ID {
		t.Fatalf("CancellationDecision()=%+v %v", decision, err)
	}
	canceled, err := invitation.Cancel(now.Add(2 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	command := application.CancelInvitationCommand{ActorUserID: administrator, Invitation: canceled, Idempotency: idempotency, Audit: audit.Event{ID: "cancel-audit", OwnerUserID: owner, ActorUserID: administrator, Action: audit.PathInvitationCanceled, TargetType: "path_invitation", TargetID: string(invitation.ID), Outcome: audit.Succeeded, CorrelationID: "cancel-correlation", OccurredAt: canceled.CanceledAt}}
	rollback := command
	rollback.Audit.ID = send.Audit.ID
	if _, err := repository.Cancel(ctx, rollback); err == nil {
		t.Fatal("Cancel() with duplicate audit unexpectedly succeeded")
	}
	var pendingAfterRollback, notificationAfterRollback, outboxAfterRollback int64
	if err := migrationDB.Table("path_invitation_models").Where("id = ? AND canceled_at IS NULL", invitation.ID).Count(&pendingAfterRollback).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_models").Where("id = ? AND deleted_at IS NULL", send.Notification.ID).Count(&notificationAfterRollback).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_outbox_models").Where("notification_id = ? AND suppressed_at IS NULL", send.Notification.ID).Count(&outboxAfterRollback).Error; err != nil {
		t.Fatal(err)
	}
	if pendingAfterRollback != 1 || notificationAfterRollback != 1 || outboxAfterRollback != 1 {
		t.Fatalf("rollback pending=%d notification=%d outbox=%d", pendingAfterRollback, notificationAfterRollback, outboxAfterRollback)
	}
	result, err := repository.Cancel(ctx, command)
	if err != nil || result.Invitation != canceled || result.Replayed {
		t.Fatalf("Cancel()=%+v %v", result, err)
	}
	replay, err := repository.CancellationDecision(ctx, administrator, entity.ID, invitation.ID, idempotency)
	if err != nil || replay.Replay == nil || !replay.Replay.Replayed || replay.Replay.Invitation != canceled {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	if _, err := repository.CancellationDecision(ctx, administrator, entity.ID, "different-invitation", idempotency); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("key reuse error=%v", err)
	}
	var activeNotification, activeOutbox, suppressedDelivery, audits, recipientMemberships int64
	if err := migrationDB.Table("notification_models").Where("path_invitation_id = ? AND deleted_at IS NULL", invitation.ID).Count(&activeNotification).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_outbox_models").Where("notification_id = ? AND suppressed_at IS NULL", send.Notification.ID).Count(&activeOutbox).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_delivery_models").Where("notification_id = ? AND installation_id = ? AND suppressed_at = ? AND failure_code = ? AND token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL", send.Notification.ID, "cancel-installation", canceled.CanceledAt, "invitation_resolved").Count(&suppressedDelivery).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("audit_event_models").Where("action = ? AND target_id = ?", audit.PathInvitationCanceled, invitation.ID).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", pathID, recipient).Count(&recipientMemberships).Error; err != nil {
		t.Fatal(err)
	}
	if activeNotification != 0 || activeOutbox != 0 || suppressedDelivery != 1 || audits != 1 || recipientMemberships != 0 {
		t.Fatalf("effects notification=%d outbox=%d delivery=%d audits=%d recipientMemberships=%d", activeNotification, activeOutbox, suppressedDelivery, audits, recipientMemberships)
	}
}
