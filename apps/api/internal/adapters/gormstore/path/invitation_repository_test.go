package pathstore

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type failingInvitationWarningPolicy struct{}

func (failingInvitationWarningPolicy) Evaluate(
	context.Context,
	application.InvitationWarningInput,
) (application.InvitationWarningDecision, error) {
	return application.InvitationWarningDecision{}, errors.New("warning policy unavailable")
}

func TestInvitationJoinRowsExposeEmbeddedPersistenceColumnsToGORM(t *testing.T) {
	tests := []struct {
		name    string
		row     any
		columns []string
	}{
		{
			name: "pending invitation",
			row:  &pendingInvitationRow{},
			columns: []string{
				"id", "path_id", "inviter_user_id", "recipient_user_id", "offered_role",
				"created_at", "accepted_at", "rejected_at", "canceled_at",
			},
		},
		{
			name: "notification",
			row:  &invitationNotificationRow{},
			columns: []string{
				"id", "recipient_user_id", "actor_user_id", "path_id", "path_invitation_id",
				"kind", "presentation_class", "channel", "offered_role", "created_at",
				"read_at", "deleted_at",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := schema.Parse(test.row, &sync.Map{}, schema.NamingStrategy{})
			if err != nil {
				t.Fatal(err)
			}
			for _, column := range test.columns {
				if parsed.LookUpField(column) == nil {
					t.Errorf("GORM schema does not expose %q", column)
				}
			}
		})
	}
}

func TestInvitationPersistenceMappingRejectsTerminalOrMalformedRows(t *testing.T) {
	now := time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC)
	valid := invitationModel{
		ID: "invitation", PathID: "path", InviterUserID: "inviter",
		RecipientUserID: "recipient", OfferedRole: string(domain.RoleSupporter), CreatedAt: now,
	}
	if invitation, err := invitationFromModel(valid); err != nil || invitation.ID != "invitation" || !invitation.Pending() {
		t.Fatalf("valid invitation mapping = %+v, %v", invitation, err)
	}
	acceptedAt := now.Add(time.Minute)
	valid.AcceptedAt = &acceptedAt
	if invitation, err := invitationFromModel(valid); err != nil || !invitation.AcceptedAt.Equal(acceptedAt) {
		t.Fatalf("accepted invitation mapping = %+v, %v", invitation, err)
	}
	valid.AcceptedAt = nil
	rejectedAt := now.Add(2 * time.Minute)
	rejectionUnreadCount := int64(3)
	valid.RejectedAt = &rejectedAt
	valid.RejectionUnreadCount = &rejectionUnreadCount
	if invitation, err := invitationFromModel(valid); err != nil || !invitation.RejectedAt.Equal(rejectedAt) {
		t.Fatalf("rejected invitation mapping = %+v, %v", invitation, err)
	}
	tests := []struct {
		name   string
		mutate func(*invitationModel)
	}{
		{name: "unknown role", mutate: func(row *invitationModel) { row.OfferedRole = "observer" }},
		{name: "accepted before created", mutate: func(row *invitationModel) {
			at := now.Add(-time.Second)
			row.AcceptedAt = &at
		}},
		{name: "accepted and rejected", mutate: func(row *invitationModel) {
			accepted := now.Add(time.Second)
			rejected := now.Add(2 * time.Second)
			row.AcceptedAt = &accepted
			row.RejectedAt = &rejected
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := invitationModel{
				ID: "invitation", PathID: "path", InviterUserID: "inviter",
				RecipientUserID: "recipient", OfferedRole: string(domain.RoleParticipant), CreatedAt: now,
			}
			test.mutate(&row)
			if invitation, err := invitationFromModel(row); err == nil || invitation != (domain.Invitation{}) {
				t.Fatalf("malformed row mapped to %+v, %v", invitation, err)
			}
		})
	}
}

func TestInvitationRepositoryRejectsMalformedCommandsBeforeDatabaseAccess(t *testing.T) {
	repository := New(nil)
	if _, err := repository.Send(context.Background(), application.SendInvitationCommand{}); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("Send error = %v, want ErrInvalidArgument", err)
	}
	if _, err := repository.ListPending(context.Background(), "", application.InvitationPageRequest{}); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("ListPending error = %v, want ErrInvalidArgument", err)
	}
	if _, err := repository.ListNotifications(context.Background(), "", application.NotificationPageRequest{}); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("ListNotifications error = %v, want ErrInvalidArgument", err)
	}
	if _, err := repository.AcceptanceDecision(context.Background(), "", "", ports.Idempotency{}); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("AcceptanceDecision error = %v, want ErrInvalidArgument", err)
	}
	if _, err := repository.Accept(context.Background(), application.AcceptInvitationCommand{}); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("Accept error = %v, want ErrInvalidArgument", err)
	}
}

func TestPostgresPathInvitationSendListAcceptAndReplayAreAtomicAndScoped(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 23, 18, 30, 0, 123_456_000, time.UTC)
	owner, recipient, stranger := "path-invite-owner", "path-invite-recipient", "path-invite-stranger"
	pathID := "path-invite-path"
	cleanupInvitationFixture(t, migrationDB, []string{owner, recipient, stranger}, []string{pathID})
	t.Cleanup(func() {
		cleanupInvitationFixture(t, migrationDB, []string{owner, recipient, stranger}, []string{pathID})
	})
	seedInvitationUser(t, migrationDB, owner, "Owner", identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, migrationDB, recipient, "Reader.One", identity.ProfileVisibilityPrivate, now)
	seedInvitationUser(t, migrationDB, stranger, "Stranger", identity.ProfileVisibilityPublic, now)
	path := domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{Name: "Reading", Visibility: "followers"},
		CreatedAt:  now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	seedGoalUpdatePath(t, migrationDB, path, owner)

	repository := New(runtimeDB)
	found, err := repository.ActiveByExactUsername(ctx, "reader.one")
	if err != nil || found.UserID != recipient || found.Username != "Reader.One" ||
		found.DisplayName != "Reader One" ||
		found.ProfileVisibility != identity.ProfileVisibilityPrivate {
		t.Fatalf("case-insensitive exact directory result = %+v, %v", found, err)
	}
	if _, err := repository.ActiveByExactUsername(ctx, "Reader"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("partial username lookup error = %v, want ErrNotFound", err)
	}

	invitation, err := domain.NewInvitation("path-invite-1", path.ID, owner, recipient, domain.RoleParticipant, now)
	if err != nil {
		t.Fatal(err)
	}
	send := sendInvitationCommand(invitation, "path-invite-notification-1", "path-invite-send-key", 1, "path-invite-send-audit", owner)
	if err := migrationDB.Table("user_models").Where("id = ?", recipient).Update("username", "Renamed.Reader").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("user_models").Where("id = ?", stranger).Update("username", "Reader.One").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Send(ctx, send); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("reassigned reviewed username Send error = %v, want opaque ErrNotFound", err)
	}
	assertInvitationReservationCount(t, migrationDB, owner, send.Idempotency.Key, 0)
	assertInvitationPersistenceCount(t, migrationDB, invitation.ID, 0)
	if err := migrationDB.Table("user_models").Where("id = ?", stranger).Update("username", "Stranger").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("user_models").Where("id = ?", recipient).Update("username", "Reader.One").Error; err != nil {
		t.Fatal(err)
	}
	sent, err := repository.Send(ctx, send)
	if err != nil || sent.Replayed || sent.Invitation != invitation {
		t.Fatalf("Send = %+v, %v; want exact invitation", sent, err)
	}
	replay, err := repository.Send(ctx, send)
	if err != nil || !replay.Replayed || replay.Invitation != invitation {
		t.Fatalf("Send replay = %+v, %v", replay, err)
	}
	conflictingSend := send
	conflictingSend.Idempotency.RequestHash = bytes.Repeat([]byte{9}, 32)
	if _, err := repository.Send(ctx, conflictingSend); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("conflicting Send replay error = %v, want ErrIdempotencyConflict", err)
	}
	assertInvitationSideEffects(t, migrationDB, invitation.ID, "path_invitation_received", recipient, 1)
	duplicate, err := domain.NewInvitation("path-invite-duplicate", path.ID, owner, recipient, domain.RoleSupporter, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	duplicateCommand := sendInvitationCommand(duplicate, "path-invite-notification-duplicate", "path-invite-send-duplicate", 4, "path-invite-send-audit-duplicate", owner)
	if _, err := repository.Send(ctx, duplicateCommand); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("duplicate pending invitation error = %v, want ErrConflict", err)
	}
	assertInvitationReservationCount(t, migrationDB, owner, duplicateCommand.Idempotency.Key, 0)

	if err := migrationDB.Table("user_models").Where("id = ?", stranger).
		Update("status", identity.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	disabled, err := domain.NewInvitation("path-invite-disabled", path.ID, owner, stranger, domain.RoleSupporter, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	disabledCommand := sendInvitationCommand(disabled, "path-invite-notification-disabled", "path-invite-send-disabled", 5, "path-invite-send-audit-disabled", owner)
	if _, err := repository.Send(ctx, disabledCommand); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("disabled recipient Send error = %v, want opaque ErrNotFound", err)
	}
	assertInvitationReservationCount(t, migrationDB, owner, disabledCommand.Idempotency.Key, 0)
	assertInvitationPersistenceCount(t, migrationDB, disabled.ID, 0)

	if err := migrationDB.Table("user_models").Where("id = ?", stranger).
		Update("status", identity.StatusActive).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Create(&membershipModel{
		PathID: pathID, UserID: stranger, Role: string(domain.RoleSupporter),
	}).Error; err != nil {
		t.Fatal(err)
	}
	currentMember, err := domain.NewInvitation("path-invite-member", path.ID, owner, stranger, domain.RoleParticipant, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	memberCommand := sendInvitationCommand(currentMember, "path-invite-notification-member", "path-invite-send-member", 6, "path-invite-send-audit-member", owner)
	if _, err := repository.Send(ctx, memberCommand); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("current-member Send error = %v, want ErrConflict", err)
	}
	assertInvitationReservationCount(t, migrationDB, owner, memberCommand.Idempotency.Key, 0)
	assertInvitationPersistenceCount(t, migrationDB, currentMember.ID, 0)

	_, err = domain.NewInvitation("path-invite-self", path.ID, owner, owner, domain.RoleParticipant, now.Add(time.Second))
	if !errors.Is(err, domain.ErrInvalidFields) {
		t.Fatalf("domain self invitation error = %v, want invalid fields", err)
	}

	page, err := repository.ListPending(ctx, recipient, application.InvitationPageRequest{Limit: 1, Snapshot: now.Add(time.Minute)})
	if err != nil || page.HasMore || len(page.Items) != 1 {
		t.Fatalf("recipient ListPending = %+v, %v", page, err)
	}
	listed := page.Items[0]
	if listed.PendingInvitation.Invitation != invitation || listed.PendingInvitation.PathName != "Reading" ||
		listed.PendingInvitation.Inviter.UserID != owner || listed.PendingInvitation.Inviter.Username != "Owner" ||
		listed.PendingInvitation.Inviter.DisplayName != "Owner" ||
		listed.WarningInput != (application.InvitationWarningInput{
			RecipientVisibility: identity.ProfileVisibilityPrivate,
			PathVisibility:      "followers",
			OfferedRole:         domain.RoleParticipant,
			HasRetainedActivity: false,
		}) {
		t.Fatalf("recipient ListPending = %+v, %v", page, err)
	}
	foreignPage, err := repository.ListPending(ctx, stranger, application.InvitationPageRequest{Limit: 10, Snapshot: now.Add(time.Minute)})
	if err != nil || len(foreignPage.Items) != 0 {
		t.Fatalf("stranger ListPending = %+v, %v", foreignPage, err)
	}
	recipientNotifications, err := repository.ListNotifications(ctx, recipient, application.NotificationPageRequest{
		Limit: 10, Snapshot: now.Add(time.Minute),
	})
	if err != nil || recipientNotifications.HasMore || len(recipientNotifications.Items) != 1 ||
		recipientNotifications.UnreadCount != 1 {
		t.Fatalf("recipient ListNotifications = %+v, %v", recipientNotifications, err)
	}
	receivedNotification := recipientNotifications.Items[0]
	if receivedNotification.ID != "path-invite-notification-1" ||
		receivedNotification.Kind != application.NotificationPathInvitationReceived ||
		receivedNotification.Presentation != application.NotificationActionable ||
		receivedNotification.Read || receivedNotification.PathID != path.ID ||
		receivedNotification.PathName != "Reading" || receivedNotification.InvitationID != invitation.ID ||
		receivedNotification.OfferedRole != domain.RoleParticipant ||
		receivedNotification.Actor.UserID != owner || receivedNotification.Actor.Username != "Owner" {
		t.Fatalf("received notification projection = %+v", receivedNotification)
	}
	resolvedReceived, err := repository.GetNotification(ctx, recipient, receivedNotification.ID)
	if err != nil || resolvedReceived != receivedNotification {
		t.Fatalf("recipient GetNotification = %+v, %v", resolvedReceived, err)
	}
	preEligibilityID := "path-invite-notification-pre-eligibility"
	if err := migrationDB.Table("notification_models").Create(map[string]any{
		"id": preEligibilityID, "recipient_user_id": stranger,
		"actor_user_id": owner, "path_id": path.ID, "path_invitation_id": invitation.ID,
		"kind": "path_invitation_accepted", "presentation_class": "informational",
		"channel": invitationNotificationChannel, "offered_role": string(domain.RoleParticipant),
		"created_at": time.Now().UTC().Add(time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetNotification(ctx, stranger, preEligibilityID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("pre-eligibility GetNotification error = %v, want opaque ErrNotFound", err)
	}
	if _, err := repository.GetNotification(ctx, stranger, receivedNotification.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("foreign GetNotification error = %v, want opaque ErrNotFound", err)
	}
	foreignNotifications, err := repository.ListNotifications(ctx, stranger, application.NotificationPageRequest{
		Limit: 10, Snapshot: now.Add(time.Minute),
	})
	if err != nil || len(foreignNotifications.Items) != 0 {
		t.Fatalf("stranger ListNotifications = %+v, %v", foreignNotifications, err)
	}
	if foreignNotifications.UnreadCount != 0 {
		t.Fatalf("stranger unread count = %d, want 0", foreignNotifications.UnreadCount)
	}
	cursorPage, err := repository.ListNotifications(ctx, recipient, application.NotificationPageRequest{
		AfterID: "older-page", AfterCreated: now.Add(-time.Second), Limit: 1,
		Snapshot: now.Add(time.Minute),
	})
	if err != nil || len(cursorPage.Items) != 0 || cursorPage.UnreadCount != 1 {
		t.Fatalf("cursor-independent unread count page = %+v, %v", cursorPage, err)
	}

	acceptIdempotency := ports.Idempotency{
		PrincipalID: recipient, Operation: application.AcceptInvitationOperation,
		Key: "path-invite-accept-key", RequestHash: bytes.Repeat([]byte{2}, 32),
	}
	if _, err := repository.AcceptanceDecision(ctx, stranger, invitation.ID, ports.Idempotency{
		PrincipalID: stranger, Operation: application.AcceptInvitationOperation,
		Key: "path-invite-stranger-key", RequestHash: bytes.Repeat([]byte{3}, 32),
	}); !errors.Is(err, domain.ErrInvitationUnavailable) {
		t.Fatalf("cross-user AcceptanceDecision error = %v, want opaque unavailable", err)
	}
	decision, err := repository.AcceptanceDecision(ctx, recipient, invitation.ID, acceptIdempotency)
	if err != nil || decision.Replay != nil || decision.Invitation != invitation ||
		decision.Path != path || decision.RecipientVisibility != identity.ProfileVisibilityPrivate ||
		decision.HasRetainedActivity {
		t.Fatalf("AcceptanceDecision = %+v, %v", decision, err)
	}
	if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{
		"id": "path-invite-foreign-activity", "path_id": pathID, "participant_id": stranger,
		"started_at": now.Add(-2 * time.Hour), "ended_at": now.Add(-time.Hour),
		"occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	decision, err = repository.AcceptanceDecision(ctx, recipient, invitation.ID, acceptIdempotency)
	if err != nil || decision.Replay != nil || decision.Invitation != invitation ||
		decision.Path != path || decision.RecipientVisibility != identity.ProfileVisibilityPrivate ||
		decision.HasRetainedActivity {
		t.Fatalf("AcceptanceDecision = %+v, %v", decision, err)
	}
	canceledContext, cancel := context.WithCancel(ctx)
	cancel()
	if retained, err := retainedActivityExists(canceledContext, runtimeDB, recipient, path.ID); err == nil || retained {
		t.Fatalf("canceled retained-activity query = %t, %v; want fail-closed error", retained, err)
	}
	if err := migrationDB.Table("recorded_activity_models").Create(map[string]any{
		"id": "path-invite-retained-activity", "path_id": pathID, "participant_id": recipient,
		"started_at": now.Add(-2 * time.Hour), "ended_at": now.Add(-time.Hour),
		"occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	decision, err = repository.AcceptanceDecision(ctx, recipient, invitation.ID, acceptIdempotency)
	if err != nil || !decision.HasRetainedActivity {
		t.Fatalf("AcceptanceDecision retained activity = %+v, %v; want exact recipient and Path match", decision, err)
	}
	page, err = repository.ListPending(ctx, recipient, application.InvitationPageRequest{Limit: 1, Snapshot: now.Add(time.Minute)})
	if err != nil || len(page.Items) != 1 || !page.Items[0].WarningInput.HasRetainedActivity {
		t.Fatalf("recipient retained-activity warning = %+v, %v", page, err)
	}

	accepted, err := invitation.Accept(recipient, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	change := ports.AuthorizationChange{
		ID: "path-invite-change-1", ResourceType: "path", ResourceID: pathID,
		Relation: string(domain.RoleParticipant), SubjectType: "user", SubjectID: recipient,
		OwnerUserID: owner, ActorUserID: recipient, Operation: ports.AuthorizationTouch,
		LockedBy: "path-invite-worker", Lease: time.Minute,
	}
	accept := acceptInvitationCommand(accepted, change, acceptIdempotency, "path-invite-notification-2", "path-invite-accept-audit", owner)
	if err := migrationDB.Model(&model{}).Where("id = ?", pathID).
		Update("visibility", "public").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Accept(ctx, accept); !errors.Is(err, application.ErrInvitationWarningRequired) {
		t.Fatalf("stale followers acknowledgement error = %v, want warning required", err)
	}
	var partialReservations, partialMemberships, partialOutbox, partialAudits, partialNotifications int64
	if err := migrationDB.Model(&idempotencyModel{}).Where(
		"principal_id = ? AND operation = ? AND key = ?",
		recipient, application.AcceptInvitationOperation, acceptIdempotency.Key,
	).Count(&partialReservations).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&membershipModel{}).Where(
		"path_id = ? AND user_id = ?", pathID, recipient,
	).Count(&partialMemberships).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&authorizationOutboxModel{}).Where("id = ?", change.ID).
		Count(&partialOutbox).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&auditModel{}).Where("id = ?", accept.Audit.ID).
		Count(&partialAudits).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&notificationModel{}).Where("id = ?", accept.Notification.ID).
		Count(&partialNotifications).Error; err != nil {
		t.Fatal(err)
	}
	var stillPending invitationModel
	if err := migrationDB.Where("id = ?", invitation.ID).First(&stillPending).Error; err != nil {
		t.Fatal(err)
	}
	if partialReservations != 0 || partialMemberships != 0 || partialOutbox != 0 ||
		partialAudits != 0 || partialNotifications != 0 || stillPending.AcceptedAt != nil {
		t.Fatalf(
			"stale acknowledgement partial writes reservations=%d memberships=%d outbox=%d audits=%d notifications=%d accepted_at=%v",
			partialReservations, partialMemberships, partialOutbox, partialAudits,
			partialNotifications, stillPending.AcceptedAt,
		)
	}
	if err := migrationDB.Model(&model{}).Where("id = ?", pathID).
		Update("visibility", "private").Error; err != nil {
		t.Fatal(err)
	}
	result, err := repository.Accept(ctx, accept)
	if err != nil || result.Replayed || result.Invitation != accepted || result.AuthorizationChange.ID != change.ID {
		t.Fatalf("Accept = %+v, %v", result, err)
	}
	repository.WarningPolicy = failingInvitationWarningPolicy{}
	replayedAccept, err := repository.Accept(ctx, accept)
	if err != nil || !replayedAccept.Replayed || replayedAccept.Invitation != accepted ||
		replayedAccept.AuthorizationChange.ID != change.ID {
		t.Fatalf("accepted same-hash replay re-evaluated mutable warning state: %+v, %v", replayedAccept, err)
	}
	repository.WarningPolicy = application.VisibilityInvitationWarningPolicy{}
	if state, err := repository.AuthorizationChangeState(ctx, change.ID); err != nil || state != application.AuthorizationChangeLocked {
		t.Fatalf("new authorization change state = %q, %v; want locked", state, err)
	}
	assertInvitationSideEffects(t, migrationDB, invitation.ID, "path_invitation_accepted", owner, 1)
	ownerNotifications, err := repository.ListNotifications(ctx, owner, application.NotificationPageRequest{
		Limit: 10, Snapshot: now.Add(2 * time.Minute),
	})
	if err != nil || ownerNotifications.HasMore || len(ownerNotifications.Items) != 1 ||
		ownerNotifications.UnreadCount != 1 {
		t.Fatalf("owner ListNotifications = %+v, %v", ownerNotifications, err)
	}
	acceptedNotification := ownerNotifications.Items[0]
	if acceptedNotification.ID != "path-invite-notification-2" ||
		acceptedNotification.Kind != application.NotificationPathInvitationAccepted ||
		acceptedNotification.Presentation != application.NotificationInformational ||
		acceptedNotification.Actor.UserID != recipient ||
		acceptedNotification.Actor.Username != "Reader.One" ||
		acceptedNotification.InvitationID != invitation.ID {
		t.Fatalf("accepted notification projection = %+v", acceptedNotification)
	}
	if _, err := repository.GetNotification(ctx, recipient, receivedNotification.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("resolved actionable GetNotification error = %v, want ErrNotFound", err)
	}
	recipientNotifications, err = repository.ListNotifications(ctx, recipient, application.NotificationPageRequest{
		Limit: 10, Snapshot: now.Add(2 * time.Minute),
	})
	if err != nil || len(recipientNotifications.Items) != 0 || recipientNotifications.UnreadCount != 0 {
		t.Fatalf("resolved actionable notification list = %+v, %v", recipientNotifications, err)
	}
	assertHiddenInvitationExcludedFromMutationCount(
		t, ctx, repository, migrationDB, recipient, owner, string(path.ID), string(invitation.ID), now,
	)
	resolvedAccepted, err := repository.GetNotification(ctx, owner, acceptedNotification.ID)
	if err != nil || resolvedAccepted != acceptedNotification {
		t.Fatalf("owner GetNotification = %+v, %v", resolvedAccepted, err)
	}
	readAt := now.Add(2 * time.Minute)
	readCommand := notificationMutationCommand(
		owner, acceptedNotification.ID, readAt, "path-invite-read-audit", audit.ResourceUpdated,
	)
	readResult, err := repository.MarkNotificationRead(ctx, readCommand)
	if err != nil || readResult.UnreadCount != 0 {
		t.Fatalf("MarkNotificationRead = %+v, %v", readResult, err)
	}
	if _, err := repository.MarkNotificationRead(ctx, notificationMutationCommand(
		stranger, acceptedNotification.ID, readAt, "path-invite-foreign-read-audit", audit.ResourceUpdated,
	)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("cross-user MarkNotificationRead error = %v, want opaque ErrNotFound", err)
	}
	repeatedRead := notificationMutationCommand(
		owner, acceptedNotification.ID, readAt.Add(time.Minute), "path-invite-repeat-read-audit", audit.ResourceUpdated,
	)
	if _, err := repository.MarkNotificationRead(ctx, repeatedRead); err != nil {
		t.Fatalf("repeated MarkNotificationRead error = %v", err)
	}
	var persistedReadAt time.Time
	if err := migrationDB.Table("notification_models").Select("read_at").
		Where("id = ?", acceptedNotification.ID).Scan(&persistedReadAt).Error; err != nil ||
		!persistedReadAt.Equal(readAt) {
		t.Fatalf("idempotent read timestamp = %v, %v; want %v", persistedReadAt, err, readAt)
	}
	ownerNotifications, err = repository.ListNotifications(ctx, owner, application.NotificationPageRequest{
		Limit: 10, Snapshot: now.Add(3 * time.Minute),
	})
	if err != nil || len(ownerNotifications.Items) != 1 || !ownerNotifications.Items[0].Read ||
		ownerNotifications.UnreadCount != 0 {
		t.Fatalf("read notification projection = %+v, %v", ownerNotifications, err)
	}
	markAllResult, err := repository.MarkAllNotificationsRead(ctx, notificationMutationCommand(
		recipient, "history", readAt, "path-invite-read-all-audit", audit.ResourceUpdated,
	))
	if err != nil || markAllResult.UnreadCount != 0 {
		t.Fatalf("MarkAllNotificationsRead = %+v, %v", markAllResult, err)
	}
	rollbackDelete := notificationMutationCommand(
		owner, acceptedNotification.ID, readAt.Add(time.Minute), "path-invite-read-audit", audit.ResourceDeleted,
	)
	if _, err := repository.DeleteNotification(ctx, rollbackDelete); err == nil {
		t.Fatal("DeleteNotification with duplicate audit unexpectedly succeeded")
	}
	var deletedCount int64
	if err := migrationDB.Model(&notificationModel{}).
		Where("id = ? AND deleted_at IS NULL", acceptedNotification.ID).
		Count(&deletedCount).Error; err != nil || deletedCount != 1 {
		t.Fatalf("unaudited delete rollback count = %d, %v", deletedCount, err)
	}
	deleteResult, err := repository.DeleteNotification(ctx, notificationMutationCommand(
		owner, acceptedNotification.ID, readAt.Add(2*time.Minute), "path-invite-delete-audit", audit.ResourceDeleted,
	))
	if err != nil || deleteResult.UnreadCount != 0 {
		t.Fatalf("DeleteNotification = %+v, %v", deleteResult, err)
	}
	if _, err := repository.DeleteNotification(ctx, notificationMutationCommand(
		owner, acceptedNotification.ID, readAt.Add(3*time.Minute), "path-invite-repeat-delete-audit", audit.ResourceDeleted,
	)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("repeated DeleteNotification error = %v, want opaque ErrNotFound", err)
	}
	ownerNotifications, err = repository.ListNotifications(ctx, owner, application.NotificationPageRequest{
		Limit: 10, Snapshot: now.Add(3 * time.Minute),
	})
	if err != nil || len(ownerNotifications.Items) != 0 || ownerNotifications.UnreadCount != 0 {
		t.Fatalf("soft-deleted notification list = %+v, %v", ownerNotifications, err)
	}
	var membershipCount int64
	if err := migrationDB.Model(&membershipModel{}).
		Where("path_id = ? AND user_id = ? AND role = ?", pathID, recipient, domain.RoleParticipant).
		Count(&membershipCount).Error; err != nil || membershipCount != 1 {
		t.Fatalf("accepted membership count = %d, %v", membershipCount, err)
	}

	replayDecision, err := repository.AcceptanceDecision(ctx, recipient, invitation.ID, acceptIdempotency)
	if err != nil || replayDecision.Replay == nil || !replayDecision.Replay.Replayed ||
		replayDecision.Replay.Invitation != accepted || replayDecision.Replay.AuthorizationChange.ID != change.ID {
		t.Fatalf("accepted same-key replay decision = %+v, %v", replayDecision, err)
	}
	newKey := acceptIdempotency
	newKey.Key = "path-invite-new-key"
	if _, err := repository.AcceptanceDecision(ctx, recipient, invitation.ID, newKey); !errors.Is(err, domain.ErrInvitationUnavailable) {
		t.Fatalf("accepted new-key decision error = %v, want opaque unavailable", err)
	}

	completedAt := now.Add(2 * time.Minute)
	if err := migrationDB.Table("authorization_outbox_models").Where("id = ?", change.ID).
		Updates(map[string]any{"completed_at": completedAt, "locked_by": "", "locked_until": nil}).Error; err != nil {
		t.Fatal(err)
	}
	if state, err := repository.AuthorizationChangeState(ctx, change.ID); err != nil || state != application.AuthorizationChangeCompleted {
		t.Fatalf("completed authorization change state = %q, %v", state, err)
	}
	if err := migrationDB.Table("authorization_outbox_models").Where("id = ?", change.ID).
		Updates(map[string]any{"completed_at": nil, "dead_lettered_at": completedAt}).Error; err != nil {
		t.Fatal(err)
	}
	if state, err := repository.AuthorizationChangeState(ctx, change.ID); err != nil || state != application.AuthorizationChangeDeadLettered {
		t.Fatalf("dead-lettered authorization change state = %q, %v", state, err)
	}
	if _, err := repository.AuthorizationChangeState(ctx, "path-invite-missing-change"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("missing authorization change error = %v, want ErrNotFound", err)
	}
}

func notificationMutationCommand(
	recipient, notificationID string,
	changedAt time.Time,
	auditID string,
	action audit.Action,
) application.NotificationMutationCommand {
	return application.NotificationMutationCommand{
		RecipientUserID: recipient,
		NotificationID:  notificationID,
		ChangedAt:       changedAt,
		Audit: audit.Event{
			ID: auditID, OwnerUserID: recipient, ActorUserID: recipient,
			Action: action, TargetType: "notification", TargetID: notificationID,
			Outcome: audit.Succeeded, CorrelationID: auditID + "-correlation",
			OccurredAt: changedAt,
		},
	}
}

func TestPostgresPathInvitationConcurrentDifferentKeyAcceptHasOneWinnerAndNoDuplicateEffects(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 23, 19, 0, 0, 123_456_000, time.UTC)
	owner, recipient := "path-invite-race-owner", "path-invite-race-recipient"
	pathID := "path-invite-race-path"
	cleanupInvitationFixture(t, migrationDB, []string{owner, recipient}, []string{pathID})
	t.Cleanup(func() {
		cleanupInvitationFixture(t, migrationDB, []string{owner, recipient}, []string{pathID})
	})
	seedInvitationUser(t, migrationDB, owner, "Race.Owner", identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, migrationDB, recipient, "Race.Reader", identity.ProfileVisibilityPrivate, now)
	path := domain.Entity{
		ID: domain.ID(pathID), OwnerUserID: owner,
		Attributes: domain.Attributes{Name: "Concurrent acceptance", Visibility: "followers"},
		CreatedAt:  now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	seedGoalUpdatePath(t, migrationDB, path, owner)

	repository := New(runtimeDB)
	invitation, err := domain.NewInvitation(
		"path-invite-race-1", path.ID, owner, recipient, domain.RoleParticipant, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	send := sendInvitationCommand(
		invitation, "path-invite-race-send-notification", "path-invite-race-send-key",
		1, "path-invite-race-send-audit", owner,
	)
	send.ExpectedRecipientUserID = recipient
	send.ExpectedRecipientUsername = "Race.Reader"
	if _, err := repository.Send(ctx, send); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	accepted, err := invitation.Accept(recipient, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	commands := []application.AcceptInvitationCommand{
		acceptInvitationCommand(
			accepted,
			ports.AuthorizationChange{
				ID: "path-invite-race-change-1", ResourceType: "path", ResourceID: pathID,
				Relation: string(domain.RoleParticipant), SubjectType: "user", SubjectID: recipient,
				OwnerUserID: owner, ActorUserID: recipient, Operation: ports.AuthorizationTouch,
				LockedBy: "path-invite-race-worker", Lease: time.Minute,
			},
			ports.Idempotency{
				PrincipalID: recipient, Operation: application.AcceptInvitationOperation,
				Key: "path-invite-race-accept-key-1", RequestHash: bytes.Repeat([]byte{2}, 32),
			},
			"path-invite-race-accept-notification-1", "path-invite-race-accept-audit-1", owner,
		),
		acceptInvitationCommand(
			accepted,
			ports.AuthorizationChange{
				ID: "path-invite-race-change-2", ResourceType: "path", ResourceID: pathID,
				Relation: string(domain.RoleParticipant), SubjectType: "user", SubjectID: recipient,
				OwnerUserID: owner, ActorUserID: recipient, Operation: ports.AuthorizationTouch,
				LockedBy: "path-invite-race-worker", Lease: time.Minute,
			},
			ports.Idempotency{
				PrincipalID: recipient, Operation: application.AcceptInvitationOperation,
				Key: "path-invite-race-accept-key-2", RequestHash: bytes.Repeat([]byte{3}, 32),
			},
			"path-invite-race-accept-notification-2", "path-invite-race-accept-audit-2", owner,
		),
	}
	type outcome struct {
		result application.AcceptInvitationResult
		err    error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, len(commands))
	for _, command := range commands {
		command := command
		go func() {
			<-start
			result, err := repository.Accept(ctx, command)
			outcomes <- outcome{result: result, err: err}
		}()
	}
	close(start)

	var successes, opaqueDenials int
	for range commands {
		result := <-outcomes
		switch {
		case result.err == nil:
			successes++
			if result.result.Invitation != accepted {
				t.Fatalf("winning Accept() result = %+v", result.result)
			}
		case errors.Is(result.err, domain.ErrInvitationUnavailable):
			opaqueDenials++
		default:
			t.Fatalf("concurrent Accept() error = %v", result.err)
		}
	}
	if successes != 1 || opaqueDenials != 1 {
		t.Fatalf("concurrent Accept() successes=%d opaque denials=%d, want 1 each", successes, opaqueDenials)
	}

	assertInvitationSideEffects(t, migrationDB, invitation.ID, "path_invitation_accepted", owner, 1)
	var memberships, acceptanceReservations, authorizationChanges, acceptanceAudits int64
	if err := migrationDB.Model(&membershipModel{}).
		Where("path_id = ? AND user_id = ?", pathID, recipient).
		Count(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&idempotencyModel{}).
		Where("principal_id = ? AND operation = ?", recipient, application.AcceptInvitationOperation).
		Count(&acceptanceReservations).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("authorization_outbox_models").
		Where("id IN ?", []string{"path-invite-race-change-1", "path-invite-race-change-2"}).
		Count(&authorizationChanges).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Model(&auditModel{}).
		Where("action = ? AND target_type = ? AND target_id = ?",
			audit.PathInvitationAccepted, "path_invitation", invitation.ID).
		Count(&acceptanceAudits).Error; err != nil {
		t.Fatal(err)
	}
	if memberships != 1 || acceptanceReservations != 1 || authorizationChanges != 1 || acceptanceAudits != 1 {
		t.Fatalf(
			"concurrent acceptance effects memberships=%d reservations=%d authorization changes=%d audits=%d, want 1 each",
			memberships, acceptanceReservations, authorizationChanges, acceptanceAudits,
		)
	}
}
func seedInvitationUser(t *testing.T, db *gorm.DB, id, username string, visibility identity.ProfileVisibility, now time.Time) {
	t.Helper()
	row := map[string]any{
		"id": id, "username": username, "display_name": strings.ReplaceAll(username, ".", " "),
		"profile_visibility": visibility,
		"status":             identity.StatusActive, "created_at": now, "updated_at": now,
	}
	if err := db.Table("user_models").Create(row).Error; err != nil {
		t.Fatal(err)
	}
}

func cleanupInvitationFixture(t *testing.T, db *gorm.DB, userIDs, pathIDs []string) {
	t.Helper()
	_ = db.Table("notification_push_outbox_models").Where(
		"notification_id IN (SELECT id FROM notification_models WHERE path_id IN ?)", pathIDs,
	).Delete(map[string]any{}).Error
	_ = db.Table("notification_models").Where("path_id IN ?", pathIDs).Delete(map[string]any{}).Error
	_ = db.Table("path_invitation_models").Where("path_id IN ?", pathIDs).Delete(map[string]any{}).Error
	_ = db.Table("authorization_outbox_models").
		Where("owner_user_id IN ? OR actor_user_id IN ?", userIDs, userIDs).Delete(map[string]any{}).Error
	cleanupGoalUpdateFixture(t, db, userIDs, pathIDs)
}

func assertInvitationSideEffects(t *testing.T, db *gorm.DB, invitationID domain.InvitationID, kind, recipient string, want int64) {
	t.Helper()
	var notifications, pushes int64
	if err := db.Table("notification_models").
		Where("path_invitation_id = ? AND kind = ? AND recipient_user_id = ?", invitationID, kind, recipient).
		Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("notification_push_outbox_models").
		Where("notification_id IN (SELECT id FROM notification_models WHERE path_invitation_id = ? AND kind = ? AND recipient_user_id = ?)", invitationID, kind, recipient).
		Count(&pushes).Error; err != nil {
		t.Fatal(err)
	}
	if notifications != want || pushes != want {
		t.Fatalf("%s side effects notifications=%d pushes=%d, want %d each", kind, notifications, pushes, want)
	}
}

func assertInvitationReservationCount(t *testing.T, db *gorm.DB, principal, key string, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&idempotencyModel{}).Where(
		"principal_id = ? AND operation = ? AND key = ?",
		principal, application.SendInvitationOperation, key,
	).Count(&count).Error; err != nil || count != want {
		t.Fatalf("invitation reservation count = %d, %v; want %d", count, err, want)
	}
}
