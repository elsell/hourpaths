package pathstore

import (
	"context"
	"errors"
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestPostgresAppearanceIsPersonalRevisionedAndAtomic(t *testing.T) {
	db, migration := goalUpdateDatabases(t)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	suffix := uuid.NewString()
	owner, member, outsider, pathID := "appearance-owner-"+suffix, "appearance-member-"+suffix, "appearance-outsider-"+suffix, "appearance-path-"+suffix
	users, paths := []string{owner, member, outsider}, []string{pathID}
	cleanupGoalUpdateFixture(t, migration, nil, paths)
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migration, nil, paths) })
	seedGoalUpdateUsers(t, migration, now, users...)
	entity, err := domain.New(domain.ID(pathID), owner, domain.Attributes{Name: "Appearance", Visibility: "private"})
	if err != nil {
		t.Fatal(err)
	}
	entity.CreatedAt, entity.UpdatedAt = now, now
	if err = migration.Create(fromEntity(entity)).Error; err != nil {
		t.Fatal(err)
	}
	if err = migration.Create(&membershipModel{PathID: pathID, UserID: member, Role: "supporter", JoinedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	ctx := context.Background()
	command := func(user, key, color string, revision int64) application.AppearanceCommand {
		return application.AppearanceCommand{UserID: user, PathID: domain.ID(pathID), Appearance: domain.Appearance{Color: color, Emoji: "🌱", Revision: revision}, UpdatedAt: now,
			Idempotency: ports.Idempotency{PrincipalID: user, Operation: "path.appearance.update", Key: key, RequestHash: make([]byte, 32)},
			Audit:       audit.Event{ID: "audit-" + suffix + key, OwnerUserID: user, ActorUserID: user, Action: audit.ResourceUpdated, TargetType: "path_appearance", TargetID: pathID, Outcome: audit.Succeeded, CorrelationID: key, OccurredAt: now}}
	}
	first := command(owner, "appearance-owner-0001", "mint", 0)
	saved, err := repository.SaveAppearance(ctx, first)
	if err != nil || saved.Revision != 1 {
		t.Fatalf("save=%+v error=%v", saved, err)
	}
	replay, err := repository.SaveAppearance(ctx, first)
	if err != nil || replay != saved {
		t.Fatalf("replay=%+v error=%v", replay, err)
	}
	changed := first
	changed.Idempotency.RequestHash = append([]byte{1}, make([]byte, 31)...)
	if _, err = repository.SaveAppearance(ctx, changed); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("changed retry=%v", err)
	}
	if _, err = repository.SaveAppearance(ctx, command(owner, "appearance-stale-0001", "blue", 0)); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale=%v", err)
	}
	empty, err := repository.ReadAppearance(ctx, member, domain.ID(pathID))
	if err != nil || empty.Revision != 0 {
		t.Fatalf("member inherited owner's appearance: %+v %v", empty, err)
	}
	if _, err = repository.SaveAppearance(ctx, command(member, "appearance-member-001", "coral", 0)); err != nil {
		t.Fatal(err)
	}
	if got, err := repository.ReadAppearance(ctx, owner, domain.ID(pathID)); err != nil || got != saved {
		t.Fatalf("member changed owner: %+v %v", got, err)
	}
	if _, err = repository.ReadAppearance(ctx, outsider, domain.ID(pathID)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("outsider read=%v", err)
	}
	if _, err = repository.SaveAppearance(ctx, command(outsider, "appearance-outsider-1", "blue", 0)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("outsider save=%v", err)
	}
	broken := command(owner, "appearance-audit-0001", "blue", 1)
	broken.Audit.ID = first.Audit.ID
	if _, err = repository.SaveAppearance(ctx, broken); err == nil {
		t.Fatal("duplicate audit should roll back mutation")
	}
	if got, err := repository.ReadAppearance(ctx, owner, domain.ID(pathID)); err != nil || got != saved {
		t.Fatalf("audit failure persisted change: %+v %v", got, err)
	}
	if err = migration.Where("path_id = ? AND user_id = ?", pathID, member).Delete(&membershipModel{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"path_appearance_models", "path_appearance_mutation_models"} {
		var count int64
		if err = migration.Table(table).Where("user_id = ?", member).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("membership cleanup %s=%d %v", table, count, err)
		}
	}
}
