package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresWeekStartPreferencePreservesOtherUsersAndReplayHistory(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtime, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, peer := "week-owner-"+newTestID(), "week-peer-"+newTestID()
	for _, id := range []string{owner, peer} {
		seedTimeZonePreferenceUser(t, admin, id, "Etc/UTC", now.Add(-time.Hour))
		t.Cleanup(func() { admin.DB.Table("user_models").Where("id = ?", id).Delete(&struct{ ID string }{}) })
	}
	command := func(id string, before, after int, key string) application.WeekStartPreferenceCommand {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", before, after)))
		return application.WeekStartPreferenceCommand{ActorUserID: id, ReviewedFirstDayOfWeek: before, ProposedFirstDayOfWeek: after, ChangedAt: now,
			Idempotency: ports.Idempotency{PrincipalID: id, Operation: application.UpdateWeekStartPreferenceOperation, Key: key, RequestHash: hash[:]},
			Audit:       audit.Event{ID: newTestID(), OwnerUserID: id, ActorUserID: id, Action: audit.ResourceUpdated, TargetType: "week_start_preference", TargetID: id, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}}
	}
	before, err := runtime.GetWeekStartPreference(ctx, owner)
	if err != nil || before.FirstDayOfWeek != 1 {
		t.Fatalf("initial: %+v %v", before, err)
	}
	first := command(owner, 1, 7, "week-start-first-0001")
	saved, err := runtime.UpdateWeekStartPreference(ctx, first)
	if err != nil || saved.Preference.FirstDayOfWeek != 7 || !saved.Changed {
		t.Fatalf("save: %+v %v", saved, err)
	}
	if _, err = runtime.UpdateWeekStartPreference(ctx, command(owner, 1, 2, "week-start-stale-0001")); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale save: %v", err)
	}
	if _, err = runtime.UpdateWeekStartPreference(ctx, command(owner, 7, 2, "week-start-next-00001")); err != nil {
		t.Fatal(err)
	}
	replay, err := runtime.UpdateWeekStartPreference(ctx, first)
	if err != nil || !replay.Replayed || replay.Preference.FirstDayOfWeek != 7 {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	current, err := runtime.GetWeekStartPreference(ctx, owner)
	if err != nil || current.FirstDayOfWeek != 2 {
		t.Fatalf("replay overwrote newer preference: %+v %v", current, err)
	}
	other, err := runtime.GetWeekStartPreference(ctx, peer)
	if err != nil || other.FirstDayOfWeek != 1 {
		t.Fatalf("cross-user preference: %+v %v", other, err)
	}
	if _, err = runtime.UpdateWeekStartPreference(ctx, command(owner, 2, 3, first.Idempotency.Key)); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("key reuse: %v", err)
	}
	bad := command(owner, 2, 4, "week-start-bad-audit1")
	bad.Audit.ID = first.Audit.ID
	if _, err = runtime.UpdateWeekStartPreference(ctx, bad); err == nil {
		t.Fatal("duplicate audit accepted")
	}
	current, err = runtime.GetWeekStartPreference(ctx, owner)
	if err != nil || current.FirstDayOfWeek != 2 {
		t.Fatalf("audit failure persisted: %+v %v", current, err)
	}
	var count int64
	if err = admin.DB.Table("user_week_start_preference_mutation_models").Where("user_id = ?", owner).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("receipt count %d %v", count, err)
	}
	if err = admin.DB.Table("audit_event_models").Where("owner_user_id = ? AND target_type = ?", owner, "week_start_preference").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("audit count %d %v", count, err)
	}
	for _, value := range []int{-1, 0, 8, 257} {
		badValue := command(owner, 2, value, "week-start-invalid-01")
		if _, err = runtime.UpdateWeekStartPreference(ctx, badValue); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("invalid weekday %d: %v", value, err)
		}
	}
	forged := command(owner, 2, 4, "week-start-forged-01")
	forged.Idempotency.PrincipalID = peer
	if _, err = runtime.UpdateWeekStartPreference(ctx, forged); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("forged principal: %v", err)
	}
}
