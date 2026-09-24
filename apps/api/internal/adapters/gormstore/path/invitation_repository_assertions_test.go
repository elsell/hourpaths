package pathstore

import (
	"testing"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"gorm.io/gorm"
)

func assertInvitationPersistenceCount(t *testing.T, db *gorm.DB, invitationID domain.InvitationID, want int64) {
	t.Helper()
	var invitations, notifications, audits int64
	if err := db.Model(&invitationModel{}).Where("id = ?", invitationID).Count(&invitations).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&notificationModel{}).Where("path_invitation_id = ?", invitationID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&auditModel{}).Where(
		"target_type = ? AND target_id = ?", "path_invitation", invitationID,
	).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if invitations != want || notifications != want || audits != want {
		t.Fatalf("invitation persistence counts invitation=%d notification=%d audit=%d, want %d each", invitations, notifications, audits, want)
	}
}
