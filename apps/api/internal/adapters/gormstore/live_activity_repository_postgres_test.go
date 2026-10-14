package gormstore

import (
	"bytes"
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/liveactivity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"testing"
	"time"
)

func TestPostgresLiveActivityTokenRotationRevocationAndAtomicAudit(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtime, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, runtime)
	owner, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, owner)
	now := time.Now().UTC().Truncate(time.Microsecond)
	alice := socialRelationshipTestUser(t, owner.DB, "livealice", identity.ProfileVisibilityPublic, now)
	bob := socialRelationshipTestUser(t, owner.DB, "livebob", identity.ProfileVisibilityPublic, now)
	t.Cleanup(func() {
		owner.DB.Table("user_models").Where("id IN ?", []string{alice.ID, bob.ID}).Delete(&struct{ ID string }{})
	})
	repository, err := NewPushRepository(runtime.DB, bytes.Repeat([]byte{0x63}, 32), pushTestClock{now})
	if err != nil {
		t.Fatal(err)
	}
	event := func(actor, id string, action audit.Action) audit.Event {
		return audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, TargetType: "live_activity_registration", TargetID: id, Action: action, Outcome: audit.Succeeded, OccurredAt: now, CorrelationID: newTestID()}
	}
	registration := liveactivity.Registration{ID: newTestID(), OwnerUserID: alice.ID, AccountLease: newTestID(), Token: strings.Repeat("ab", 32), ExpiresAt: now.Add(time.Hour), Boundaries: []time.Time{now.Add(30 * time.Minute)}}
	firstAudit := event(alice.ID, registration.ID, audit.ResourceUpdated)
	if err := repository.UpsertLiveActivity(context.Background(), registration, firstAudit); err != nil {
		t.Fatal(err)
	}
	var stored liveActivityModel
	if err := owner.DB.Where("id=?", registration.ID).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored.TokenCiphertext, []byte(registration.Token)) || len(stored.TokenNonce) != 12 {
		t.Fatal("token not protected at rest")
	}
	other := registration
	other.OwnerUserID = bob.ID
	if err := repository.UpsertLiveActivity(context.Background(), other, event(bob.ID, registration.ID, audit.ResourceUpdated)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("cross-owner replacement: %v", err)
	}
	if err := repository.RevokeLiveActivity(context.Background(), bob.ID, registration.ID, registration.AccountLease, event(bob.ID, registration.ID, audit.ResourceDeleted)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("cross-owner revoke: %v", err)
	}
	claims, err := repository.ClaimLiveActivities(context.Background(), "worker", now, time.Minute, 10)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claim count=%d error=%v", len(claims), err)
	}
	registration.Token = strings.Repeat("cd", 32)
	if err := repository.UpsertLiveActivity(context.Background(), registration, event(alice.ID, registration.ID, audit.ResourceUpdated)); err != nil {
		t.Fatal(err)
	}
	called := false
	if err := repository.HandoffLiveActivity(context.Background(), "worker", claims[0], now, func(bool) ports.LiveActivityTransition { called = true; return ports.LiveActivityTransition{} }); err != nil || called {
		t.Fatalf("stale claim reached provider: %v", err)
	}
	claims, err = repository.ClaimLiveActivities(context.Background(), "worker", now, time.Minute, 10)
	if err != nil || len(claims) != 1 || claims[0].Registration.Token != registration.Token {
		t.Fatalf("rotated token unavailable: %v", err)
	}
	if err := repository.RevokeLiveActivity(context.Background(), alice.ID, registration.ID, registration.AccountLease, event(alice.ID, registration.ID, audit.ResourceDeleted)); err != nil {
		t.Fatal(err)
	}
	if err := repository.HandoffLiveActivity(context.Background(), "worker", claims[0], now, func(bool) ports.LiveActivityTransition { called = true; return ports.LiveActivityTransition{} }); err != nil || called {
		t.Fatalf("revoked claim reached provider: %v", err)
	}
	if err := owner.DB.Where("id=?", registration.ID).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored.TokenCiphertext) != 0 || stored.RevokedAt == nil {
		t.Fatal("revocation retained token")
	}
	if err := repository.UpsertLiveActivity(context.Background(), registration, event(alice.ID, registration.ID, audit.ResourceUpdated)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("delayed PUT revived revoked lease: %v", err)
	}
	another := registration
	another.ID = newTestID()
	another.AccountLease = newTestID()
	duplicateAudit := firstAudit
	duplicateAudit.TargetID = another.ID
	if err := repository.UpsertLiveActivity(context.Background(), another, duplicateAudit); err == nil {
		t.Fatal("duplicate audit unexpectedly committed")
	}
	var count int64
	owner.DB.Model(&liveActivityModel{}).Where("id=?", another.ID).Count(&count)
	if count != 0 {
		t.Fatal("registration survived failed audit")
	}
	// A runtime-role insert cannot exploit SQL CHECK's NULL semantics to retain an
	// active registration without an encrypted token.
	if err := runtime.DB.Exec(`INSERT INTO live_activity_registration_models
  (id,owner_user_id,account_lease,boundaries_json,version,expires_at,available_at,created_at,updated_at)
  VALUES(?,?,?,'[]',1,?,?,?,?)`, newTestID(), alice.ID, newTestID(), registration.ExpiresAt, now, now, now).Error; err == nil {
		t.Fatal("database accepted active registration without protected token")
	}

	concurrent := registration
	concurrent.ID = newTestID()
	concurrent.AccountLease = newTestID()
	if err := repository.UpsertLiveActivity(context.Background(), concurrent, event(alice.ID, concurrent.ID, audit.ResourceUpdated)); err != nil {
		t.Fatal(err)
	}
	claims, err = repository.ClaimLiveActivities(context.Background(), "worker", now, time.Minute, 10)
	if err != nil || len(claims) != 1 {
		t.Fatalf("concurrent claim: %v", err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	handoffDone := make(chan error, 1)
	revokeDone := make(chan error, 1)
	handoffEvent := event(alice.ID, concurrent.ID, audit.ResourceUpdated)
	go func() {
		handoffDone <- repository.HandoffLiveActivity(context.Background(), "worker", claims[0], now, func(eligible bool) ports.LiveActivityTransition {
			if !eligible {
				t.Error("active handoff unexpectedly ineligible")
			}
			close(entered)
			<-release
			return ports.LiveActivityTransition{Outcome: ports.LiveActivityAccepted, AvailableAt: now.Add(30 * time.Minute), Audit: handoffEvent}
		})
	}()
	select {
	case <-entered:
	case err := <-handoffDone:
		t.Fatalf("handoff never entered: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("handoff timed out")
	}
	revokeEvent := event(alice.ID, concurrent.ID, audit.ResourceDeleted)
	go func() {
		revokeDone <- repository.RevokeLiveActivity(context.Background(), alice.ID, concurrent.ID, concurrent.AccountLease, revokeEvent)
	}()
	select {
	case err := <-revokeDone:
		close(release)
		<-handoffDone
		t.Fatalf("revocation crossed active provider handoff: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-handoffDone; err != nil {
		t.Fatal(err)
	}
	if err := <-revokeDone; err != nil {
		t.Fatal(err)
	}

	// Bound active device registrations even when many authenticated requests race
	// or a client continually invents activity IDs.
	for i := 0; i < 8; i++ {
		value := registration
		value.ID = newTestID()
		value.AccountLease = newTestID()
		value.Token = strings.ReplaceAll(newTestID(), "-", "")
		if err := repository.UpsertLiveActivity(context.Background(), value, event(alice.ID, value.ID, audit.ResourceUpdated)); err != nil {
			t.Fatal(err)
		}
	}
	overflow := registration
	overflow.ID = newTestID()
	overflow.Token = strings.ReplaceAll(newTestID(), "-", "")
	if err := repository.UpsertLiveActivity(context.Background(), overflow, event(alice.ID, overflow.ID, audit.ResourceUpdated)); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("unbounded active registrations: %v", err)
	}

	claims, err = repository.ClaimLiveActivities(context.Background(), "worker", now, time.Minute, 25)
	if err != nil || len(claims) != 8 {
		t.Fatalf("pre-enforcement claims=%d error=%v", len(claims), err)
	}
	if err := owner.DB.Table("moderation_enforcement_models").Create(map[string]any{"id": newTestID(), "subject_user_id": alice.ID, "action": "ban", "policy_reason": "test enforcement", "issued_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.HandoffLiveActivity(context.Background(), "worker", claims[0], now, func(eligible bool) ports.LiveActivityTransition {
		if eligible {
			t.Error("banned account reached provider handoff")
		}
		return ports.LiveActivityTransition{Retired: true, AvailableAt: registration.ExpiresAt, Audit: event(alice.ID, claims[0].Registration.ID, audit.ResourceDeleted)}
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertLiveActivity(context.Background(), overflow, event(alice.ID, overflow.ID, audit.ResourceUpdated)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("banned account registration was not denied: %v", err)
	}
	now = now.Add(2 * time.Hour)
	claims, err = repository.ClaimLiveActivities(context.Background(), "worker", now, time.Minute, 25)
	if err != nil || len(claims) != 10 {
		t.Fatalf("expiry claims=%d error=%v", len(claims), err)
	}
	for _, claim := range claims {
		if err := repository.HandoffLiveActivity(context.Background(), "worker", claim, now, func(eligible bool) ports.LiveActivityTransition {
			if eligible {
				t.Error("expired registration remained eligible")
			}
			return ports.LiveActivityTransition{Retired: true, Audit: event(alice.ID, claim.Registration.ID, audit.ResourceDeleted)}
		}); err != nil {
			t.Fatal(err)
		}
	}
	owner.DB.Model(&liveActivityModel{}).Where("owner_user_id=?", alice.ID).Count(&count)
	if count != 0 {
		t.Fatal("expired token tombstones were retained")
	}

}
