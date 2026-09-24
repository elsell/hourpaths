package pathstore

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPostgresOwnershipTransferLifecycleIsActorScopedReplayableAndRoleAtomic(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	creator, recipient, administrator, supporter, stranger := "transfer-owner", "transfer-recipient", "transfer-admin", "transfer-supporter", "transfer-stranger"
	pathID := "transfer-path"
	users := []string{creator, recipient, administrator, supporter, stranger}
	cleanupOwnershipTransferFixture(t, migrationDB, users, []string{pathID})
	t.Cleanup(func() { cleanupOwnershipTransferFixture(t, migrationDB, users, []string{pathID}) })
	for _, user := range users {
		seedInvitationUser(t, migrationDB, user, strings.ReplaceAll(user, "-", "_")+".name", "public", now)
	}
	path := domain.Entity{ID: domain.ID(pathID), OwnerUserID: creator, Attributes: domain.Attributes{Name: "Transfer", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	seedGoalUpdatePath(t, migrationDB, path, creator)
	for _, membership := range []membershipModel{
		{PathID: pathID, UserID: recipient, Role: "participant"},
		{PathID: pathID, UserID: administrator, Role: "administrator"},
		{PathID: pathID, UserID: supporter, Role: "supporter"},
	} {
		if err := migrationDB.Create(&membership).Error; err != nil {
			t.Fatal(err)
		}
	}
	activity := map[string]any{"id": "transfer-activity", "path_id": pathID, "participant_id": recipient, "started_at": now.Add(-10 * time.Minute), "ended_at": now.Add(-5 * time.Minute), "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now}
	if err := migrationDB.Table("recorded_activity_models").Create(activity).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewOwnershipTransferRepository(runtimeDB)
	firstCandidatePage, err := repository.ListCandidates(ctx, application.OwnershipTransferQuery{ActorUserID: creator, PathID: path.ID}, application.OwnershipTransferCandidatePageRequest{Limit: 1, Snapshot: now.Add(time.Minute)})
	if err != nil || len(firstCandidatePage.Items) != 1 || firstCandidatePage.Items[0].UserID != administrator || !firstCandidatePage.HasMore {
		t.Fatalf("first ListCandidates() = %+v, %v", firstCandidatePage, err)
	}
	candidates, err := repository.ListCandidates(ctx, application.OwnershipTransferQuery{ActorUserID: creator, PathID: path.ID}, application.OwnershipTransferCandidatePageRequest{AfterUserID: administrator, AfterCreated: now, Limit: 10, Snapshot: now.Add(time.Minute)})
	if err != nil || len(candidates.Items) != 1 || candidates.Items[0].UserID != recipient || candidates.Items[0].Administrator || candidates.HasMore {
		t.Fatalf("ListCandidates() = %+v, %v", candidates, err)
	}
	if _, err := repository.ListCandidates(ctx, application.OwnershipTransferQuery{ActorUserID: stranger, PathID: path.ID}, application.OwnershipTransferCandidatePageRequest{Limit: 10, Snapshot: now.Add(time.Minute)}); !errors.Is(err, domain.ErrOwnershipTransferUnavailable) {
		t.Fatalf("wrong-actor ListCandidates error = %v", err)
	}

	first := mustOwnershipTransfer(t, "transfer-first", path.ID, creator, recipient, now, now.Add(time.Hour))
	created, err := repository.Initiate(ctx, initiateTransferCommand(first, "transfer-init-1", bytes.Repeat([]byte{1}, 32), "transfer-created-1"))
	if err != nil || created.Transfer != first || created.Replayed {
		t.Fatalf("Initiate() = %+v, %v", created, err)
	}
	for _, actor := range []string{creator, recipient} {
		decision, err := repository.GetPending(ctx, application.OwnershipTransferQuery{ActorUserID: actor, PathID: path.ID})
		if err != nil || decision.Transfer != first || decision.Path != path || !decision.RecipientIsParticipant {
			t.Fatalf("GetPending(%s) = %+v, %v", actor, decision, err)
		}
	}
	if _, err := repository.GetPending(ctx, application.OwnershipTransferQuery{ActorUserID: stranger, TransferID: first.ID}); !errors.Is(err, domain.ErrOwnershipTransferUnavailable) {
		t.Fatalf("opaque GetPending error = %v", err)
	}
	replayCandidate := mustOwnershipTransfer(t, "transfer-replay-ignored", path.ID, creator, recipient, now, now.Add(time.Hour))
	replay, err := repository.Initiate(ctx, initiateTransferCommand(replayCandidate, "transfer-init-1", bytes.Repeat([]byte{1}, 32), "ignored-audit"))
	if err != nil || !replay.Replayed || replay.Transfer != first {
		t.Fatalf("Initiate replay = %+v, %v", replay, err)
	}
	secondPending := mustOwnershipTransfer(t, "transfer-blocked", path.ID, creator, administrator, now.Add(time.Minute), now.Add(time.Hour))
	if _, err := repository.Initiate(ctx, initiateTransferCommand(secondPending, "transfer-init-2", bytes.Repeat([]byte{2}, 32), "transfer-created-blocked")); !errors.Is(err, domain.ErrOwnershipTransferUnavailable) {
		t.Fatalf("second pending error = %v", err)
	}

	declined, _ := first.Decline(recipient, now.Add(2*time.Minute))
	declineResult, err := repository.Decline(ctx, terminalTransferCommand(declined, recipient, application.DeclineOwnershipTransferOperation, "transfer-decline-1", 3, audit.PathOwnershipTransferDeclined, "transfer-declined-1"))
	if err != nil || declineResult.Transfer != declined || declineResult.Replayed {
		t.Fatalf("Decline() = %+v, %v", declineResult, err)
	}
	assertOwnershipRoles(t, migrationDB, pathID, creator, map[string]string{creator: "participant", recipient: "participant", administrator: "administrator", supporter: "supporter"})

	cancelable := mustOwnershipTransfer(t, "transfer-cancelable", path.ID, creator, administrator, now.Add(3*time.Minute), now.Add(2*time.Hour))
	if _, err := repository.Initiate(ctx, initiateTransferCommand(cancelable, "transfer-init-3", bytes.Repeat([]byte{4}, 32), "transfer-created-cancel")); err != nil {
		t.Fatal(err)
	}
	canceled, _ := cancelable.Cancel(creator, now.Add(4*time.Minute))
	cancelResult, err := repository.Cancel(ctx, cancelTransferCommand(canceled, "transfer-cancel-1", 5, "transfer-canceled-1"))
	if err != nil || cancelResult.Transfer != canceled || cancelResult.Replayed {
		t.Fatalf("Cancel() = %+v, %v", cancelResult, err)
	}
	assertOwnershipRoles(t, migrationDB, pathID, creator, map[string]string{creator: "participant", recipient: "participant", administrator: "administrator", supporter: "supporter"})

	second := mustOwnershipTransfer(t, "transfer-second", path.ID, creator, recipient, now.Add(5*time.Minute), now.Add(2*time.Hour))
	if _, err := repository.Initiate(ctx, initiateTransferCommand(second, "transfer-init-4", bytes.Repeat([]byte{6}, 32), "transfer-created-2")); err != nil {
		t.Fatal(err)
	}
	accepted, _ := second.Accept(recipient, now.Add(6*time.Minute))
	acceptResult, err := repository.Accept(ctx, acceptTransferCommand(accepted, "transfer-accept-1", 7, "transfer-accepted-1"))
	if err != nil || acceptResult.Transfer != accepted || acceptResult.Path.OwnerUserID != recipient || acceptResult.Replayed {
		t.Fatalf("Accept() = %+v, %v", acceptResult, err)
	}
	assertOwnershipTransferRelationshipUpdates(t, acceptResult.RelationshipUpdates, accepted)
	assertOwnershipTransferAuthorizationBatch(t, acceptResult.AuthorizationBatch, accepted)
	assertOwnershipRoles(t, migrationDB, pathID, recipient, map[string]string{creator: "administrator", recipient: "participant", administrator: "administrator", supporter: "supporter"})
	var activityCount int64
	if err := migrationDB.Table("recorded_activity_models").Where("id = ? AND path_id = ? AND participant_id = ?", "transfer-activity", pathID, recipient).Count(&activityCount).Error; err != nil || activityCount != 1 {
		t.Fatalf("activity count=%d error=%v", activityCount, err)
	}
	acceptReplay, err := repository.Accept(ctx, acceptTransferCommand(accepted, "transfer-accept-1", 7, "ignored-accept-audit"))
	if err != nil || !acceptReplay.Replayed || acceptReplay.Transfer != accepted || acceptReplay.Path.OwnerUserID != recipient {
		t.Fatalf("Accept replay = %+v, %v", acceptReplay, err)
	}
	assertOwnershipTransferRelationshipUpdates(t, acceptReplay.RelationshipUpdates, accepted)
	assertOwnershipTransferAuthorizationBatch(t, acceptReplay.AuthorizationBatch, accepted)
}

func assertOwnershipTransferAuthorizationBatch(t *testing.T, batch ports.AuthorizationBatch, transfer domain.OwnershipTransfer) {
	t.Helper()
	if batch.ID != "ownership-transfer-"+string(transfer.ID) || batch.TransferID != string(transfer.ID) ||
		batch.ResourceType != "path" || batch.ResourceID != string(transfer.PathID) ||
		batch.OwnerUserID != transfer.RecipientUserID || batch.ActorUserID != transfer.RecipientUserID {
		t.Fatalf("authorization batch = %+v", batch)
	}
	assertOwnershipTransferRelationshipUpdates(t, batch.Updates, transfer)
}

func assertOwnershipTransferRelationshipUpdates(t *testing.T, actual []ports.RelationshipUpdate, transfer domain.OwnershipTransfer) {
	t.Helper()
	expected := ownershipTransferRelationshipUpdates(transfer)
	if len(actual) != len(expected) {
		t.Fatalf("relationship updates count = %d, want %d: %+v", len(actual), len(expected), actual)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("relationship update %d = %+v, want %+v", index, actual[index], expected[index])
		}
	}
}

func TestPostgresOwnershipTransferExpiresStaleAndAllowsOnlyOneConcurrentTerminalAction(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	creator, recipient := "transfer-race-owner", "transfer-race-recipient"
	pathID := "transfer-race-path"
	users := []string{creator, recipient}
	cleanupOwnershipTransferFixture(t, migrationDB, users, []string{pathID})
	t.Cleanup(func() { cleanupOwnershipTransferFixture(t, migrationDB, users, []string{pathID}) })
	for _, user := range users {
		seedInvitationUser(t, migrationDB, user, strings.ReplaceAll(user, "-", "_")+".name", "public", now)
	}
	path := domain.Entity{ID: domain.ID(pathID), OwnerUserID: creator, Attributes: domain.Attributes{Name: "Race", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	seedGoalUpdatePath(t, migrationDB, path, creator)
	if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: recipient, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewOwnershipTransferRepository(runtimeDB)
	stale := mustOwnershipTransfer(t, "transfer-stale", path.ID, creator, recipient, now.Add(-2*time.Hour), now.Add(-time.Hour))
	if err := migrationDB.Create(ownershipTransferToModel(stale)).Error; err != nil {
		t.Fatal(err)
	}
	fresh := mustOwnershipTransfer(t, "transfer-race", path.ID, creator, recipient, now, now.Add(time.Hour))
	if _, err := repository.Initiate(ctx, initiateTransferCommand(fresh, "transfer-race-init", bytes.Repeat([]byte{6}, 32), "transfer-race-created")); err != nil {
		t.Fatal(err)
	}
	var staleExpired *time.Time
	if err := migrationDB.Model(&ownershipTransferModel{}).Select("expired_at").Where("id = ?", stale.ID).Scan(&staleExpired).Error; err != nil || staleExpired == nil || !staleExpired.Equal(fresh.CreatedAt) {
		t.Fatalf("expired_at=%v error=%v", staleExpired, err)
	}
	accepted, _ := fresh.Accept(recipient, now.Add(time.Minute))
	declined, _ := fresh.Decline(recipient, now.Add(time.Minute))
	type outcome struct {
		accepted bool
		err      error
	}
	start, outcomes := make(chan struct{}), make(chan outcome, 2)
	go func() {
		<-start
		_, err := repository.Accept(ctx, acceptTransferCommand(accepted, "transfer-race-accept", 7, "transfer-race-accepted"))
		outcomes <- outcome{accepted: true, err: err}
	}()
	go func() {
		<-start
		_, err := repository.Decline(ctx, terminalTransferCommand(declined, recipient, application.DeclineOwnershipTransferOperation, "transfer-race-decline", 8, audit.PathOwnershipTransferDeclined, "transfer-race-declined"))
		outcomes <- outcome{err: err}
	}()
	close(start)
	wins := 0
	for range 2 {
		if result := <-outcomes; result.err == nil {
			wins++
		} else if !errors.Is(result.err, domain.ErrOwnershipTransferUnavailable) {
			t.Fatalf("terminal race error = %v", result.err)
		}
	}
	if wins != 1 {
		t.Fatalf("terminal winners=%d, want 1", wins)
	}
	var row ownershipTransferModel
	if err := migrationDB.Where("id = ?", fresh.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if (row.AcceptedAt == nil) == (row.DeclinedAt == nil) {
		t.Fatalf("terminal row = %+v", row)
	}
}

func mustOwnershipTransfer(t *testing.T, id string, pathID domain.ID, creator, recipient string, createdAt, expiresAt time.Time) domain.OwnershipTransfer {
	t.Helper()
	transfer, err := domain.NewOwnershipTransfer(domain.OwnershipTransferID(id), pathID, creator, recipient, createdAt, expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	return transfer
}

func initiateTransferCommand(transfer domain.OwnershipTransfer, key string, hash []byte, auditID string) application.InitiateOwnershipTransferCommand {
	return application.InitiateOwnershipTransferCommand{Transfer: transfer, ExpectedCreatorUserID: transfer.InitiatorUserID, RequireActivePath: true, RequireRecipientParticipant: true, RequireNoPendingForPath: true, Notification: transferNotification(auditID+"-notification", transfer, application.NotificationPathOwnershipTransferReceived), Idempotency: ports.Idempotency{PrincipalID: transfer.InitiatorUserID, Operation: application.InitiateOwnershipTransferOperation, Key: key, RequestHash: hash}, Audit: transferAudit(auditID, transfer.InitiatorUserID, transfer.InitiatorUserID, audit.PathOwnershipTransferCreated, transfer.ID, transfer.CreatedAt)}
}

func terminalTransferCommand(transfer domain.OwnershipTransfer, actor, operation, key string, hashByte byte, action audit.Action, auditID string) application.DeclineOwnershipTransferCommand {
	return application.DeclineOwnershipTransferCommand{Transfer: transfer, Notification: transferNotification(auditID+"-notification", transfer, application.NotificationPathOwnershipTransferDeclined), Idempotency: ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: bytes.Repeat([]byte{hashByte}, 32)}, Audit: transferAudit(auditID, transfer.InitiatorUserID, actor, action, transfer.ID, transfer.DeclinedAt)}
}

func acceptTransferCommand(transfer domain.OwnershipTransfer, key string, hashByte byte, auditID string) application.AcceptOwnershipTransferCommand {
	return application.AcceptOwnershipTransferCommand{Transfer: transfer, ExpectedCreatorUserID: transfer.InitiatorUserID, RequireActivePath: true, RequireRecipientParticipant: true, PromoteRecipientToCreator: true, RetainFormerCreatorAsAdministrator: true, PreserveMembershipAndActivity: true, Notification: transferNotification(auditID+"-notification", transfer, application.NotificationPathOwnershipTransferAccepted), Idempotency: ports.Idempotency{PrincipalID: transfer.RecipientUserID, Operation: application.AcceptOwnershipTransferOperation, Key: key, RequestHash: bytes.Repeat([]byte{hashByte}, 32)}, Audit: transferAudit(auditID, transfer.InitiatorUserID, transfer.RecipientUserID, audit.PathOwnershipTransferAccepted, transfer.ID, transfer.AcceptedAt)}
}

func cancelTransferCommand(transfer domain.OwnershipTransfer, key string, hashByte byte, auditID string) application.CancelOwnershipTransferCommand {
	return application.CancelOwnershipTransferCommand{Transfer: transfer, Notification: transferNotification(auditID+"-notification", transfer, application.NotificationPathOwnershipTransferCanceled), Idempotency: ports.Idempotency{PrincipalID: transfer.InitiatorUserID, Operation: application.CancelOwnershipTransferOperation, Key: key, RequestHash: bytes.Repeat([]byte{hashByte}, 32)}, Audit: transferAudit(auditID, transfer.InitiatorUserID, transfer.InitiatorUserID, audit.PathOwnershipTransferCanceled, transfer.ID, transfer.CanceledAt)}
}

func transferNotification(id string, transfer domain.OwnershipTransfer, kind application.InvitationNotificationKind) application.OwnershipTransferNotification {
	n := application.OwnershipTransferNotification{ID: id, PathID: transfer.PathID, OwnershipTransferID: transfer.ID, Kind: kind, Presentation: application.NotificationInformational}
	switch kind {
	case application.NotificationPathOwnershipTransferReceived:
		n.RecipientUserID, n.ActorUserID, n.Presentation, n.PushRequested, n.CreatedAt = transfer.RecipientUserID, transfer.InitiatorUserID, application.NotificationActionable, true, transfer.CreatedAt
	case application.NotificationPathOwnershipTransferAccepted:
		n.RecipientUserID, n.ActorUserID, n.PushRequested, n.CreatedAt = transfer.InitiatorUserID, transfer.RecipientUserID, true, transfer.AcceptedAt
	case application.NotificationPathOwnershipTransferDeclined:
		n.RecipientUserID, n.ActorUserID, n.CreatedAt = transfer.InitiatorUserID, transfer.RecipientUserID, transfer.DeclinedAt
	case application.NotificationPathOwnershipTransferCanceled:
		n.RecipientUserID, n.ActorUserID, n.CreatedAt = transfer.RecipientUserID, transfer.InitiatorUserID, transfer.CanceledAt
	}
	return n
}

func transferAudit(id, owner, actor string, action audit.Action, transferID domain.OwnershipTransferID, at time.Time) audit.Event {
	return audit.Event{ID: id, OwnerUserID: owner, ActorUserID: actor, Action: action, TargetType: "path_ownership_transfer", TargetID: string(transferID), Outcome: audit.Succeeded, CorrelationID: id, OccurredAt: at}
}

func assertOwnershipRoles(t *testing.T, db *gorm.DB, pathID, owner string, roles map[string]string) {
	t.Helper()
	var row model
	if err := db.Where("id = ?", pathID).First(&row).Error; err != nil || row.OwnerUserID != owner {
		t.Fatalf("path owner=%q error=%v, want %q", row.OwnerUserID, err, owner)
	}
	var memberships []membershipModel
	if err := db.Where("path_id = ?", pathID).Order("user_id").Find(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if len(memberships) != len(roles) {
		t.Fatalf("memberships=%+v", memberships)
	}
	for _, membership := range memberships {
		if roles[membership.UserID] != membership.Role {
			t.Fatalf("membership=%+v want role %q", membership, roles[membership.UserID])
		}
	}
}

func cleanupOwnershipTransferFixture(t *testing.T, db *gorm.DB, userIDs, pathIDs []string) {
	t.Helper()
	_ = db.Table("recorded_activity_models").Where("path_id IN ?", pathIDs).Delete(map[string]any{}).Error
	_ = db.Table("authorization_batch_outbox_models").Where("resource_type = ? AND resource_id IN ?", "path", pathIDs).Delete(map[string]any{}).Error
	_ = db.Table("path_ownership_transfer_models").Where("path_id IN ?", pathIDs).Delete(map[string]any{}).Error
	cleanupGoalUpdateFixture(t, db, userIDs, pathIDs)
}
