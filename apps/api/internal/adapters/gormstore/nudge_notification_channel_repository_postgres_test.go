package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

type nudgeNotificationChannelFixture struct {
	runtime, migration *Store
	repository         *NudgeNotificationChannelRepository
	actor, other       userModel
	now                time.Time
}

func newNudgeNotificationChannelFixture(t *testing.T, prefix string) nudgeNotificationChannelFixture {
	t.Helper()
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
	now := time.Date(2026, 8, 5, 18, 0, 0, 0, time.UTC)
	actor := socialRelationshipTestUser(t, migrationStore.DB, prefix+"actor", identity.ProfileVisibilityPublic, now)
	other := socialRelationshipTestUser(t, migrationStore.DB, prefix+"other", identity.ProfileVisibilityPublic, now)
	t.Cleanup(func() {
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{actor.ID, other.ID}).Delete(&struct{ ID string }{})
	})
	return nudgeNotificationChannelFixture{runtime: runtimeStore, migration: migrationStore, repository: NewNudgeNotificationChannelRepository(runtimeStore.DB), actor: actor, other: other, now: now}
}

func nudgeNotificationChannelTestCommand(actor, key string, expected int64, enabled bool, at time.Time) socialapp.NudgeNotificationChannelCommand {
	digest := sha256.Sum256([]byte(socialapp.NudgeNotificationChannel + "\x00" + key))
	return socialapp.NudgeNotificationChannelCommand{
		ActorUserID: actor, Enabled: enabled, ExpectedRevision: expected, OccurredAt: at,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: socialapp.UpdateNudgeNotificationChannelOperation, Key: key, RequestHash: digest[:]},
		Audit:       audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: audit.ResourceUpdated, TargetType: "notification_channel", TargetID: socialapp.NudgeNotificationChannel, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at},
	}
}

func TestPostgresNudgeNotificationChannelAbsentReadAndViewerScope(t *testing.T) {
	fixture := newNudgeNotificationChannelFixture(t, "nudgechannelread")
	ctx := context.Background()
	preference, found, err := fixture.repository.GetNudgeNotificationChannel(ctx, fixture.actor.ID)
	if err != nil || found || preference != (socialapp.NudgeNotificationChannelPreference{}) {
		t.Fatalf("absent preference=%+v found=%t err=%v", preference, found, err)
	}
	if err := fixture.migration.DB.Table("notification_channel_preference_models").Create(map[string]any{"user_id": fixture.actor.ID, "channel": socialapp.NudgeNotificationChannel, "enabled": false, "revision": 3, "created_at": fixture.now, "updated_at": fixture.now}).Error; err != nil {
		t.Fatal(err)
	}
	preference, found, err = fixture.repository.GetNudgeNotificationChannel(ctx, fixture.actor.ID)
	if err != nil || !found || preference != (socialapp.NudgeNotificationChannelPreference{Enabled: false, Revision: 3}) {
		t.Fatalf("stored preference=%+v found=%t err=%v", preference, found, err)
	}
	other, otherFound, err := fixture.repository.GetNudgeNotificationChannel(ctx, fixture.other.ID)
	if err != nil || otherFound || other != (socialapp.NudgeNotificationChannelPreference{}) {
		t.Fatalf("other preference=%+v found=%t err=%v", other, otherFound, err)
	}
	if _, _, err := fixture.repository.GetNudgeNotificationChannel(ctx, "missing-user-"+newTestID()); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("missing actor error=%v", err)
	}
	if err := fixture.migration.DB.Table("user_models").Where("id = ?", fixture.other.ID).Update("status", identity.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.repository.GetNudgeNotificationChannel(ctx, fixture.other.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("disabled actor error=%v", err)
	}
}

func TestPostgresNudgeNotificationChannelRevisionReplayConflictAndAtomicAudit(t *testing.T) {
	fixture := newNudgeNotificationChannelFixture(t, "nudgechannelupdate")
	ctx := context.Background()
	first := nudgeNotificationChannelTestCommand(fixture.actor.ID, "nudge-channel-key-0001", 0, false, fixture.now)
	created, err := fixture.repository.UpdateNudgeNotificationChannel(ctx, first)
	if err != nil || created.Replayed || created.Preference != (socialapp.NudgeNotificationChannelPreference{Enabled: false, Revision: 1}) {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	replayed, err := fixture.repository.UpdateNudgeNotificationChannel(ctx, first)
	if err != nil || !replayed.Replayed || replayed.Preference != created.Preference {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	conflict := first
	conflictHash := sha256.Sum256([]byte("conflicting notification channel request"))
	conflict.Idempotency.RequestHash = conflictHash[:]
	if _, err := fixture.repository.UpdateNudgeNotificationChannel(ctx, conflict); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict=%v", err)
	}
	stale := nudgeNotificationChannelTestCommand(fixture.actor.ID, "nudge-channel-key-0002", 0, true, fixture.now.Add(time.Second))
	if _, err := fixture.repository.UpdateNudgeNotificationChannel(ctx, stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale revision=%v", err)
	}
	second := nudgeNotificationChannelTestCommand(fixture.actor.ID, "nudge-channel-key-0003", 1, true, fixture.now.Add(2*time.Second))
	updated, err := fixture.repository.UpdateNudgeNotificationChannel(ctx, second)
	if err != nil || updated.Replayed || updated.Preference != (socialapp.NudgeNotificationChannelPreference{Enabled: true, Revision: 2}) {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	assertNudgeNotificationChannelEvidence(t, fixture.migration.DB, fixture.actor.ID, true, 2, 2, 2)
	assertNudgeNotificationChannelEvidence(t, fixture.migration.DB, fixture.other.ID, false, 0, 0, 0)
}

func TestPostgresNudgeNotificationChannelAuditFailureRollsBackPreferenceAndReplay(t *testing.T) {
	fixture := newNudgeNotificationChannelFixture(t, "nudgechannelrollback")
	ctx := context.Background()
	first := nudgeNotificationChannelTestCommand(fixture.actor.ID, "nudge-channel-key-0004", 0, false, fixture.now)
	if _, err := fixture.repository.UpdateNudgeNotificationChannel(ctx, first); err != nil {
		t.Fatal(err)
	}
	failed := nudgeNotificationChannelTestCommand(fixture.actor.ID, "nudge-channel-key-0005", 1, true, fixture.now.Add(time.Second))
	failed.Audit.ID = first.Audit.ID
	if _, err := fixture.repository.UpdateNudgeNotificationChannel(ctx, failed); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("audit rollback error=%v", err)
	}
	assertNudgeNotificationChannelEvidence(t, fixture.migration.DB, fixture.actor.ID, false, 1, 1, 1)
}

func assertNudgeNotificationChannelEvidence(t *testing.T, db *gorm.DB, actor string, enabled bool, revision, replays, audits int64) {
	t.Helper()
	var preference nudgeNotificationChannelPreferenceModel
	err := db.Where("user_id = ? AND channel = ?", actor, socialapp.NudgeNotificationChannel).Take(&preference).Error
	if revision == 0 {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("unexpected preference=%+v err=%v", preference, err)
		}
	} else if err != nil || preference.Enabled != enabled || preference.Revision != revision {
		t.Fatalf("preference=%+v err=%v", preference, err)
	}
	var replayCount, auditCount int64
	if err := db.Table("notification_channel_preference_replay_models").Where("actor_user_id = ? AND channel = ?", actor, socialapp.NudgeNotificationChannel).Count(&replayCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("audit_event_models").Where("actor_user_id = ? AND target_type = 'notification_channel' AND target_id = ?", actor, socialapp.NudgeNotificationChannel).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if replayCount != replays || auditCount != audits {
		t.Fatalf("replays=%d want=%d audits=%d want=%d", replayCount, replays, auditCount, audits)
	}
}
