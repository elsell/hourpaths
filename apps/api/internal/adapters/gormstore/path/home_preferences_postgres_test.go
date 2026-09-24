package pathstore

import (
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresHomePreferenceUpdateAcceptsEveryOwnedAndJoinedPathOnly(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	now := time.Date(2026, 8, 7, 4, 0, 0, 0, time.UTC)
	actor, joinedOwner, inaccessibleOwner := "home-preference-actor", "home-preference-joined-owner", "home-preference-inaccessible-owner"
	ownedPath, joinedPath, inaccessiblePath := "home-preference-owned", "home-preference-joined", "home-preference-inaccessible"
	users := []string{actor, joinedOwner, inaccessibleOwner}
	paths := []string{ownedPath, joinedPath, inaccessiblePath}
	cleanupGoalUpdateFixture(t, migrationDB, users, paths)
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migrationDB, users, paths) })
	seedGoalUpdateUsers(t, migrationDB, now, users...)
	if err := migrationDB.Table("user_preference_models").Create(map[string]any{
		"user_id": actor, "first_day_of_week": 1, "current_time_zone": "Etc/UTC", "created_at": now, "updated_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	for pathID, owner := range map[string]string{ownedPath: actor, joinedPath: joinedOwner, inaccessiblePath: inaccessibleOwner} {
		entity, entityErr := domain.New(domain.ID(pathID), owner, domain.Attributes{Name: pathID, Visibility: "private"})
		if entityErr != nil {
			t.Fatal(entityErr)
		}
		entity.CreatedAt, entity.UpdatedAt = now.Add(-time.Hour), now.Add(-time.Hour)
		if err := migrationDB.Create(fromEntity(entity)).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrationDB.Create(&membershipModel{PathID: joinedPath, UserID: actor, Role: "participant", JoinedAt: now.Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}

	repository := New(runtimeDB)
	command := homePreferenceCommand(actor, 0, "home-owned-joined-0001", []domain.ID{domain.ID(ownedPath)}, []domain.ID{domain.ID(ownedPath), domain.ID(joinedPath)}, now)
	result, err := repository.UpdateHomePreferences(context.Background(), command)
	if err != nil {
		t.Fatalf("owned and joined Path set rejected: %v", err)
	}
	if result.Revision != 1 || !samePathSet([]string{joinedPath, ownedPath}, result.ManualPathIDs) {
		t.Fatalf("result=%+v", result)
	}

	for name, requested := range map[string][]domain.ID{
		"inaccessible": {domain.ID(ownedPath), domain.ID(joinedPath), domain.ID(inaccessiblePath)},
		"omitted":      {domain.ID(ownedPath)},
	} {
		t.Run(name, func(t *testing.T) {
			attempt := homePreferenceCommand(actor, 1, "home-set-conflict-"+name, nil, requested, now.Add(time.Second))
			if _, err := repository.UpdateHomePreferences(context.Background(), attempt); !errors.Is(err, ports.ErrConflict) {
				t.Fatalf("error=%v want conflict", err)
			}
		})
	}
	persisted, err := repository.readHomePreferences(runtimeDB, actor)
	if err != nil || persisted.Revision != 1 || !samePathSet([]string{joinedPath, ownedPath}, persisted.ManualPathIDs) {
		t.Fatalf("persisted=%+v error=%v", persisted, err)
	}
	if err := migrationDB.Where("path_id = ? AND user_id = ?", joinedPath, actor).Delete(&membershipModel{}).Error; err != nil {
		t.Fatal(err)
	}
	var retained []homePathPreferenceModel
	if err := migrationDB.Where("user_id = ?", actor).Order("path_id").Find(&retained).Error; err != nil {
		t.Fatal(err)
	}
	if len(retained) != 1 || retained[0].PathID != ownedPath {
		t.Fatalf("membership cleanup retained preferences=%+v want owned Path only", retained)
	}
}

func homePreferenceCommand(actor string, revision int64, key string, pinned, manual []domain.ID, at time.Time) application.UpdateHomePreferencesCommand {
	return application.UpdateHomePreferencesCommand{
		ActorUserID: actor, ExpectedRevision: revision, OrderMethod: domain.HomeOrderRecent,
		PinnedPathIDs: pinned, ManualPathIDs: manual, UpdatedAt: at,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: application.UpdateHomePreferencesOperation, Key: key, RequestHash: make([]byte, 32)},
		Audit: audit.Event{
			ID: "audit-" + key, OwnerUserID: actor, ActorUserID: actor, Action: audit.ResourceUpdated,
			TargetType: "home_preferences", TargetID: actor, Outcome: audit.Succeeded, CorrelationID: key, OccurredAt: at,
		},
	}
}

func TestPostgresHomeProjectionIncludesOwnerWithoutMembershipAndCountsOwnerOnce(t *testing.T) {
	if *databaseDSN == "" || *migrationDatabaseDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	runtimeDB, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 4, 18, 0, 0, 0, time.UTC)
	owner, participant := "home-owner-no-membership", "home-owner-peer"
	legacyPath, ordinaryPath, sharedPath := "home-legacy-owner", "home-ordinary-owner", "home-shared-owner"
	users, paths := []string{owner, participant}, []string{legacyPath, ordinaryPath, sharedPath}
	cleanupGoalUpdateFixture(t, migrationDB, users, paths)
	t.Cleanup(func() { cleanupGoalUpdateFixture(t, migrationDB, users, paths) })
	seedGoalUpdateUsers(t, migrationDB, now, users...)
	for _, userID := range users {
		if err := migrationDB.Table("user_preference_models").Create(map[string]any{
			"user_id": userID, "first_day_of_week": 1, "current_time_zone": "Etc/UTC", "created_at": now, "updated_at": now,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, pathID := range paths {
		entity, entityErr := domain.New(domain.ID(pathID), owner, domain.Attributes{Name: pathID, Visibility: "private"})
		if entityErr != nil {
			t.Fatal(entityErr)
		}
		entity.CreatedAt, entity.UpdatedAt = now.Add(-time.Hour), now.Add(-time.Hour)
		if err := migrationDB.Create(fromEntity(entity)).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrationDB.Create(&membershipModel{PathID: ordinaryPath, UserID: owner, Role: "participant", JoinedAt: now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Create(&[]membershipModel{
		{PathID: sharedPath, UserID: owner, Role: "participant", JoinedAt: now.Add(-time.Hour)},
		{PathID: sharedPath, UserID: participant, Role: "participant", JoinedAt: now.Add(-time.Minute)},
	}).Error; err != nil {
		t.Fatal(err)
	}

	projection, err := New(runtimeDB).ProjectHome(context.Background(), owner, []domain.ID{domain.ID(legacyPath), domain.ID(ordinaryPath), domain.ID(sharedPath)})
	if err != nil {
		t.Fatal(err)
	}
	for _, pathID := range []string{legacyPath, ordinaryPath} {
		if got := projection.Organization[domain.ID(pathID)].Classification; got != application.HomeSolo {
			t.Fatalf("%s classification=%q want solo", pathID, got)
		}
	}
	if got := projection.Organization[domain.ID(sharedPath)].Classification; got != application.HomeShared {
		t.Fatalf("shared classification=%q want shared", got)
	}
}
