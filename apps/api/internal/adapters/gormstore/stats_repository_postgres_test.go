package gormstore

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"testing"
	"time"
)

func TestPostgresStatsIncludesOnlyOwnRecordedMembershipActivity(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtime, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, runtime)
	closeSocialProfileTestStore(t, migration)
	now := time.Now().UTC().Truncate(time.Microsecond)
	viewer := socialRelationshipTestUser(t, migration.DB, "statsviewer", identity.ProfileVisibilityPublic, now)
	other := socialRelationshipTestUser(t, migration.DB, "statsother", identity.ProfileVisibilityPublic, now)
	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		ArchivedAt                        *time.Time
		CreatedAt, UpdatedAt              time.Time
	}
	type member struct{ PathID, UserID, Role string }
	type activity struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
		Note                                          *string
	}
	pathID, foreignID := newTestID(), newTestID()
	t.Cleanup(func() {
		migration.DB.Table("path_models").Where("id IN ?", []string{pathID, foreignID}).Delete(&struct{ ID string }{})
		migration.DB.Table("user_preference_models").Where("user_id IN ?", []string{viewer.ID, other.ID}).Delete(&struct{ UserID string }{})
		migration.DB.Table("user_models").Where("id IN ?", []string{viewer.ID, other.ID}).Delete(&struct{ ID string }{})
	})
	if err = migration.DB.Create(&userPreferenceModel{UserID: viewer.ID, FirstDayOfWeek: 1, CurrentTimeZone: "Pacific/Auckland", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err = migration.DB.Table("path_models").Create([]pathRow{{pathID, other.ID, "Shared", "public", &now, now, now}, {foreignID, other.ID, "Foreign", "public", nil, now, now}}).Error; err != nil {
		t.Fatal(err)
	}
	if err = migration.DB.Table("path_membership_models").Create([]member{{pathID, viewer.ID, "participant"}, {pathID, other.ID, "participant"}, {foreignID, other.ID, "participant"}}).Error; err != nil {
		t.Fatal(err)
	}
	note := "private text must never enter analytics"
	rows := []activity{{newTestID(), pathID, viewer.ID, "America/New_York", now.Add(-time.Hour), now, now, now, &note}, {newTestID(), pathID, other.ID, "Etc/UTC", now.Add(-2 * time.Hour), now, now, now, nil}, {newTestID(), foreignID, other.ID, "Etc/UTC", now.Add(-3 * time.Hour), now, now, now, nil}}
	if err = migration.DB.Table("recorded_activity_models").Create(rows).Error; err != nil {
		t.Fatal(err)
	}
	s, err := (StatsRepository{DB: runtime.DB}).Read(context.Background(), viewer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Paths) != 1 || s.Paths[0].ID != pathID || !s.Paths[0].Archived || len(s.Records) != 1 || s.Records[0].PathID != pathID || s.Records[0].TimeZone != "America/New_York" || s.TimeZone != "Pacific/Auckland" {
		t.Fatalf("incorrect scope %+v", s)
	}
}
