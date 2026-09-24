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

func TestPostgresPathInvitationRejectIsAtomicReplayableAndCreatesNoAccess(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 15, 0, 0, 123_456_000, time.UTC)
	owner, recipient := "path-reject-owner", "path-reject-recipient"
	pathID := "path-reject-path"
	cleanupInvitationFixture(t, migrationDB, []string{owner, recipient}, []string{pathID})
	t.Cleanup(func() {
		cleanupInvitationFixture(t, migrationDB, []string{owner, recipient}, []string{pathID})
	})
	seedInvitationUser(t, migrationDB, owner, "Reject.Owner", identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, migrationDB, recipient, "Reject.Reader", identity.ProfileVisibilityPrivate, now)
	if err := migrationDB.Table("push_installation_models").Create(map[string]any{
		"id": "path-reject-installation", "owner_user_id": recipient,
		"provider": "expo", "platform": "ios", "locale": "en",
		"token_ciphertext": bytes.Repeat([]byte{1}, 17),
		"token_nonce":      bytes.Repeat([]byte{2}, 12),
		"token_hash":       bytes.Repeat([]byte{3}, 32),
		"created_at":       now, "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	path := domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{Name: "Reject invitation", Visibility: "private"},
		CreatedAt:  now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	seedGoalUpdatePath(t, migrationDB, path, owner)
	repository := New(runtimeDB)
	invitation, err := domain.NewInvitation(
		"path-reject-invitation", path.ID, owner, recipient, domain.RoleSupporter, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	send := sendInvitationCommand(
		invitation, "path-reject-notification", "path-reject-send-key", 1,
		"path-reject-send-audit", owner,
	)
	send.ExpectedRecipientUsername = "Reject.Reader"
	if _, err := repository.Send(ctx, send); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	var activeDeliveries, activeOutbox int64
	if err := migrationDB.Table("notification_push_delivery_models").Where(
		"notification_id = ? AND installation_id = ? AND suppressed_at IS NULL AND token_ciphertext IS NOT NULL",
		send.Notification.ID, "path-reject-installation",
	).Count(&activeDeliveries).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_outbox_models").Where(
		"notification_id = ? AND suppressed_at IS NULL", send.Notification.ID,
	).Count(&activeOutbox).Error; err != nil {
		t.Fatal(err)
	}
	if activeDeliveries != 1 || activeOutbox != 1 {
		t.Fatalf("seeded push delivery=%d outbox=%d, want one active each", activeDeliveries, activeOutbox)
	}
	idempotency := ports.Idempotency{
		PrincipalID: recipient, Operation: application.RejectInvitationOperation,
		Key: "path-reject-key-0001", RequestHash: bytes.Repeat([]byte{4}, 32),
	}
	decision, err := repository.RejectionDecision(ctx, recipient, invitation.ID, idempotency)
	if err != nil || decision.Invitation != invitation || decision.OwnerUserID != owner || decision.Replay != nil {
		t.Fatalf("RejectionDecision() = %+v, %v", decision, err)
	}
	rejected, err := invitation.Reject(recipient, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	command := rejectInvitationCommand(rejected, idempotency, "path-reject-audit", owner)
	rollbackCommand := command
	rollbackCommand.Audit.ID = send.Audit.ID
	if _, err := repository.Reject(ctx, rollbackCommand); err == nil {
		t.Fatal("Reject() with duplicate audit unexpectedly succeeded")
	}
	var pendingAfterRollback int64
	if err := migrationDB.Model(&invitationModel{}).Where(
		"id = ? AND rejected_at IS NULL AND accepted_at IS NULL", invitation.ID,
	).Count(&pendingAfterRollback).Error; err != nil || pendingAfterRollback != 1 {
		t.Fatalf("rejection audit failure rollback pending=%d error=%v", pendingAfterRollback, err)
	}
	activeDeliveries, activeOutbox = 0, 0
	if err := migrationDB.Table("notification_push_delivery_models").Where(
		"notification_id = ? AND installation_id = ? AND suppressed_at IS NULL AND token_ciphertext IS NOT NULL",
		send.Notification.ID, "path-reject-installation",
	).Count(&activeDeliveries).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_outbox_models").Where(
		"notification_id = ? AND suppressed_at IS NULL", send.Notification.ID,
	).Count(&activeOutbox).Error; err != nil {
		t.Fatal(err)
	}
	if activeDeliveries != 1 || activeOutbox != 1 {
		t.Fatalf("audit rollback push delivery=%d outbox=%d, want one active each", activeDeliveries, activeOutbox)
	}
	result, err := repository.Reject(ctx, command)
	if err != nil || result.Invitation != rejected || result.Replayed || result.UnreadCount != 0 {
		t.Fatalf("Reject() = %+v, %v", result, err)
	}
	var suppressedDeliveries, suppressedOutbox int64
	if err := migrationDB.Table("notification_push_delivery_models").Where(
		"notification_id = ? AND installation_id = ? AND suppressed_at = ? AND failure_code = ? AND token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL",
		send.Notification.ID, "path-reject-installation", rejected.RejectedAt, "invitation_resolved",
	).Count(&suppressedDeliveries).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_push_outbox_models").Where(
		"notification_id = ? AND suppressed_at = ? AND failure_code = ?",
		send.Notification.ID, rejected.RejectedAt, "invitation_resolved",
	).Count(&suppressedOutbox).Error; err != nil {
		t.Fatal(err)
	}
	if suppressedDeliveries != 1 || suppressedOutbox != 1 {
		t.Fatalf("rejected push delivery=%d outbox=%d, want one suppressed each", suppressedDeliveries, suppressedOutbox)
	}
	secondInvitation, err := domain.NewInvitation(
		"path-reject-second-invitation", path.ID, owner, recipient,
		domain.RoleParticipant, now.Add(2*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondSend := sendInvitationCommand(
		secondInvitation, "path-reject-second-notification", "path-reject-second-send-key", 5,
		"path-reject-second-send-audit", owner,
	)
	secondSend.ExpectedRecipientUsername = "Reject.Reader"
	if _, err := repository.Send(ctx, secondSend); err != nil {
		t.Fatalf("second Send() error = %v", err)
	}
	replayDecision, err := repository.RejectionDecision(ctx, recipient, invitation.ID, idempotency)
	if err != nil || replayDecision.Replay == nil || !replayDecision.Replay.Replayed ||
		replayDecision.Replay.Invitation != rejected || replayDecision.Replay.UnreadCount != 0 {
		t.Fatalf("replay decision = %+v, %v", replayDecision, err)
	}
	replay, err := repository.Reject(ctx, command)
	if err != nil || !replay.Replayed || replay.Invitation != rejected {
		t.Fatalf("direct replay = %+v, %v", replay, err)
	}
	newKey := idempotency
	newKey.Key = "path-reject-key-0002"
	if _, err := repository.RejectionDecision(ctx, recipient, invitation.ID, newKey); !errors.Is(err, domain.ErrInvitationUnavailable) {
		t.Fatalf("new-key rejection error = %v, want opaque unavailable", err)
	}

	var memberships, authorizationChanges, successAudits, outgoingNotifications, activeReceived, reservations int64
	if err := migrationDB.Model(&membershipModel{}).Where("path_id = ? AND user_id = ?", pathID, recipient).Count(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("authorization_outbox_models").Where("resource_type = ? AND resource_id = ? AND subject_id = ?", "path", pathID, recipient).Count(&authorizationChanges).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&auditModel{}).Where("action = ? AND target_id = ?", audit.PathInvitationRejected, invitation.ID).Count(&successAudits).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&notificationModel{}).Where("path_invitation_id = ? AND kind = ?", invitation.ID, "path_invitation_accepted").Count(&outgoingNotifications).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&notificationModel{}).Where("path_invitation_id = ? AND kind = ? AND deleted_at IS NULL", invitation.ID, "path_invitation_received").Count(&activeReceived).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&idempotencyModel{}).Where("principal_id = ? AND operation = ?", recipient, application.RejectInvitationOperation).Count(&reservations).Error; err != nil {
		t.Fatal(err)
	}
	if memberships != 0 || authorizationChanges != 0 || successAudits != 1 || outgoingNotifications != 0 || activeReceived != 0 || reservations != 1 {
		t.Fatalf("rejection effects memberships=%d auth=%d audits=%d outgoing=%d activeReceived=%d reservations=%d", memberships, authorizationChanges, successAudits, outgoingNotifications, activeReceived, reservations)
	}
}

func TestPostgresPathInvitationConcurrentAcceptAndRejectHaveOneTerminalWinner(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 16, 0, 0, 123_456_000, time.UTC)
	owner, recipient := "path-terminal-race-owner", "path-terminal-race-recipient"
	pathID := "path-terminal-race-path"
	cleanupInvitationFixture(t, migrationDB, []string{owner, recipient}, []string{pathID})
	t.Cleanup(func() {
		cleanupInvitationFixture(t, migrationDB, []string{owner, recipient}, []string{pathID})
	})
	seedInvitationUser(t, migrationDB, owner, "Terminal.Owner", identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, migrationDB, recipient, "Terminal.Reader", identity.ProfileVisibilityPublic, now)
	path := domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{Name: "Terminal race", Visibility: "private"},
		CreatedAt:  now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	seedGoalUpdatePath(t, migrationDB, path, owner)
	repository := New(runtimeDB)
	invitation, err := domain.NewInvitation(
		"path-terminal-race-invitation", path.ID, owner, recipient, domain.RoleParticipant, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	send := sendInvitationCommand(
		invitation, "path-terminal-race-notification", "path-terminal-race-send-key", 1,
		"path-terminal-race-send-audit", owner,
	)
	send.ExpectedRecipientUsername = "Terminal.Reader"
	if _, err := repository.Send(ctx, send); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	terminalAt := now.Add(time.Minute)
	accepted, err := invitation.Accept(recipient, terminalAt)
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := invitation.Reject(recipient, terminalAt)
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := invitation.Cancel(terminalAt)
	if err != nil {
		t.Fatal(err)
	}
	acceptCommand := acceptInvitationCommand(
		accepted,
		ports.AuthorizationChange{
			ID: "path-terminal-race-change", ResourceType: "path", ResourceID: pathID,
			Relation: string(domain.RoleParticipant), SubjectType: "user", SubjectID: recipient,
			OwnerUserID: owner, ActorUserID: recipient, Operation: ports.AuthorizationTouch,
			LockedBy: "path-terminal-race-worker", Lease: time.Minute,
		},
		ports.Idempotency{
			PrincipalID: recipient, Operation: application.AcceptInvitationOperation,
			Key: "path-terminal-race-accept", RequestHash: bytes.Repeat([]byte{2}, 32),
		},
		"path-terminal-race-accepted-notification", "path-terminal-race-accept-audit", owner,
	)
	rejectCommand := rejectInvitationCommand(
		rejected,
		ports.Idempotency{
			PrincipalID: recipient, Operation: application.RejectInvitationOperation,
			Key: "path-terminal-race-reject", RequestHash: bytes.Repeat([]byte{3}, 32),
		},
		"path-terminal-race-reject-audit", owner,
	)
	cancelCommand := application.CancelInvitationCommand{
		ActorUserID: owner, Invitation: canceled,
		Idempotency: ports.Idempotency{PrincipalID: owner, Operation: application.CancelInvitationOperation, Key: "path-terminal-race-cancel", RequestHash: bytes.Repeat([]byte{4}, 32)},
		Audit:       audit.Event{ID: "path-terminal-race-cancel-audit", OwnerUserID: owner, ActorUserID: owner, Action: audit.PathInvitationCanceled, TargetType: "path_invitation", TargetID: string(invitation.ID), Outcome: audit.Succeeded, CorrelationID: "path-terminal-race", OccurredAt: terminalAt},
	}
	type terminalOutcome struct {
		accepted bool
		err      error
	}
	start := make(chan struct{})
	outcomes := make(chan terminalOutcome, 3)
	go func() {
		<-start
		_, err := repository.Accept(ctx, acceptCommand)
		outcomes <- terminalOutcome{accepted: err == nil, err: err}
	}()
	go func() {
		<-start
		_, err := repository.Cancel(ctx, cancelCommand)
		outcomes <- terminalOutcome{err: err}
	}()
	go func() {
		<-start
		_, err := repository.Reject(ctx, rejectCommand)
		outcomes <- terminalOutcome{err: err}
	}()
	close(start)
	var successes, denials int
	for range 3 {
		outcome := <-outcomes
		if outcome.err == nil {
			successes++
		} else if errors.Is(outcome.err, domain.ErrInvitationUnavailable) {
			denials++
		} else {
			t.Fatalf("terminal race error = %v", outcome.err)
		}
	}
	if successes != 1 || denials != 2 {
		t.Fatalf("terminal race successes=%d denials=%d, want one winner", successes, denials)
	}
	var row invitationModel
	if err := migrationDB.Where("id = ?", invitation.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	terminalCount := 0
	for _, value := range []*time.Time{row.AcceptedAt, row.RejectedAt, row.CanceledAt} {
		if value != nil {
			terminalCount++
		}
	}
	if terminalCount != 1 {
		t.Fatalf("terminal row accepted=%v rejected=%v canceled=%v, want exactly one", row.AcceptedAt, row.RejectedAt, row.CanceledAt)
	}
	var memberships, authChanges, acceptedAudits, rejectedAudits, canceledAudits, reservations int64
	if err := migrationDB.Model(&membershipModel{}).Where("path_id = ? AND user_id = ?", pathID, recipient).Count(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("authorization_outbox_models").Where("id = ?", acceptCommand.AuthorizationChange.ID).Count(&authChanges).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&auditModel{}).Where("action = ? AND target_id = ?", audit.PathInvitationAccepted, invitation.ID).Count(&acceptedAudits).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&auditModel{}).Where("action = ? AND target_id = ?", audit.PathInvitationRejected, invitation.ID).Count(&rejectedAudits).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&auditModel{}).Where("action = ? AND target_id = ?", audit.PathInvitationCanceled, invitation.ID).Count(&canceledAudits).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&idempotencyModel{}).Where("resource_id = ? AND operation IN ?", invitation.ID, []string{application.AcceptInvitationOperation, application.RejectInvitationOperation, application.CancelInvitationOperation}).Count(&reservations).Error; err != nil {
		t.Fatal(err)
	}
	if acceptedAudits+rejectedAudits+canceledAudits != 1 || reservations != 1 {
		t.Fatalf("terminal effects acceptedAudits=%d rejectedAudits=%d canceledAudits=%d reservations=%d", acceptedAudits, rejectedAudits, canceledAudits, reservations)
	}
	if row.AcceptedAt != nil && (memberships != 1 || authChanges != 1 || acceptedAudits != 1) {
		t.Fatalf("accept winner memberships=%d auth=%d audits=%d", memberships, authChanges, acceptedAudits)
	}
	if row.RejectedAt != nil && (memberships != 0 || authChanges != 0 || rejectedAudits != 1) {
		t.Fatalf("reject winner memberships=%d auth=%d audits=%d", memberships, authChanges, rejectedAudits)
	}
	if row.CanceledAt != nil && (memberships != 0 || authChanges != 0 || canceledAudits != 1) {
		t.Fatalf("cancel winner memberships=%d auth=%d audits=%d", memberships, authChanges, canceledAudits)
	}
}
