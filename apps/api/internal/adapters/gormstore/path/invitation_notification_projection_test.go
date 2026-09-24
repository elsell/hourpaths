package pathstore

import (
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestInvitationNotificationProjectionFailsClosedOnMalformedOrMismatchedContext(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	username := "Reader.One"
	readAt := now.Add(time.Minute)
	valid := invitationNotificationRow{
		NotificationPersistence: notificationModel{
			ID: "notification", RecipientUserID: "recipient", ActorUserID: "actor",
			PathID: "path", PathInvitationID: "invitation",
			Kind: "path_invitation_received", PresentationClass: "actionable",
			Channel: "path_access", OfferedRole: "supporter", CreatedAt: now, ReadAt: &readAt,
		},
		PathName: "Reading", ActorID: "actor", ActorUsername: &username, ActorName: "Reader One",
		InvitationPathID: "path", InvitationInviterID: "actor",
		InvitationRecipientID: "recipient", InvitationOfferedRole: "supporter",
		InvitationCreatedAt: &now,
	}
	projected, err := invitationNotificationFromRow(valid, "recipient")
	if err != nil || projected.ID != "notification" || !projected.Read ||
		projected.Actor != (application.InvitationPublicIdentity{
			UserID: "actor", Username: "Reader.One", DisplayName: "Reader One",
		}) {
		t.Fatalf("valid notification projection = %+v, %v", projected, err)
	}
	for name, mutate := range map[string]func(*invitationNotificationRow){
		"foreign recipient":  func(row *invitationNotificationRow) { row.RecipientUserID = "other" },
		"mismatched actor":   func(row *invitationNotificationRow) { row.ActorID = "other" },
		"missing username":   func(row *invitationNotificationRow) { row.ActorUsername = nil },
		"mismatched path":    func(row *invitationNotificationRow) { row.InvitationPathID = "other" },
		"mismatched role":    func(row *invitationNotificationRow) { row.InvitationOfferedRole = "participant" },
		"wrong semantics":    func(row *invitationNotificationRow) { row.InvitationInviterID = "other" },
		"wrong presentation": func(row *invitationNotificationRow) { row.PresentationClass = "informational" },
		"soft deleted": func(row *invitationNotificationRow) {
			deletedAt := now.Add(time.Minute)
			row.DeletedAt = &deletedAt
		},
	} {
		t.Run(name, func(t *testing.T) {
			row := valid
			mutate(&row)
			if result, err := invitationNotificationFromRow(row, "recipient"); err == nil ||
				result != (application.InvitationNotificationProjection{}) {
				t.Fatalf("malformed notification mapped to %+v, %v", result, err)
			}
		})
	}
}

func TestPathDeletionNoticeProjectsOnlyStandaloneSnapshots(t *testing.T) {
	now := time.Date(2026, 7, 27, 16, 0, 0, 0, time.UTC)
	pathName, username, displayName := "Former Guitar", "Creator.One", "Creator One"
	row := invitationNotificationRow{
		NotificationPersistence: notificationModel{
			ID: "deletion-notice", RecipientUserID: "participant", ActorUserID: "creator",
			Kind: string(application.NotificationPathDeleted), PresentationClass: string(application.NotificationInformational),
			Channel: "path_access", CreatedAt: now, PathNameSnapshot: &pathName,
			ActorUsernameSnapshot: &username, ActorDisplayNameSnapshot: &displayName,
		},
		PathName: pathName, ActorID: "creator",
	}
	projected, err := invitationNotificationFromRow(row, "participant")
	if err != nil || projected.PathID != "" || projected.InvitationID != "" || projected.OwnershipTransferID != "" || projected.PathName != pathName || projected.Actor.Username != username || projected.Actor.DisplayName != displayName {
		t.Fatalf("deletion notice = %+v, %v", projected, err)
	}
	row.PathID = "deleted-path"
	if projected, err := invitationNotificationFromRow(row, "participant"); err == nil || projected != (application.InvitationNotificationProjection{}) {
		t.Fatalf("navigable deletion notice projected as %+v, %v", projected, err)
	}
}

func TestMemberAccessNotificationProjectsInformationalRoleContext(t *testing.T) {
	now := time.Date(2026, 7, 29, 20, 0, 0, 0, time.UTC)
	username := "Manager.One"
	for _, kind := range []application.InvitationNotificationKind{application.NotificationPathMemberRemoved, application.NotificationPathMemberRoleChanged} {
		t.Run(string(kind), func(t *testing.T) {
			row := invitationNotificationRow{NotificationPersistence: notificationModel{ID: "access-notice", RecipientUserID: "member", ActorUserID: "manager", PathID: "path", Kind: string(kind), PresentationClass: string(application.NotificationInformational), Channel: "path_access", OfferedRole: "supporter", CreatedAt: now}, PathName: "Reading", ActorID: "manager", ActorUsername: &username, ActorName: "Manager One"}
			projected, err := invitationNotificationFromRow(row, "member")
			if err != nil || projected.Kind != kind || projected.OfferedRole != "supporter" || projected.PathID != "path" || projected.PathName != "Reading" {
				t.Fatalf("projection=%+v err=%v", projected, err)
			}
			row.OfferedRole = ""
			if malformed, err := invitationNotificationFromRow(row, "member"); err == nil || malformed != (application.InvitationNotificationProjection{}) {
				t.Fatalf("malformed projection=%+v err=%v", malformed, err)
			}
		})
	}
}

func TestAdministratorRoleChangeNotificationsProjectGrantAndSelfStepDown(t *testing.T) {
	now := time.Date(2026, 8, 3, 20, 0, 0, 0, time.UTC)
	username := "Manager.One"
	for _, tc := range []struct {
		name, recipient, actor string
		role                   domain.MembershipRole
	}{
		{"grant", "member", "manager", domain.RoleAdministrator},
		{"self step down", "manager", "manager", domain.RoleParticipant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := invitationNotificationRow{NotificationPersistence: notificationModel{ID: "administrator-notice", RecipientUserID: tc.recipient, ActorUserID: tc.actor, PathID: "path", Kind: string(application.NotificationPathMemberRoleChanged), PresentationClass: string(application.NotificationInformational), Channel: "path_access", OfferedRole: string(tc.role), CreatedAt: now}, PathName: "Reading", ActorID: tc.actor, ActorUsername: &username, ActorName: "Manager One"}
			projected, err := invitationNotificationFromRow(row, tc.recipient)
			if err != nil || projected.OfferedRole != tc.role {
				t.Fatalf("projection=%+v err=%v", projected, err)
			}
		})
	}
	row := invitationNotificationRow{NotificationPersistence: notificationModel{ID: "invalid-self", RecipientUserID: "manager", ActorUserID: "manager", PathID: "path", Kind: string(application.NotificationPathMemberRoleChanged), PresentationClass: string(application.NotificationInformational), Channel: "path_access", OfferedRole: string(domain.RoleAdministrator), CreatedAt: now}, PathName: "Reading", ActorID: "manager", ActorUsername: &username, ActorName: "Manager One"}
	if projected, err := invitationNotificationFromRow(row, "manager"); err == nil || projected != (application.InvitationNotificationProjection{}) {
		t.Fatalf("self grant projected=%+v err=%v", projected, err)
	}
}
