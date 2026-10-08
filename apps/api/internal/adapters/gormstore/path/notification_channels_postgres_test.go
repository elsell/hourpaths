package pathstore

import (
	"bytes"
	"context"
	"fmt"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"testing"
	"time"
)

func TestPostgresDisabledAccessChannelPreservesVisibilityChangeAndOtherRecipients(t *testing.T) {
	runtime, db := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, quiet, enabled, pathID := "channel-owner-"+suffix, "channel-quiet-"+suffix, "channel-enabled-"+suffix, "channel-path-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedInvitationUser(t, db, owner, "owner."+suffix, identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, db, quiet, "quiet."+suffix, identity.ProfileVisibilityPrivate, now)
	seedInvitationUser(t, db, enabled, "enabled."+suffix, identity.ProfileVisibilityPrivate, now)
	existing := domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Practice", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	seedGoalUpdatePath(t, db, existing, owner)
	if err := db.Create(&[]membershipModel{{PathID: pathID, UserID: quiet, Role: "participant"}, {PathID: pathID, UserID: enabled, Role: "participant"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("notification_channel_preference_models").Create(map[string]any{"user_id": quiet, "channel": "path_access", "enabled": false, "revision": 1, "created_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupInvitationFixture(t, db, []string{owner, quiet, enabled}, []string{pathID}) })
	changed, _ := existing.SetVisibility("followers", now)
	sequence := 0
	command := visibilityCommand(owner, existing.Visibility, changed, "channel-visibility-change", bytes.Repeat([]byte{3}, 32), "channel-visibility-audit-"+suffix, now, func() string { sequence++; return fmt.Sprintf("channel-notice-%s-%d", suffix, sequence) })
	result, err := New(runtime).SetVisibility(context.Background(), command)
	if err != nil || result.Path.Visibility != "followers" {
		t.Fatalf("visibility change %+v %v", result, err)
	}
	var recipients []string
	if err := db.Table("notification_models").Where("path_id = ?", pathID).Pluck("recipient_user_id", &recipients).Error; err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 1 || recipients[0] != enabled {
		t.Fatalf("channel recipients %v", recipients)
	}
}
