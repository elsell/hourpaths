package gormstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresPushClaimSuppressesBlockedActorRecipientDeliveryPermanently(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtimeStore, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, runtimeStore)
	closeSocialProfileTestStore(t, migrationStore)
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	recipient := socialRelationshipTestUser(t, migrationStore.DB, "blockedpushrecipient", identity.ProfileVisibilityPublic, now)
	actor := socialRelationshipTestUser(t, migrationStore.DB, "blockedpushactor", identity.ProfileVisibilityPublic, now)
	installationID, notificationID := "blocked-push-installation-"+newTestID(), "blocked-push-notification-"+newTestID()
	t.Cleanup(func() {
		migrationStore.DB.Table("notification_push_outbox_models").Where("notification_id = ?", notificationID).Delete(map[string]any{})
		migrationStore.DB.Table("notification_models").Where("id = ?", notificationID).Delete(map[string]any{})
		migrationStore.DB.Table("push_installation_models").Where("id = ?", installationID).Delete(map[string]any{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{recipient.ID, actor.ID}).Delete(&struct{ ID string }{})
	})
	repository, err := NewPushRepository(runtimeStore.DB, bytes.Repeat([]byte{0x62}, 32))
	if err != nil {
		t.Fatal(err)
	}
	const token = "ExponentPushToken[blocked-pair]"
	if err := repository.UpsertPushInstallation(context.Background(), ports.PushInstallation{
		ID: installationID, OwnerUserID: recipient.ID, Provider: "expo", Platform: "ios",
		Locale: "en", Token: token, CreatedAt: now, UpdatedAt: now,
	}, audit.Event{ID: newTestID(), OwnerUserID: recipient.ID, ActorUserID: recipient.ID, Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	var installation pushInstallationModel
	if err := migrationStore.DB.Where("id = ?", installationID).Take(&installation).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("notification_models").Create(map[string]any{
		"id": notificationID, "recipient_user_id": recipient.ID, "actor_user_id": actor.ID,
		"follow_subject_user_id": actor.ID, "kind": "new_follower", "presentation_class": "informational",
		"channel": "following", "created_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("notification_push_outbox_models").Create(map[string]any{"notification_id": notificationID, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("notification_push_delivery_models").Create(map[string]any{
		"notification_id": notificationID, "installation_id": installationID, "recipient_user_id": recipient.ID,
		"provider": installation.Provider, "platform": installation.Platform, "locale": installation.Locale,
		"token_ciphertext": installation.TokenCiphertext, "token_nonce": installation.TokenNonce, "token_hash": installation.TokenHash,
		"available_at": now, "created_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("block_models").Create(&socialBlockTestModel{BlockerUserID: actor.ID, BlockedUserID: recipient.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	claimed, err := repository.ClaimPushDeliveries(context.Background(), "blocked-pair-worker", time.Minute, 10)
	if err != nil || len(claimed) != 0 {
		t.Fatalf("blocked-pair claimed=%+v err=%v", claimed, err)
	}
	var delivery struct {
		SuppressedAt                           *time.Time
		FailureCode                            string
		TokenCiphertext, TokenNonce, TokenHash []byte
	}
	if err := migrationStore.DB.Table("notification_push_delivery_models").Where("notification_id = ? AND installation_id = ?", notificationID, installationID).Take(&delivery).Error; err != nil {
		t.Fatal(err)
	}
	if delivery.SuppressedAt == nil || delivery.FailureCode != "blocked_relationship" || delivery.TokenCiphertext != nil || delivery.TokenNonce != nil || delivery.TokenHash != nil {
		t.Fatalf("blocked-pair delivery=%+v", delivery)
	}
	var outbox struct {
		SuppressedAt *time.Time
		FailureCode  string
	}
	if err := migrationStore.DB.Table("notification_push_outbox_models").Where("notification_id = ?", notificationID).Take(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if outbox.SuppressedAt == nil || outbox.FailureCode != "all_installations_suppressed" {
		t.Fatalf("blocked-pair outbox=%+v", outbox)
	}
	if err := migrationStore.DB.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", actor.ID, recipient.ID).Delete(&socialBlockTestModel{}).Error; err != nil {
		t.Fatal(err)
	}
	claimed, err = repository.ClaimPushDeliveries(context.Background(), "unblocked-pair-worker", time.Minute, 10)
	if err != nil || len(claimed) != 0 {
		t.Fatalf("suppressed delivery restored after unblock: claimed=%+v err=%v", claimed, err)
	}
}

func TestPostgresPushHandoffWaitsForPairLockAndSkipsProviderWhenBlockCommitsFirst(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtimeStore, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, runtimeStore)
	closeSocialProfileTestStore(t, migrationStore)
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	recipient := socialRelationshipTestUser(t, migrationStore.DB, "handoffrecipient", identity.ProfileVisibilityPublic, now)
	actor := socialRelationshipTestUser(t, migrationStore.DB, "handoffactor", identity.ProfileVisibilityPublic, now)
	installationID, notificationID := "handoff-installation-"+newTestID(), "handoff-notification-"+newTestID()
	t.Cleanup(func() {
		migrationStore.DB.Table("notification_push_outbox_models").Where("notification_id = ?", notificationID).Delete(map[string]any{})
		migrationStore.DB.Table("notification_models").Where("id = ?", notificationID).Delete(map[string]any{})
		migrationStore.DB.Table("push_installation_models").Where("id = ?", installationID).Delete(map[string]any{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{recipient.ID, actor.ID}).Delete(&struct{ ID string }{})
	})
	repository, err := NewPushRepository(runtimeStore.DB, bytes.Repeat([]byte{0x63}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertPushInstallation(context.Background(), ports.PushInstallation{
		ID: installationID, OwnerUserID: recipient.ID, Provider: "expo", Platform: "ios",
		Locale: "en", Token: "ExponentPushToken[pair-lock]", CreatedAt: now, UpdatedAt: now,
	}, audit.Event{ID: newTestID(), OwnerUserID: recipient.ID, ActorUserID: recipient.ID, Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	var installation pushInstallationModel
	if err := migrationStore.DB.Where("id = ?", installationID).Take(&installation).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("notification_models").Create(map[string]any{
		"id": notificationID, "recipient_user_id": recipient.ID, "actor_user_id": actor.ID,
		"follow_subject_user_id": actor.ID, "kind": "new_follower", "presentation_class": "informational",
		"channel": "following", "created_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("notification_push_outbox_models").Create(map[string]any{"notification_id": notificationID, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("notification_push_delivery_models").Create(map[string]any{
		"notification_id": notificationID, "installation_id": installationID, "recipient_user_id": recipient.ID,
		"provider": installation.Provider, "platform": installation.Platform, "locale": installation.Locale,
		"token_ciphertext": installation.TokenCiphertext, "token_nonce": installation.TokenNonce, "token_hash": installation.TokenHash,
		"available_at": now, "created_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	claimed, err := repository.ClaimPushDeliveries(context.Background(), "pair-lock-worker", time.Minute, 1)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}

	blockTx := migrationStore.DB.Begin()
	if blockTx.Error != nil {
		t.Fatal(blockTx.Error)
	}
	defer blockTx.Rollback()
	if err := lockSocialPair(blockTx, actor.ID, recipient.ID); err != nil {
		t.Fatal(err)
	}
	if err := blockTx.Create(&socialBlockModel{BlockerUserID: actor.ID, BlockedUserID: recipient.ID, CreatedAt: now.Add(time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	type handoffResult struct {
		ticket    ports.PushTicket
		handedOff bool
		err       error
	}
	providerCalled := make(chan struct{}, 1)
	done := make(chan handoffResult, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		ticket, handedOff, err := repository.HandoffPushDelivery(context.Background(), "pair-lock-worker", notificationID, installationID, func() ports.PushTicket {
			providerCalled <- struct{}{}
			return ports.PushTicket{State: ports.PushDelivered}
		})
		done <- handoffResult{ticket: ticket, handedOff: handedOff, err: err}
	}()
	<-started

	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case result := <-done:
			t.Fatalf("handoff escaped block transaction before commit: %+v provider_called=%v", result, len(providerCalled) != 0)
		default:
		}
		var lockState struct{ Waiting bool }
		if err := blockTx.Table("pg_locks AS held").
			Select("COUNT(*) > 0 AS waiting").
			Joins(`JOIN pg_locks waiter
  ON waiter.locktype = held.locktype
 AND waiter.database IS NOT DISTINCT FROM held.database
 AND waiter.classid IS NOT DISTINCT FROM held.classid
 AND waiter.objid IS NOT DISTINCT FROM held.objid
 AND waiter.objsubid IS NOT DISTINCT FROM held.objsubid`).
			Where("held.pid = pg_backend_pid() AND held.locktype = 'advisory' AND held.granted AND NOT waiter.granted").
			Scan(&lockState).Error; err != nil {
			t.Fatal(err)
		}
		if lockState.Waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("handoff did not contend on the block transaction's pair lock")
		}
	}
	if err := blockTx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	var result handoffResult
	select {
	case result = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handoff did not resume after the winning block committed")
	}
	if result.err != nil || result.handedOff || result.ticket != (ports.PushTicket{}) || len(providerCalled) != 0 {
		t.Fatalf("handoff after block commit=%+v provider_called=%v", result, len(providerCalled) != 0)
	}
}

func TestPushRepositoryPersistsEncryptedSnapshotClaimsAndSuppressesOnDelete(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tx := store.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	ownerID, installationID := newTestID(), newTestID()
	// PostgreSQL CURRENT_TIMESTAMP is fixed when this rollback-scoped test
	// transaction begins. Keep every fixture transition behind that clock so
	// availability claims remain deterministic inside the outer transaction.
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	if err := tx.Create(&userModel{
		ID: ownerID, Email: ownerID + "@example.test", DisplayName: "Push owner",
		Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository, err := NewPushRepository(tx, bytes.Repeat([]byte{0x61}, 32))
	if err != nil {
		t.Fatal(err)
	}
	const plaintextToken = "ExponentPushToken[postgres-secret]"
	upsertAudit := audit.Event{
		ID: newTestID(), OwnerUserID: ownerID, ActorUserID: ownerID,
		Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID,
		Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now,
	}
	if err := repository.UpsertPushInstallation(ctx, ports.PushInstallation{
		ID: installationID, OwnerUserID: ownerID, Provider: "expo", Platform: "ios",
		Locale: "en", Token: plaintextToken, CreatedAt: now, UpdatedAt: now,
	}, upsertAudit); err != nil {
		t.Fatal(err)
	}
	var persisted pushInstallationModel
	if err := tx.Where("id = ?", installationID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(persisted.TokenCiphertext, []byte(plaintextToken)) ||
		len(persisted.TokenHash) != sha256.Size || len(persisted.TokenNonce) != repository.aead.NonceSize() {
		t.Fatal("persisted push credential is not protected")
	}
	rollbackAt := now.Add(time.Millisecond)
	rollbackAudit := upsertAudit
	rollbackAudit.OccurredAt = rollbackAt
	if err := repository.UpsertPushInstallation(ctx, ports.PushInstallation{
		ID: installationID, OwnerUserID: ownerID, Provider: "apns", Platform: "ios",
		Locale: "en", Token: plaintextToken, CreatedAt: now, UpdatedAt: rollbackAt,
	}, rollbackAudit); err == nil {
		t.Fatal("duplicate audit unexpectedly committed an installation update")
	}
	if err := tx.Where("id = ?", installationID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Provider != "expo" {
		t.Fatal("installation update survived its failed atomic audit append")
	}

	pathID, invitationID, notificationID := newTestID(), newTestID(), newTestID()
	if err := tx.Exec(`INSERT INTO path_models
		(id, owner_user_id, name, visibility, created_at, updated_at)
		VALUES (?, ?, 'Push path', 'private', ?, ?)`, pathID, ownerID, now, now).Error; err != nil {
		t.Fatal(err)
	}
	recipientID := newTestID()
	if err := tx.Create(&userModel{
		ID: recipientID, Email: recipientID + "@example.test", DisplayName: "Recipient",
		Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// The notification targets the registered owner; the separate recipient only
	// satisfies the invitation's distinct-user invariant.
	if err := tx.Exec(`INSERT INTO path_invitation_models
		(id, path_id, inviter_user_id, recipient_user_id, offered_role, created_at)
		VALUES (?, ?, ?, ?, 'participant', ?)`,
		invitationID, pathID, recipientID, ownerID, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`INSERT INTO notification_models
		(id, recipient_user_id, actor_user_id, path_id, path_invitation_id,
		 kind, presentation_class, channel, offered_role, created_at)
		VALUES (?, ?, ?, ?, ?, 'path_invitation_received', 'actionable',
		        'path_access', 'participant', ?)`,
		notificationID, ownerID, recipientID, pathID, invitationID, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`INSERT INTO notification_push_delivery_models
		(notification_id, installation_id, recipient_user_id, provider, platform, locale,
		 token_ciphertext, token_nonce, token_hash, available_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		notificationID, installationID, ownerID, persisted.Provider, persisted.Platform, persisted.Locale,
		persisted.TokenCiphertext, persisted.TokenNonce, persisted.TokenHash, now, now).Error; err != nil {
		t.Fatal(err)
	}
	claimed, err := repository.ClaimPushDeliveries(ctx, "worker-1", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].Token != plaintextToken || claimed[0].Attempts != 1 ||
		claimed[0].LockedBy != "worker-1" {
		t.Fatalf("claimed deliveries = %#v", claimed)
	}
	failedTerminalAt := now.Add(time.Millisecond)
	failedTerminalAudit := audit.Event{
		ID: upsertAudit.ID, OwnerUserID: ownerID, ActorUserID: ownerID,
		Action: audit.ResourceUpdated, TargetType: "notification_push_delivery",
		TargetID: notificationID + "/" + installationID, Outcome: audit.Succeeded,
		CorrelationID: newTestID(), OccurredAt: failedTerminalAt,
	}
	if err := repository.TransitionPushDelivery(ctx, "worker-1", ports.PushDeliveryTransition{
		NotificationID: notificationID, InstallationID: installationID,
		Outcome: ports.PushDeliveryDelivered, OccurredAt: failedTerminalAt,
	}, failedTerminalAudit); err == nil {
		t.Fatal("terminal delivery committed despite failed atomic audit append")
	}
	if err := repository.TransitionPushDelivery(ctx, "worker-1", ports.PushDeliveryTransition{
		NotificationID: notificationID, InstallationID: installationID,
		Outcome: ports.PushDeliveryRetry, OccurredAt: now.Add(time.Millisecond),
		AvailableAt: now, FailureCode: "temporary_provider_failure",
	}, audit.Event{}); err != nil {
		t.Fatal(err)
	}
	retried, err := repository.ClaimPushDeliveries(ctx, "worker-2", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(retried) != 1 || retried[0].Attempts != 2 || retried[0].LockedBy != "worker-2" {
		t.Fatalf("retry was not claimable: %#v", retried)
	}
	if err := tx.Exec(`UPDATE notification_push_delivery_models
		SET locked_until = CURRENT_TIMESTAMP - INTERVAL '1 second'
		WHERE notification_id = ? AND installation_id = ?`, notificationID, installationID).Error; err != nil {
		t.Fatal(err)
	}
	reclaimed, err := repository.ClaimPushDeliveries(ctx, "worker-3", time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(reclaimed) != 1 || reclaimed[0].Attempts != 3 || reclaimed[0].LockedBy != "worker-3" {
		t.Fatalf("expired lease was not recovered: %#v", reclaimed)
	}

	moveAt := now.Add(time.Second)
	newOwnerID := newTestID()
	if err := tx.Create(&userModel{
		ID: newOwnerID, Email: newOwnerID + "@example.test", DisplayName: "New owner",
		Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	moveAudit := audit.Event{
		ID: newTestID(), OwnerUserID: newOwnerID, ActorUserID: newOwnerID,
		Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID,
		Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: moveAt,
	}
	if err := repository.UpsertPushInstallation(ctx, ports.PushInstallation{
		ID: installationID, OwnerUserID: newOwnerID, Provider: "expo", Platform: "ios",
		Locale: "en", Token: plaintextToken, CreatedAt: now, UpdatedAt: moveAt,
	}, moveAudit); err != nil {
		t.Fatal(err)
	}
	var moved pushInstallationModel
	if err := tx.Where("id = ?", installationID).First(&moved).Error; err != nil || moved.OwnerUserID != newOwnerID {
		t.Fatalf("exact-token ownership move failed: owner=%q err=%v", moved.OwnerUserID, err)
	}

	backAt := moveAt.Add(time.Second)
	backAudit := audit.Event{
		ID: newTestID(), OwnerUserID: ownerID, ActorUserID: ownerID,
		Action: audit.ResourceUpdated, TargetType: "push_installation", TargetID: installationID,
		Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: backAt,
	}
	if err := repository.UpsertPushInstallation(ctx, ports.PushInstallation{
		ID: installationID, OwnerUserID: ownerID, Provider: "expo", Platform: "ios",
		Locale: "en", Token: plaintextToken, CreatedAt: now, UpdatedAt: backAt,
	}, backAudit); err != nil {
		t.Fatal(err)
	}
	if err := tx.Where("id = ?", installationID).First(&moved).Error; err != nil {
		t.Fatal(err)
	}

	disableNotificationID := newTestID()
	if err := tx.Exec(`INSERT INTO notification_models
		(id, recipient_user_id, actor_user_id, path_id, path_invitation_id,
		 kind, presentation_class, channel, offered_role, created_at)
		VALUES (?, ?, ?, ?, ?, 'path_invitation_accepted', 'informational',
		        'path_access', 'participant', ?)`,
		disableNotificationID, ownerID, recipientID, pathID, invitationID, backAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`INSERT INTO notification_push_outbox_models (notification_id, created_at) VALUES (?, ?)`,
		disableNotificationID, backAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`INSERT INTO notification_push_delivery_models
		(notification_id, installation_id, recipient_user_id, provider, platform, locale,
		 token_ciphertext, token_nonce, token_hash, available_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		disableNotificationID, installationID, ownerID, moved.Provider, moved.Platform, moved.Locale,
		moved.TokenCiphertext, moved.TokenNonce, moved.TokenHash, backAt, backAt).Error; err != nil {
		t.Fatal(err)
	}
	disableClaim, err := repository.ClaimPushDeliveries(ctx, "worker-disable", time.Minute, 10)
	if err != nil || len(disableClaim) != 1 {
		t.Fatalf("disable claim = %#v, err=%v", disableClaim, err)
	}
	disabledAt := backAt.Add(time.Second)
	disableAudit := audit.Event{
		ID: newTestID(), OwnerUserID: ownerID, ActorUserID: ownerID,
		Action: audit.ResourceDeleted, TargetType: "push_installation", TargetID: installationID,
		Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: disabledAt,
	}
	if err := repository.DisablePushInstallation(ctx, "worker-disable", disableNotificationID,
		installationID, disabledAt, "device_not_registered", disableAudit); err != nil {
		t.Fatal(err)
	}
	var suppressedAt *time.Time
	var ciphertext []byte
	if err := tx.Raw(`SELECT suppressed_at, token_ciphertext
		FROM notification_push_delivery_models
		WHERE notification_id = ? AND installation_id = ?`,
		notificationID, installationID).Row().Scan(&suppressedAt, &ciphertext); err != nil {
		t.Fatal(err)
	}
	if suppressedAt == nil || ciphertext != nil {
		t.Fatal("ownership move did not suppress and erase the old account delivery credential")
	}
	var permanentlyFailedAt *time.Time
	if err := tx.Raw(`SELECT permanently_failed_at
		FROM notification_push_delivery_models
		WHERE notification_id = ? AND installation_id = ?`,
		disableNotificationID, installationID).Row().Scan(&permanentlyFailedAt); err != nil {
		t.Fatal(err)
	}
	if permanentlyFailedAt == nil {
		t.Fatal("device-not-registered transition did not permanently fail pending installation deliveries")
	}
}
