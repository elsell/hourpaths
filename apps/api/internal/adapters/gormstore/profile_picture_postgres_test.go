package gormstore

import (
	"bytes"
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"testing"
	"time"
)

func TestPostgresPictureAtomicReplacementRemovalAndCleanup(t *testing.T) {
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
	owner := "picture-" + newTestID()
	username := "picture_" + strings.ReplaceAll(newTestID(), "-", "")
	if err = migration.DB.Create(&userModel{ID: owner, Email: owner + "@example.test", Username: &username, DisplayName: "Picture owner", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	initial, err := runtime.GetOwnPicture(ctx, owner)
	if err != nil || initial.Revision != 1 || initial.URL != "" {
		t.Fatalf("initial=%+v %v", initial, err)
	}
	command := app.PictureWriteCommand{Owner: owner, ExpectedRevision: 1, AssetID: newTestID(), URL: "https://api.example.test/picture/first", JPEG: []byte{0xff, 0xd8, 0xff, 0xd9}, ChangedAt: now, Idempotency: ports.Idempotency{PrincipalID: owner, Operation: "account.profile.picture.update", Key: "picture-operation-0001", RequestHash: bytes.Repeat([]byte{1}, 32)}, Audit: audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceUpdated, TargetType: "user", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}}
	saved, err := runtime.WriteOwnPicture(ctx, command)
	if err != nil || saved.Revision != 2 || saved.URL != command.URL {
		t.Fatalf("saved=%+v %v", saved, err)
	}
	replay, err := runtime.WriteOwnPicture(ctx, command)
	if err != nil || replay != saved {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	if _, err = runtime.ReadPublicPicture(ctx, command.AssetID); err != nil {
		t.Fatal(err)
	}
	replacement := command
	replacement.Idempotency.Key = "picture-operation-0002"
	replacement.ExpectedRevision = 2
	replacement.AssetID = newTestID()
	replacement.URL = "https://api.example.test/picture/second"
	// A duplicate audit ID rolls the image replacement back atomically.
	if _, err = runtime.WriteOwnPicture(ctx, replacement); err == nil {
		t.Fatal("accepted failed audit")
	}
	unchanged, err := runtime.GetOwnPicture(ctx, owner)
	if err != nil || unchanged != saved {
		t.Fatal("failed replacement changed picture")
	}
	if _, err = runtime.ReadPublicPicture(ctx, command.AssetID); err != nil {
		t.Fatal("old bytes lost on rollback")
	}
	replacement.Audit.ID = newTestID()
	replaced, err := runtime.WriteOwnPicture(ctx, replacement)
	if err != nil || replaced.Revision != 3 {
		t.Fatalf("replace %+v %v", replaced, err)
	}
	if _, err = runtime.ReadPublicPicture(ctx, command.AssetID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("old media still accessible")
	}
	removal := replacement
	removal.ExpectedRevision = 3
	removal.Idempotency.Key = "picture-operation-0003"
	removal.Audit.ID = newTestID()
	removal.AssetID = ""
	removal.URL = ""
	removal.JPEG = nil
	removed, err := runtime.WriteOwnPicture(ctx, removal)
	if err != nil || removed.URL != "" || removed.Revision != 4 {
		t.Fatalf("removal %+v %v", removed, err)
	}
	if _, err = runtime.ReadPublicPicture(ctx, replacement.AssetID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("removed media accessible")
	}
	if err = runtime.DB.Model(&profilePictureMutationModel{}).Where("user_id = ?", owner).Update("revision", 99).Error; err == nil {
		t.Fatal("runtime mutated replay")
	}
	restore := replacement
	restore.ExpectedRevision = 4
	restore.Idempotency.Key = "picture-operation-0004"
	restore.Audit.ID = newTestID()
	restore.AssetID = newTestID()
	if _, err = runtime.WriteOwnPicture(ctx, restore); err != nil {
		t.Fatal(err)
	}
	if err = migration.DB.Where("id = ?", owner).Delete(&userModel{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.ReadPublicPicture(ctx, restore.AssetID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("deleted account image remained accessible")
	}
	var count int64
	runtime.DB.Table("user_profile_picture_mutation_models").Where("user_id = ?", owner).Count(&count)
	if count != 0 {
		t.Fatal("account cleanup retained replay")
	}
}
