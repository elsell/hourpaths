package gormstore

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresProfileEditingAtomicRetryAndConflicts(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL integration DSNs required")
	}
	runtime, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, other := "profile-owner-"+newTestID(), "profile-other-"+newTestID()
	username := "profile_" + strings.ReplaceAll(newTestID(), "-", "")
	taken := "taken_" + strings.ReplaceAll(newTestID(), "-", "")
	for id, name := range map[string]string{owner: username, other: taken} {
		row := userModel{ID: id, Email: id + "@example.test", Username: &name, DisplayName: "Original", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}
		if err := migration.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	initial, err := runtime.GetOwnProfile(ctx, owner)
	if err != nil || initial.Revision != 1 {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	command := app.ProfileUpdateCommand{ActorUserID: owner, Update: app.ProfileUpdate{ExpectedRevision: 1, Text: identity.ProfileText{Username: username, DisplayName: "Edited", Description: "About me"}}, ChangedAt: now.Add(time.Second), Idempotency: ports.Idempotency{PrincipalID: owner, Operation: "account.profile.update", Key: "profile-edit-key-0001", RequestHash: bytes.Repeat([]byte{1}, 32)}, Audit: audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceUpdated, TargetType: "user", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now.Add(time.Second)}}
	saved, err := runtime.UpdateOwnProfile(ctx, command)
	if err != nil || saved.Revision != 2 || saved.Text != command.Update.Text {
		t.Fatalf("save=%+v err=%v", saved, err)
	}
	replay, err := runtime.UpdateOwnProfile(ctx, command)
	if err != nil || replay != saved {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	changedKey := command
	changedKey.Idempotency.RequestHash = bytes.Repeat([]byte{2}, 32)
	if _, err := runtime.UpdateOwnProfile(ctx, changedKey); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("key conflict=%v", err)
	}
	failed := command
	failed.Idempotency.Key = "profile-edit-key-0002"
	failed.Audit.ID = newTestID()
	if _, err := runtime.UpdateOwnProfile(ctx, failed); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale=%v", err)
	}
	failed.Update.ExpectedRevision = 2
	failed.Update.Text.Username = strings.ToUpper(taken)
	if _, err := runtime.UpdateOwnProfile(ctx, failed); !errors.Is(err, ports.ErrUsernameUnavailable) {
		t.Fatalf("collision=%v", err)
	}
	failed.Update.Text.Username = username
	failed.Update.Text.DisplayName = "Must roll back"
	failed.Audit.ID = command.Audit.ID
	if _, err := runtime.UpdateOwnProfile(ctx, failed); err == nil {
		t.Fatal("duplicate audit accepted")
	}
	persisted, err := runtime.GetOwnProfile(ctx, owner)
	if err != nil || persisted != saved {
		t.Fatalf("failed save changed state=%+v err=%v", persisted, err)
	}
	untouched, err := runtime.GetOwnProfile(ctx, other)
	if err != nil || untouched.Text.Username != taken || untouched.Text.DisplayName != "Original" {
		t.Fatalf("other=%+v err=%v", untouched, err)
	}
	// Runtime cannot tamper with saved replay evidence; deleting the account removes it.
	if err := runtime.DB.Exec("UPDATE user_profile_mutation_models SET display_name = ? WHERE user_id = ?", "tampered", owner).Error; err == nil {
		t.Fatal("runtime can alter replay evidence")
	}
	if err := runtime.DB.Exec("DELETE FROM user_profile_mutation_models WHERE user_id = ?", owner).Error; err == nil {
		t.Fatal("runtime can delete replay evidence")
	}
	var count int64
	if err := migration.DB.Table("audit_event_models").Where("owner_user_id = ? AND target_type = 'user'", owner).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
	if err := migration.DB.Exec("DELETE FROM user_models WHERE id = ?", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := migration.DB.Table("user_profile_mutation_models").Where("user_id = ?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("account deletion retained profile replay: %d %v", count, err)
	}

}
