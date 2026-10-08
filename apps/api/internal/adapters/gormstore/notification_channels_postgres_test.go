package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/notification"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresNotificationChannelsIndependentRevisionReplayAndAccountScope(t *testing.T) {
	f := newNudgeNotificationChannelFixture(t, "allchannels")
	ctx := context.Background()
	for _, channel := range notification.Channels() {
		command := socialapp.NotificationChannelCommand{Channel: channel, NudgeNotificationChannelCommand: nudgeNotificationChannelTestCommand(f.actor.ID, "channel-setting-"+string(channel), 0, false, f.now)}
		command.Audit.TargetID = string(channel)
		digest := sha256.Sum256([]byte(string(channel) + "\x00" + command.Idempotency.Key))
		command.Idempotency.RequestHash = digest[:]
		result, err := f.repository.UpdateNotificationChannel(ctx, command)
		if err != nil || result.Preference.Enabled || result.Preference.Revision != 1 || result.Replayed {
			t.Fatalf("%s result=%+v err=%v", channel, result, err)
		}
		replay, err := f.repository.UpdateNotificationChannel(ctx, command)
		if err != nil || !replay.Replayed {
			t.Fatalf("replay %s: %+v %v", channel, replay, err)
		}
		command.Idempotency.Key += "-stale"
		command.Audit.ID = newTestID()
		if _, err = f.repository.UpdateNotificationChannel(ctx, command); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("stale %s: %v", channel, err)
		}
		got, found, err := f.repository.GetNotificationChannel(ctx, f.actor.ID, channel)
		if err != nil || !found || got.Enabled || got.Revision != 1 {
			t.Fatalf("persisted %s: %+v %t %v", channel, got, found, err)
		}
		_, found, err = f.repository.GetNotificationChannel(ctx, f.other.ID, channel)
		if err != nil || found {
			t.Fatalf("cross account %s: %t %v", channel, found, err)
		}
	}
	command := socialapp.NotificationChannelCommand{Channel: "comments", NudgeNotificationChannelCommand: nudgeNotificationChannelTestCommand(f.actor.ID, "channel-setting-following", 0, false, f.now)}
	command.Audit.TargetID = "comments"
	digest := sha256.Sum256([]byte("comments\x00" + command.Idempotency.Key))
	command.Idempotency.RequestHash = digest[:]
	if _, err := f.repository.UpdateNotificationChannel(ctx, command); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("cross-channel replay accepted: %v", err)
	}
	if _, _, err := f.repository.GetNotificationChannel(ctx, f.actor.ID, "unknown"); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("unknown channel: %v", err)
	}
}

func TestPostgresNotificationChannelAuditFailurePreservesPreference(t *testing.T) {
	f := newNudgeNotificationChannelFixture(t, "channelaudit")
	ctx := context.Background()
	command := socialapp.NotificationChannelCommand{Channel: "comments", NudgeNotificationChannelCommand: nudgeNotificationChannelTestCommand(f.actor.ID, "channel-audit-first", 0, false, f.now)}
	command.Audit.TargetID = "comments"
	if _, err := f.repository.UpdateNotificationChannel(ctx, command); err != nil {
		t.Fatal(err)
	}
	command.Idempotency.Key = "channel-audit-collision"
	command.ExpectedRevision = 1
	command.Enabled = true
	if _, err := f.repository.UpdateNotificationChannel(ctx, command); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("audit collision: %v", err)
	}
	got, found, err := f.repository.GetNotificationChannel(ctx, f.actor.ID, "comments")
	if err != nil || !found || got.Enabled || got.Revision != 1 {
		t.Fatalf("rollback: %+v %t %v", got, found, err)
	}
}

func TestPostgresDisabledFollowingChannelPreservesFollowAndSuppressesNotification(t *testing.T) {
	repository, db := socialRelationshipTestRepository(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	actor, target := socialRelationshipTestUsers(t, db, identity.ProfileVisibilityPublic, now)
	command := socialapp.NotificationChannelCommand{Channel: "following", NudgeNotificationChannelCommand: nudgeNotificationChannelTestCommand(target.ID, "disabled-following-setting", 0, false, now)}
	command.Audit.TargetID = "following"
	if _, err := NewNudgeNotificationChannelRepository(repository.db).UpdateNotificationChannel(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	follow := socialRelationshipTestCommand(actor.ID, socialapp.FollowOperation, *target.Username, "disabled-following-action", now.Add(time.Second))
	result, err := repository.Follow(context.Background(), follow)
	if err != nil || !result.Changed || result.Target.Relationship != socialdomain.RelationshipFollowing {
		t.Fatalf("follow blocked: %+v %v", result, err)
	}
	var notices int64
	if err := db.Table("notification_models").Where("recipient_user_id = ?", target.ID).Count(&notices).Error; err != nil {
		t.Fatal(err)
	}
	if notices != 0 {
		t.Fatalf("disabled channel produced %d notifications", notices)
	}
}
