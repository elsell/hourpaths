package pathstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"strconv"
	"strings"
	"testing"
	"time"

	gormstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	databaseDSN          = flag.String("database-dsn", "", "PostgreSQL runtime integration test DSN")
	migrationDatabaseDSN = flag.String("migration-database-dsn", "", "PostgreSQL migration-owner integration test DSN")
)

func TestPersistenceMappingPreservesDomainIdentityAndTime(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name     string
		interval domain.IntervalGoal
		overall  domain.OverallTarget
		archived bool
	}{
		{name: "neither"},
		{name: "archived", archived: true},
		{name: "hourly interval only", interval: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceHourly, Alignment: domain.GoalAlignment{Minute: 59}}},
		{name: "daily interval only", interval: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 23}}},
		{name: "weekly interval only", interval: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceWeekly, Alignment: domain.GoalAlignment{ISOWeekday: 7}}},
		{name: "monthly interval only", interval: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceMonthly, Alignment: domain.GoalAlignment{Day: 31}}},
		{name: "yearly interval only", interval: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceYearly, Alignment: domain.GoalAlignment{Month: 2, Day: 29}}},
		{name: "overall only", overall: domain.OverallTarget{Present: true, TargetSeconds: 3600}},
		{name: "both", interval: domain.IntervalGoal{Present: true, TargetSeconds: 600, Recurrence: domain.RecurrenceYearly, Alignment: domain.GoalAlignment{Month: 2, Day: 29}}, overall: domain.OverallTarget{Present: true, TargetSeconds: 36000}},
	} {
		t.Run(test.name, func(t *testing.T) {
			entity := domain.Entity{ID: "id", OwnerUserID: "owner", Attributes: domain.Attributes{Name: "value", Visibility: "private", IntervalGoal: test.interval, OverallTarget: test.overall}, CreatedAt: now, UpdatedAt: now}
			if test.archived {
				entity.ArchivedAt = now
			}
			got, err := toEntity(*fromEntity(entity))
			if err != nil || got != entity {
				t.Fatalf("mapping drifted: got %+v error %v want %+v", got, err, entity)
			}
			row := fromEntity(entity)
			if !test.interval.Present && (row.IntervalGoalTargetSeconds != nil || row.IntervalGoalRecurrence != nil || row.IntervalGoalStartMinute != nil || row.IntervalGoalStartHour != nil || row.IntervalGoalStartWeekday != nil || row.IntervalGoalStartDay != nil || row.IntervalGoalStartMonth != nil) {
				t.Fatalf("absent interval goal did not map to NULLs: %+v", row)
			}
			if !test.overall.Present && row.OverallTargetSeconds != nil {
				t.Fatalf("absent overall target did not map to NULL: %+v", row)
			}
		})
	}
}

func TestPersistenceMappingRejectsMalformedPartialGoalRows(t *testing.T) {
	valid := model{ID: "id", OwnerUserID: "owner", Name: "value", Visibility: "private", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	target, recurrence, minute, hour, weekday, day, month := int64(60), "hourly", int16(0), int16(0), int16(1), int16(1), int16(1)
	for _, test := range []struct {
		name   string
		mutate func(*model)
	}{
		{name: "target without recurrence", mutate: func(row *model) { row.IntervalGoalTargetSeconds = &target }},
		{name: "recurrence without target", mutate: func(row *model) { row.IntervalGoalRecurrence = &recurrence }},
		{name: "hourly without minute", mutate: func(row *model) { row.IntervalGoalTargetSeconds, row.IntervalGoalRecurrence = &target, &recurrence }},
		{name: "hourly with extra hour", mutate: func(row *model) {
			row.IntervalGoalTargetSeconds, row.IntervalGoalRecurrence, row.IntervalGoalStartMinute, row.IntervalGoalStartHour = &target, &recurrence, &minute, &hour
		}},
		{name: "dangling weekday", mutate: func(row *model) { row.IntervalGoalStartWeekday = &weekday }},
		{name: "dangling day", mutate: func(row *model) { row.IntervalGoalStartDay = &day }},
		{name: "dangling month", mutate: func(row *model) { row.IntervalGoalStartMonth = &month }},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := valid
			test.mutate(&row)
			if entity, err := toEntity(row); err == nil || entity != (domain.Entity{}) {
				t.Fatalf("malformed row returned entity %+v without a fail-closed error", entity)
			}
		})
	}
}

func TestPersistenceMappingCanonicalizesZeroOffsetTimestampsToUTC(t *testing.T) {
	databaseLocation := time.FixedZone("database-zero-offset", 0)
	createdAt := time.Date(2026, 7, 22, 12, 34, 56, 789, databaseLocation)
	updatedAt := createdAt.Add(time.Minute)

	entity, err := toEntity(model{ID: "id", OwnerUserID: "owner", Name: "value", Visibility: "private", CreatedAt: createdAt, UpdatedAt: updatedAt})
	if err != nil {
		t.Fatal(err)
	}
	if entity.CreatedAt != createdAt.UTC() || entity.UpdatedAt != updatedAt.UTC() {
		t.Fatalf("timestamps were not canonicalized: created=%#v updated=%#v", entity.CreatedAt, entity.UpdatedAt)
	}
	if entity.CreatedAt.Location() != time.UTC || entity.UpdatedAt.Location() != time.UTC {
		t.Fatalf("timestamp locations = %v, %v; want time.UTC", entity.CreatedAt.Location(), entity.UpdatedAt.Location())
	}
}

func TestPathVisibilityProducesAudienceAuthorizationRelationship(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	change := ports.AuthorizationChange{
		ID: "creator-change", ResourceType: "path", ResourceID: "path-id", Relation: "creator",
		SubjectType: "user", SubjectID: "owner", OwnerUserID: "owner", ActorUserID: "owner",
		Operation: ports.AuthorizationTouch, LockedBy: "worker", Lease: time.Minute,
	}
	entity := domain.Entity{ID: "path-id", OwnerUserID: "owner", Attributes: domain.Attributes{Name: "Practice", Visibility: "followers"}, CreatedAt: createdAt, UpdatedAt: createdAt}
	row, present := visibilityAuthorizationOutbox(entity, change)
	if !present {
		t.Fatal("followers-visible Path omitted its followers_owner relationship")
	}
	if row.ID != "creator-change-followers-owner" || row.ResourceType != "path" || row.ResourceID != "path-id" || row.Relation != "followers_owner" || row.SubjectType != "user" || row.SubjectID != "owner" || row.OwnerUserID != "owner" || row.ActorUserID != "owner" || row.Operation != ports.AuthorizationTouch || row.LockedBy != "worker" || !row.CreatedAt.Equal(createdAt) {
		t.Fatalf("followers_owner outbox row = %+v", row)
	}
	entity.Visibility = "private"
	if _, present := visibilityAuthorizationOutbox(entity, change); present {
		t.Fatal("private Path produced a followers_owner relationship")
	}
	entity.Visibility = "public"
	row, present = visibilityAuthorizationOutbox(entity, change)
	if !present || row.ID != "creator-change-public-viewer" || row.Relation != "public_viewer" || row.SubjectType != "user" || row.SubjectID != "*" {
		t.Fatalf("public_viewer outbox row = %+v, present=%v", row, present)
	}
}

func TestCutoverAuthorizationOutboxMustMatchTheApplicationRelationship(t *testing.T) {
	want := authorizationOutboxModel{ID: "creator-public-viewer", ResourceType: "path", ResourceID: "path", Relation: "public_viewer", SubjectType: "user", SubjectID: "*", OwnerUserID: "owner", ActorUserID: "owner", Operation: ports.AuthorizationTouch}
	fromTrigger := want
	fromTrigger.CreatedAt = time.Now().UTC()
	if !sameAuthorizationRelationship(fromTrigger, want) {
		t.Fatal("matching cutover relationship was rejected because delivery metadata differed")
	}
	fromTrigger.SubjectID = "someone"
	if sameAuthorizationRelationship(fromTrigger, want) {
		t.Fatal("conflicting cutover relationship was accepted")
	}
}

func TestPersistenceRejectsMalformedDomainGoals(t *testing.T) {
	entity := domain.Entity{
		ID:          "id",
		OwnerUserID: "owner",
		Attributes: domain.Attributes{
			Name:       "value",
			Visibility: "private",
			IntervalGoal: domain.IntervalGoal{
				Present:       true,
				TargetSeconds: 60,
				Recurrence:    domain.RecurrenceHourly,
				Alignment:     domain.GoalAlignment{Hour: 1},
			},
		},
	}
	if validEntityForPersistence(entity) {
		t.Fatal("malformed exported goal fields were accepted for persistence")
	}
}

func TestPostgresGoalCreateReplayGetAndListRoundTrips(t *testing.T) {
	if *databaseDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	db, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	const owner = "path-goal-roundtrip-owner"
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Email, DisplayName   string
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	if err := tx.Table("user_models").Create(&userRow{ID: owner, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	entities := []domain.Entity{
		{ID: "path-goal-neither", OwnerUserID: owner, Attributes: domain.Attributes{Name: "Neither", Visibility: "private"}, CreatedAt: now, UpdatedAt: now},
		{ID: "path-goal-interval", OwnerUserID: owner, Attributes: domain.Attributes{Name: "Interval", Visibility: "private", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceHourly, Alignment: domain.GoalAlignment{Minute: 59}}}, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)},
		{ID: "path-goal-overall", OwnerUserID: owner, Attributes: domain.Attributes{Name: "Overall", Visibility: "private", OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 3600}}, CreatedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second)},
		{ID: "path-goal-both", OwnerUserID: owner, Attributes: domain.Attributes{Name: "Both", Visibility: "private", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 600, Recurrence: domain.RecurrenceYearly, Alignment: domain.GoalAlignment{Month: 2, Day: 29}}, OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 36000}}, CreatedAt: now.Add(3 * time.Second), UpdatedAt: now.Add(3 * time.Second)},
	}

	repository := New(tx)
	for i, entity := range entities {
		change := ports.AuthorizationChange{ID: "path-goal-change-" + string(rune('0'+i)), ResourceType: "path", ResourceID: string(entity.ID), Relation: "creator", SubjectType: "user", SubjectID: owner, OwnerUserID: owner, ActorUserID: owner, Operation: ports.AuthorizationTouch, LockedBy: "worker", Lease: time.Minute}
		idempotency := ports.Idempotency{PrincipalID: owner, Operation: "path.create", Key: "path-goal-key-000" + string(rune('0'+i)), RequestHash: make([]byte, 32)}
		event := audit.Event{ID: "path-goal-audit-" + string(rune('0'+i)), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceCreated, TargetType: "path", TargetID: string(entity.ID), Outcome: audit.Succeeded, CorrelationID: "path-goal-roundtrip", OccurredAt: entity.CreatedAt}

		created, replayed, err := repository.Create(context.Background(), entity, change, idempotency, event)
		if err != nil || replayed || created != entity {
			t.Fatalf("Create(%s) = %+v, %v, %v; want exact non-replayed entity", entity.ID, created, replayed, err)
		}
		ignored := domain.Entity{ID: domain.ID("ignored-" + string(entity.ID)), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Ignored", Visibility: "public"}, CreatedAt: now.Add(time.Hour), UpdatedAt: now.Add(time.Hour)}
		change.ID, change.ResourceID = "ignored-change-"+string(rune('0'+i)), string(ignored.ID)
		event.ID, event.TargetID, event.OccurredAt = "ignored-audit-"+string(rune('0'+i)), string(ignored.ID), ignored.CreatedAt
		replayedEntity, replayed, err := repository.Create(context.Background(), ignored, change, idempotency, event)
		if err != nil || !replayed || replayedEntity != entity {
			t.Fatalf("replay(%s) = %+v, %v, %v; want exact original entity", entity.ID, replayedEntity, replayed, err)
		}
		got, err := repository.Get(context.Background(), owner, entity.ID)
		if err != nil || got != entity {
			t.Fatalf("Get(%s) = %+v, %v; want %+v", entity.ID, got, err, entity)
		}
	}

	page, err := repository.List(context.Background(), owner, application.PageRequest{Limit: 10, Snapshot: now.Add(time.Minute)})
	if err != nil || page.HasMore || len(page.Items) != len(entities) {
		t.Fatalf("List() = %+v, %v; want all goal combinations", page, err)
	}
	for i := range entities {
		if page.Items[i] != entities[i] {
			t.Fatalf("List item %d = %+v; want %+v", i, page.Items[i], entities[i])
		}
	}
}

func TestReadsRequirePrincipalAndScopePrivatePathsToExplicitMembership(t *testing.T) {
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
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	users := []string{"path-read-creator", "path-read-administrator", "path-read-participant", "path-read-supporter", "path-read-stranger"}
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Email, DisplayName   string
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	cleanup := func() {
		_ = migrationDB.Table("path_models").Where("id = ?", "private-path-read").Delete(&model{}).Error
		_ = migrationDB.Table("user_models").Where("id IN ?", users).Delete(&userRow{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)
	for _, userID := range users {
		if err := migrationDB.Table("user_models").Create(&userRow{ID: userID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	entity := domain.Entity{ID: "private-path-read", OwnerUserID: users[0], Attributes: domain.Attributes{Name: "Guitar", Visibility: "private"}, CreatedAt: now, UpdatedAt: now}
	if err := migrationDB.Create(fromEntity(entity)).Error; err != nil {
		t.Fatal(err)
	}
	type membershipRow struct{ PathID, UserID, Role string }
	memberships := []membershipRow{
		{PathID: string(entity.ID), UserID: users[0], Role: "participant"},
		{PathID: string(entity.ID), UserID: users[1], Role: "administrator"},
		{PathID: string(entity.ID), UserID: users[2], Role: "participant"},
		{PathID: string(entity.ID), UserID: users[3], Role: "supporter"},
	}
	if err := migrationDB.Table("path_membership_models").Create(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	repository := New(runtimeDB)
	for _, userID := range users[:4] {
		got, err := repository.Get(context.Background(), userID, entity.ID)
		if err != nil || got.ID != entity.ID || got.OwnerUserID != users[0] {
			t.Fatalf("Get(%s) = %+v, %v", userID, got, err)
		}
	}
	pageRequest := application.PageRequest{Limit: 25, Snapshot: now.Add(time.Hour)}
	for _, userID := range users[:4] {
		page, err := repository.List(context.Background(), userID, pageRequest)
		if err != nil || page.HasMore || len(page.Items) != 1 || page.Items[0].ID != entity.ID {
			t.Fatalf("List(%s) = %+v, %v", userID, page, err)
		}
	}
	empty, err := repository.List(context.Background(), users[4], pageRequest)
	if err != nil || empty.HasMore || len(empty.Items) != 0 {
		t.Fatalf("stranger List() = %+v, %v", empty, err)
	}
	if _, err := repository.List(context.Background(), "", pageRequest); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("unscoped list returned %v", err)
	}
	if _, err := repository.Get(context.Background(), users[4], entity.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("stranger read returned %v", err)
	}
	if _, err := repository.Get(context.Background(), "", entity.ID); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("unscoped read returned %v", err)
	}
}

func TestPostgresFollowerPathReadFailsClosedWhenAuthorizationRevocationDeadLetters(t *testing.T) {
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
	now := time.Now().UTC().Truncate(time.Microsecond)
	ownerID, viewerID := "follower-path-owner-"+newPathTestID(), "follower-path-viewer-"+newPathTestID()
	ownerUsername := "owner" + strings.ReplaceAll(newPathTestID(), "-", "")[:12]
	pathID := domain.ID("follower-path-" + newPathTestID())
	type userRow struct {
		ID                           string `gorm:"primaryKey"`
		Email, Username, DisplayName string
		Status                       identity.Status
		CreatedAt, UpdatedAt         time.Time
	}
	users := []userRow{
		{ID: ownerID, Username: ownerUsername, DisplayName: "Owner", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: viewerID, Username: "viewer" + strings.ReplaceAll(newPathTestID(), "-", "")[:12], DisplayName: "Viewer", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	if err := migrationDB.Table("user_models").Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	entity := domain.Entity{ID: pathID, OwnerUserID: ownerID, Attributes: domain.Attributes{Name: "Follower safety", Visibility: "followers"}, CreatedAt: now, UpdatedAt: now}
	if err := migrationDB.Create(fromEntity(entity)).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("follow_models").Create(map[string]any{"follower_user_id": viewerID, "following_user_id": ownerID, "created_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationDB.Table("path_models").Where("id = ?", pathID).Delete(&model{})
		migrationDB.Table("user_models").Where("id IN ?", []string{ownerID, viewerID}).Delete(&userRow{})
	})

	staleAuthorization := staleFollowerPathAuthorizer{}
	service := application.New(application.Dependencies{
		Auth: pathReadAuthenticator{userID: viewerID}, Authorizer: staleAuthorization,
		Repository: New(runtimeDB), Audits: pathReadAudits{}, AuditRateLimiter: pathReadRateLimiter{},
		Clock: pathReadClock{now: now},
	})
	if got, err := service.Get(context.Background(), "session", pathID); err != nil || got != entity {
		t.Fatalf("follower read before block=%+v err=%v", got, err)
	}

	sequence := 0
	blocks := gormstore.NewSocialRelationshipRepository(runtimeDB, func() string {
		sequence++
		return "follower-path-change-" + strconv.Itoa(sequence)
	}, "dead-letter-worker", time.Minute)
	digest := sha256.Sum256([]byte("block\x00" + viewerID + "\x00" + ownerUsername))
	command := socialapp.BlockCommand{
		ActorUserID: viewerID, TargetUsername: ownerUsername, TargetUserID: ownerID,
		ReviewExpiresAt: now.Add(time.Minute), OccurredAt: now.Add(time.Second),
		Idempotency: ports.Idempotency{PrincipalID: viewerID, Operation: socialapp.BlockOperation, Key: "follower-path-block-key", RequestHash: digest[:]},
		Audit:       audit.Event{ID: "follower-path-block-audit", OwnerUserID: viewerID, ActorUserID: viewerID, Action: audit.ResourceUpdated, TargetType: "user_block", TargetID: ownerUsername, Outcome: audit.Succeeded, CorrelationID: "follower-path-block", OccurredAt: now.Add(time.Second)},
	}
	blocked, err := blocks.Block(context.Background(), command)
	if err != nil || !blocked.Blocked || len(blocked.AuthorizationChanges) != 1 {
		t.Fatalf("block=%+v err=%v", blocked, err)
	}
	if err := migrationDB.Table("authorization_outbox_models").Where("id = ?", blocked.AuthorizationChanges[0].ID).
		Updates(map[string]any{"attempts": 3, "dead_lettered_at": now.Add(2 * time.Second), "failure_code": "authorizer_unavailable"}).Error; err != nil {
		t.Fatal(err)
	}
	if allowed, err := staleAuthorization.Check(context.Background(), "path", string(pathID), "view", viewerID); err != nil || !allowed {
		t.Fatalf("stale authorizer did not preserve the simulated grant: allowed=%t err=%v", allowed, err)
	}
	if _, err := service.Get(context.Background(), "session", pathID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("stale follower authorization exposed blocked Path: %v", err)
	}
}

type pathReadAuthenticator struct{ userID string }

func (auth pathReadAuthenticator) Authenticate(context.Context, string) (ports.Principal, error) {
	return ports.Principal{UserID: auth.userID, Scopes: []string{"api:user"}}, nil
}

type staleFollowerPathAuthorizer struct{}

func (staleFollowerPathAuthorizer) WriteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (staleFollowerPathAuthorizer) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return errors.New("authorizer unavailable")
}
func (staleFollowerPathAuthorizer) Check(context.Context, string, string, string, string) (bool, error) {
	return true, nil
}

type pathReadAudits struct{}

func (pathReadAudits) AppendAuditEvent(context.Context, audit.Event) error { return nil }
func (pathReadAudits) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}

type pathReadRateLimiter struct{}

func (pathReadRateLimiter) Allow(string, time.Time) bool { return true }

type pathReadClock struct{ now time.Time }

func (clock pathReadClock) Now() time.Time { return clock.now }

func newPathTestID() string { return uuid.NewString() }

func TestCreatePersistsRequiredPlatformRowsAtomically(t *testing.T) {
	if *databaseDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	db, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	now := time.Now().UTC()
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Email, DisplayName   string
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	if err := tx.Table("user_models").Create(&userRow{ID: "path-owner", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := New(tx)
	entity := domain.Entity{ID: "entity", OwnerUserID: "path-owner", Attributes: domain.Attributes{Name: "value", Visibility: "private"}, CreatedAt: now, UpdatedAt: now}
	createdEvent := audit.Event{ID: "path-created-audit", OwnerUserID: entity.OwnerUserID, ActorUserID: entity.OwnerUserID, Action: audit.ResourceCreated, TargetType: "path", TargetID: string(entity.ID), Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: now}
	createChange := ports.AuthorizationChange{ID: "path-touch", ResourceType: "path", ResourceID: string(entity.ID), Relation: "creator", SubjectType: "user", SubjectID: entity.OwnerUserID, OwnerUserID: entity.OwnerUserID, ActorUserID: createdEvent.ActorUserID, Operation: ports.AuthorizationTouch, LockedBy: "worker", Lease: time.Minute}
	legacyGenericChange := createChange
	legacyGenericChange.ResourceType, legacyGenericChange.ResourceID, legacyGenericChange.Relation = "resource", "path/"+string(entity.ID), "owner"
	if _, _, err := repository.Create(context.Background(), entity, legacyGenericChange, ports.Idempotency{PrincipalID: entity.OwnerUserID, Operation: "path.create", Key: "invalid-generic-create-key-0001", RequestHash: make([]byte, 32)}, createdEvent); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("generic authorization tuple was accepted: %v", err)
	}
	missingContext := createChange
	missingContext.OwnerUserID = ""
	if _, _, err := repository.Create(context.Background(), entity, missingContext, ports.Idempotency{PrincipalID: entity.OwnerUserID, Operation: "path.create", Key: "invalid-domain-create-key-0001", RequestHash: make([]byte, 32)}, createdEvent); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("missing authorization owner was accepted: %v", err)
	}
	created, replayed, err := repository.Create(context.Background(), entity, createChange, ports.Idempotency{PrincipalID: entity.OwnerUserID, Operation: "path.create", Key: "domain-create-key-0001", RequestHash: make([]byte, 32)}, createdEvent)
	if err != nil || replayed || created.ID != entity.ID {
		t.Fatalf("domain create failed: created=%+v replayed=%v err=%v", created, replayed, err)
	}
	var creatorMembership struct{ PathID, UserID, Role string }
	if err := tx.Table("path_membership_models").Where("path_id = ? AND user_id = ?", entity.ID, entity.OwnerUserID).First(&creatorMembership).Error; err != nil {
		t.Fatalf("creator membership missing: %v", err)
	}
	if creatorMembership.Role != "participant" {
		t.Fatalf("creator membership role = %q, want participant", creatorMembership.Role)
	}
	var membershipCount int64
	if err := tx.Table("path_membership_models").Where("path_id = ?", entity.ID).Count(&membershipCount).Error; err != nil || membershipCount != 1 {
		t.Fatalf("initial membership count = %d, %v, want exactly one", membershipCount, err)
	}
	replayedEntity, replayed, err := repository.Create(context.Background(), domain.Entity{ID: "ignored-replay-id", OwnerUserID: entity.OwnerUserID, Attributes: domain.Attributes{Name: "ignored", Visibility: "public"}, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)}, ports.AuthorizationChange{ID: "ignored-replay-change", ResourceType: "path", ResourceID: "ignored-replay-id", Relation: "creator", SubjectType: "user", SubjectID: entity.OwnerUserID, OwnerUserID: entity.OwnerUserID, ActorUserID: entity.OwnerUserID, Operation: ports.AuthorizationTouch, LockedBy: "worker", Lease: time.Minute}, ports.Idempotency{PrincipalID: entity.OwnerUserID, Operation: "path.create", Key: "domain-create-key-0001", RequestHash: make([]byte, 32)}, audit.Event{ID: "ignored-replay-audit", OwnerUserID: entity.OwnerUserID, ActorUserID: entity.OwnerUserID, Action: audit.ResourceCreated, TargetType: "path", TargetID: "ignored-replay-id", Outcome: audit.Succeeded, CorrelationID: "replay", OccurredAt: now.Add(time.Second)})
	if err != nil || !replayed || replayedEntity.ID != entity.ID {
		t.Fatalf("idempotent replay = %+v, %v, %v; want original entity", replayedEntity, replayed, err)
	}
	if err := tx.Table("path_membership_models").Where("path_id = ?", entity.ID).Count(&membershipCount).Error; err != nil || membershipCount != 1 {
		t.Fatalf("replay membership count = %d, %v, want exactly one", membershipCount, err)
	}
	if _, _, err := repository.Create(context.Background(), entity, createChange, ports.Idempotency{PrincipalID: entity.OwnerUserID, Operation: "path.create", Key: "domain-create-key-0001", RequestHash: bytes.Repeat([]byte{1}, 32)}, createdEvent); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("conflicting idempotency replay returned %v, want ErrIdempotencyConflict", err)
	}
	var reservation idempotencyModel
	if err := tx.Where("resource_id = ?", entity.ID).First(&reservation).Error; err != nil || reservation.CreatedAt.IsZero() {
		t.Fatalf("required idempotency timestamp missing: %+v %v", reservation, err)
	}
	var outbox authorizationOutboxModel
	if err := tx.Where("id = ?", createChange.ID).First(&outbox).Error; err != nil || outbox.CreatedAt.IsZero() || outbox.ResourceType != "path" || outbox.ResourceID != string(entity.ID) || outbox.Relation != "creator" || outbox.SubjectType != "user" || outbox.SubjectID != entity.OwnerUserID || outbox.OwnerUserID != entity.OwnerUserID || outbox.ActorUserID != createdEvent.ActorUserID {
		t.Fatalf("required outbox context missing: %+v %v", outbox, err)
	}
}

func TestPostgresPublicPathCreationConvergesWithMigrationCutoverTrigger(t *testing.T) {
	if *databaseDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	db, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	now := time.Now().UTC()
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Email, DisplayName   string
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	if err := tx.Table("user_models").Create(&userRow{ID: "public-cutover-owner", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	entity := domain.Entity{ID: "public-cutover-path", OwnerUserID: "public-cutover-owner", Attributes: domain.Attributes{Name: "Public", Visibility: "public"}, CreatedAt: now, UpdatedAt: now}
	change := ports.AuthorizationChange{ID: "public-cutover-creator", ResourceType: "path", ResourceID: string(entity.ID), Relation: "creator", SubjectType: "user", SubjectID: entity.OwnerUserID, OwnerUserID: entity.OwnerUserID, ActorUserID: entity.OwnerUserID, Operation: ports.AuthorizationTouch, LockedBy: "worker", Lease: time.Minute}
	idempotency := ports.Idempotency{PrincipalID: entity.OwnerUserID, Operation: "path.create", Key: "public-cutover-create-key", RequestHash: make([]byte, 32)}
	event := audit.Event{ID: "public-cutover-audit", OwnerUserID: entity.OwnerUserID, ActorUserID: entity.OwnerUserID, Action: audit.ResourceCreated, TargetType: "path", TargetID: string(entity.ID), Outcome: audit.Succeeded, CorrelationID: "public-cutover", OccurredAt: now}
	if created, replayed, err := New(tx).Create(context.Background(), entity, change, idempotency, event); err != nil || replayed || created != entity {
		t.Fatalf("Create() = %+v, %v, %v", created, replayed, err)
	}
	var rows []authorizationOutboxModel
	if err := tx.Where("resource_type = ? AND resource_id = ? AND relation = ?", "path", entity.ID, "public_viewer").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != change.ID+"-public-viewer" || rows[0].SubjectType != "user" || rows[0].SubjectID != "*" || rows[0].Operation != ports.AuthorizationTouch {
		t.Fatalf("public viewer outbox rows = %+v", rows)
	}
}

func TestCreateRollsBackPathMembershipIdempotencyAndAuditWhenOutboxInsertFails(t *testing.T) {
	if *databaseDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	db, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	now := time.Now().UTC()
	type userRow struct {
		ID                   string `gorm:"primaryKey"`
		Email, DisplayName   string
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	const owner = "path-atomic-owner"
	if err := tx.Table("user_models").Create(&userRow{ID: owner, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	conflict := authorizationOutboxModel{ID: "duplicate-path-outbox", ResourceType: "path", ResourceID: "existing", Relation: "creator", SubjectType: "user", SubjectID: owner, OwnerUserID: owner, ActorUserID: owner, Operation: ports.AuthorizationTouch, CreatedAt: now}
	if err := tx.Create(&conflict).Error; err != nil {
		t.Fatal(err)
	}
	repository := New(tx)
	entity := domain.Entity{ID: "rolled-back-path", OwnerUserID: owner, Attributes: domain.Attributes{Name: "Atomic", Visibility: "private", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 900, Recurrence: domain.RecurrenceWeekly, Alignment: domain.GoalAlignment{ISOWeekday: 1}}, OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 36000}}, CreatedAt: now, UpdatedAt: now}
	change := ports.AuthorizationChange{ID: conflict.ID, ResourceType: "path", ResourceID: string(entity.ID), Relation: "creator", SubjectType: "user", SubjectID: owner, OwnerUserID: owner, ActorUserID: owner, Operation: ports.AuthorizationTouch, LockedBy: "worker", Lease: time.Minute}
	idempotency := ports.Idempotency{PrincipalID: owner, Operation: "path.create", Key: "path-atomic-key-0001", RequestHash: make([]byte, 32)}
	event := audit.Event{ID: "rolled-back-path-audit", OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceCreated, TargetType: "path", TargetID: string(entity.ID), Outcome: audit.Succeeded, CorrelationID: "atomic", OccurredAt: now}
	if _, _, err := repository.Create(context.Background(), entity, change, idempotency, event); err == nil {
		t.Fatal("create succeeded despite duplicate outbox identifier")
	}
	for table, predicate := range map[string]string{
		"path_models":            "id = 'rolled-back-path'",
		"path_membership_models": "path_id = 'rolled-back-path'",
		"idempotency_models":     "principal_id = 'path-atomic-owner' AND operation = 'path.create' AND key = 'path-atomic-key-0001'",
		"audit_event_models":     "id = 'rolled-back-path-audit'",
	} {
		var count int64
		if err := tx.Table(table).Where(predicate).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s rollback count = %d, %v, want zero", table, count, err)
		}
	}
}

func TestTransactionalModelsCarryRequiredCreationTime(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	if got := (idempotencyModel{CreatedAt: now}).CreatedAt; got != now {
		t.Fatalf("idempotency creation time drifted: %v", got)
	}
	if got := (authorizationOutboxModel{CreatedAt: now}).CreatedAt; got != now {
		t.Fatalf("authorization outbox creation time drifted: %v", got)
	}
}
