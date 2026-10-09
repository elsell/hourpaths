package gormstore

import (
	"context"
	"errors"
	"testing"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

// The client receives notification IDs, never internal nudge IDs. A report
// must resolve only the recipient's currently accessible notification.
func TestPostgresReportReceivedNudgeUsesRecipientNotification(t *testing.T) {
	f := newNudgeFixture(t, "reportnudge", false)
	ctx := context.Background()
	sent := nudgeSendTestCommand("nudge-"+newTestID(), f.sender.ID, f.recipient.ID, f.pathID, "report-nudge-send-01", socialdomain.NudgeLetsGo, f.now)
	if _, err := f.repository.SendNudge(ctx, sent); err != nil {
		t.Fatal(err)
	}
	var notice struct{ ID string }
	if err := f.migration.DB.Table("notification_models").Select("id").Where("nudge_id=? AND recipient_user_id=?", sent.Nudge.ID, f.recipient.ID).Take(&notice).Error; err != nil {
		t.Fatal(err)
	}
	r := NewModerationRepository(f.runtime.DB)
	target := domain.Target{Kind: domain.Nudge, ID: notice.ID}
	access, err := r.ResolveReportTarget(ctx, f.recipient.ID, target, f.now)
	if err != nil || access.SubjectUserID != f.sender.ID || access.PathID != f.pathID {
		t.Fatalf("recipient access=%+v error=%v", access, err)
	}
	for _, tc := range []struct{ viewer, id string }{{f.sender.ID, notice.ID}, {f.recipient.ID, sent.Nudge.ID}} {
		if _, err := r.ResolveReportTarget(ctx, tc.viewer, domain.Target{Kind: domain.Nudge, ID: tc.id}, f.now); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("inaccessible notification error=%v", err)
		}
	}
	if err := f.migration.DB.Create(&socialBlockTestModel{BlockerUserID: f.sender.ID, BlockedUserID: f.recipient.ID, CreatedAt: f.now}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := r.ResolveReportTarget(ctx, f.recipient.ID, target, f.now); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("blocked notification error=%v", err)
	}
}
