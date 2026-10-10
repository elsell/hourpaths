package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	channelstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationchannel"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/preferences"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestPostgresUnavailablePeriodIsOwnedVersionedAndAtomic(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtime, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, runtime)
	admin, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, admin)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, peer := "quiet-owner-"+newTestID(), "quiet-peer-"+newTestID()
	for _, id := range []string{owner, peer} {
		seedTimeZonePreferenceUser(t, admin, id, "Etc/UTC", now.Add(-time.Hour))
		t.Cleanup(func() { admin.DB.Table("user_models").Where("id = ?", id).Delete(&struct{ ID string }{}) })
	}
	command := func(rev int64, enabled bool, key string) application.UnavailablePeriodCommand {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%t", rev, enabled)))
		return application.UnavailablePeriodCommand{ActorUserID: owner, ExpectedRevision: rev, ReviewedTimeZone: "Etc/UTC", Period: preferences.UnavailablePeriod{Enabled: enabled, StartMinute: 1320, EndMinute: 480}, ChangedAt: now,
			Idempotency: ports.Idempotency{PrincipalID: owner, Operation: application.UpdateUnavailablePeriodOperation, Key: key, RequestHash: hash[:]}, Audit: audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceUpdated, TargetType: "unavailable_period", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}}
	}
	initial, err := runtime.GetUnavailablePeriod(ctx, owner)
	if err != nil || initial.Period.Enabled || initial.Revision != 0 || initial.TimeZone != "Etc/UTC" {
		t.Fatalf("initial %+v %v", initial, err)
	}
	first := command(0, true, "quiet-first-00001")
	saved, err := runtime.UpdateUnavailablePeriod(ctx, first)
	if err != nil || !saved.Preference.Period.Enabled || saved.Preference.Revision != 1 {
		t.Fatalf("save %+v %v", saved, err)
	}
	if _, err = runtime.UpdateUnavailablePeriod(ctx, command(0, false, "quiet-stale-00001")); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale %v", err)
	}
	if _, err = runtime.UpdateUnavailablePeriod(ctx, command(1, false, "quiet-next-000001")); err != nil {
		t.Fatal(err)
	}
	replay, err := runtime.UpdateUnavailablePeriod(ctx, first)
	if err != nil || !replay.Replayed || !replay.Preference.Period.Enabled {
		t.Fatalf("replay %+v %v", replay, err)
	}
	current, err := runtime.GetUnavailablePeriod(ctx, owner)
	if err != nil || current.Period.Enabled || current.Revision != 2 {
		t.Fatalf("old replay replaced newer value %+v %v", current, err)
	}
	other, err := runtime.GetUnavailablePeriod(ctx, peer)
	if err != nil || other.Period.Enabled || other.Revision != 0 {
		t.Fatalf("peer %+v %v", other, err)
	}
	bad := command(2, true, "quiet-audit-00001")
	bad.Audit.ID = first.Audit.ID
	if _, err = runtime.UpdateUnavailablePeriod(ctx, bad); err == nil {
		t.Fatal("duplicate audit accepted")
	}
	current, err = runtime.GetUnavailablePeriod(ctx, owner)
	if err != nil || current.Revision != 2 {
		t.Fatalf("audit rollback %+v %v", current, err)
	}
	changedZone := command(2, true, "quiet-zone-000001")
	changedZone.ReviewedTimeZone = "America/New_York"
	if _, err = runtime.UpdateUnavailablePeriod(ctx, changedZone); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("zone conflict %v", err)
	}
	forged := command(2, true, "quiet-forged-0001")
	forged.Idempotency.PrincipalID = peer
	if _, err = runtime.UpdateUnavailablePeriod(ctx, forged); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("forged owner %v", err)
	}
}

func TestPostgresUnavailablePeriodSuppressesPushWithoutRemovingNotice(t *testing.T) {
	at := time.Date(2026, 10, 9, 23, 0, 0, 0, time.UTC)
	assertQuietPushSuppressed(t, at, at)
}
func TestPostgresUnavailablePeriodDoesNotCatchUpDelayedSocialPush(t *testing.T) {
	at := time.Date(2026, 10, 10, 7, 59, 58, 0, time.UTC)
	assertQuietPushSuppressed(t, at, at.Add(5*time.Second))
}
func assertQuietPushSuppressed(t *testing.T, at, eligibleAt time.Time) {
	t.Helper()
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtime, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, runtime)
	admin, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, admin)
	owner, peer, id := "quiet-push-owner-"+newTestID(), "quiet-push-peer-"+newTestID(), "quiet-notice-"+newTestID()
	for _, u := range []string{owner, peer} {
		seedTimeZonePreferenceUser(t, admin, u, "Etc/UTC", at.Add(-time.Hour))
		t.Cleanup(func() { admin.DB.Table("user_models").Where("id=?", u).Delete(&struct{ ID string }{}) })
	}
	if err = admin.DB.Create(&unavailablePeriodModel{UserID: owner, Enabled: true, StartMinute: 1320, EndMinute: 480, Revision: 1, UpdatedAt: at}).Error; err != nil {
		t.Fatal(err)
	}
	if err = admin.DB.Table("notification_models").Create(map[string]any{"id": id, "recipient_user_id": owner, "actor_user_id": peer, "follow_subject_user_id": peer, "kind": "new_follower", "presentation_class": "informational", "channel": "following", "created_at": at}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.DB.Table("notification_models").Where("id=?", id).Delete(&struct{ ID string }{}) })
	if err = runtime.DB.Transaction(func(tx *gorm.DB) error { return channelstore.QueuePushAvailable(tx, id, owner, at, eligibleAt) }); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SuppressedAt *time.Time
		FailureCode  string
	}
	if err = admin.DB.Table("notification_push_outbox_models").Where("notification_id=?", id).Take(&result).Error; err != nil || result.SuppressedAt == nil || result.FailureCode != "quiet_period" {
		t.Fatalf("quiet outbox %+v %v", result, err)
	}
	var count int64
	if err = admin.DB.Table("notification_models").Where("id=?", id).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("in-app notice lost: %d %v", count, err)
	}
	if err = admin.DB.Table("notification_push_delivery_models").Where("notification_id=?", id).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("quiet push queued: %d %v", count, err)
	}
	suppressed, err := channelstore.QuietAt(runtime.DB, owner, at.Add(9*time.Hour))
	if err != nil || suppressed {
		t.Fatalf("end boundary: %t %v", suppressed, err)
	}
}
