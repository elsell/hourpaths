package gormstore

import (
	"context"
	"errors"
	"testing"
	"time"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPathCreationProfileIsScopedToRequestedActiveUser(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-postgres-dsn is required for PostgreSQL integration")
	}
	db, err := gorm.Open(postgres.Open(*postgresTestDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	now := time.Now().UTC()
	publicVisibility := identity.ProfileVisibilityPublic
	privateVisibility := identity.ProfileVisibilityPrivate
	users := []userModel{
		{ID: "profile-reader-public", Status: identity.StatusActive, ProfileVisibility: &publicVisibility, CreatedAt: now, UpdatedAt: now},
		{ID: "profile-reader-private", Status: identity.StatusActive, ProfileVisibility: &privateVisibility, CreatedAt: now, UpdatedAt: now},
		{ID: "profile-reader-disabled", Status: identity.StatusDisabled, ProfileVisibility: &publicVisibility, CreatedAt: now, UpdatedAt: now},
		{ID: "profile-reader-incomplete", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "profile-reader-missing-preference", Status: identity.StatusActive, ProfileVisibility: &publicVisibility, CreatedAt: now, UpdatedAt: now},
	}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	preferences := []userPreferenceModel{
		{UserID: "profile-reader-public", FirstDayOfWeek: int16(identity.FirstDayMonday), CurrentTimeZone: "Etc/UTC", CreatedAt: now, UpdatedAt: now},
		{UserID: "profile-reader-private", FirstDayOfWeek: int16(identity.FirstDaySunday), CurrentTimeZone: "Etc/UTC", CreatedAt: now, UpdatedAt: now},
		{UserID: "profile-reader-disabled", FirstDayOfWeek: int16(identity.FirstDayMonday), CurrentTimeZone: "Etc/UTC", CreatedAt: now, UpdatedAt: now},
		{UserID: "profile-reader-incomplete", FirstDayOfWeek: int16(identity.FirstDayMonday), CurrentTimeZone: "Etc/UTC", CreatedAt: now, UpdatedAt: now},
	}
	if err := tx.Create(&preferences).Error; err != nil {
		t.Fatal(err)
	}

	store := &Store{DB: tx}
	got, err := store.PathCreationProfile(context.Background(), "profile-reader-private")
	want := pathapp.PathCreationProfile{ProfileVisibility: identity.ProfileVisibilityPrivate, FirstDayOfWeek: identity.FirstDaySunday}
	if err != nil || got != want {
		t.Fatalf("PathCreationProfile() = %+v, %v; want %+v", got, err, want)
	}
	for _, userID := range []string{"profile-reader-disabled", "profile-reader-missing", "profile-reader-missing-preference"} {
		if _, err := store.PathCreationProfile(context.Background(), userID); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("PathCreationProfile(%q) error = %v, want not found", userID, err)
		}
	}
	for _, userID := range []string{"profile-reader-incomplete"} {
		if _, err := store.PathCreationProfile(context.Background(), userID); !errors.Is(err, errInvalidPersistedPathCreationProfile) {
			t.Fatalf("PathCreationProfile(%q) error = %v, want invalid persisted state", userID, err)
		}
	}
	if _, err := store.PathCreationProfile(context.Background(), ""); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("empty user error = %v, want invalid argument", err)
	}
}

func TestPathCreationProfileRejectsInvalidPersistedValues(t *testing.T) {
	publicVisibility := identity.ProfileVisibilityPublic
	unsupportedVisibility := identity.ProfileVisibility("followers")
	for _, test := range []struct {
		name       string
		visibility *identity.ProfileVisibility
		firstDay   int16
	}{
		{name: "missing visibility", firstDay: int16(identity.FirstDayMonday)},
		{name: "unsupported visibility", visibility: &unsupportedVisibility, firstDay: int16(identity.FirstDayMonday)},
		{name: "weekday below ISO range", visibility: &publicVisibility, firstDay: 0},
		{name: "weekday above ISO range", visibility: &publicVisibility, firstDay: 8},
		{name: "weekday conversion overflow", visibility: &publicVisibility, firstDay: 257},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := pathCreationProfileFromPersistence(test.visibility, test.firstDay); !errors.Is(err, errInvalidPersistedPathCreationProfile) {
				t.Fatalf("pathCreationProfileFromPersistence() error = %v, want invalid persisted state", err)
			}
		})
	}
}

func TestTimeZoneIsScopedToRequestedActiveUser(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-postgres-dsn is required for PostgreSQL integration")
	}
	db, err := gorm.Open(postgres.Open(*postgresTestDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	now := time.Now().UTC()
	users := []userModel{
		{ID: "time-zone-reader-active", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "time-zone-reader-disabled", Status: identity.StatusDisabled, CreatedAt: now, UpdatedAt: now},
		{ID: "time-zone-reader-invalid", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	preferences := []userPreferenceModel{
		{UserID: users[0].ID, FirstDayOfWeek: int16(identity.FirstDayMonday), CurrentTimeZone: "America/New_York", CreatedAt: now, UpdatedAt: now},
		{UserID: users[1].ID, FirstDayOfWeek: int16(identity.FirstDayMonday), CurrentTimeZone: "Etc/UTC", CreatedAt: now, UpdatedAt: now},
		{UserID: users[2].ID, FirstDayOfWeek: int16(identity.FirstDayMonday), CurrentTimeZone: "Mars/Olympus", CreatedAt: now, UpdatedAt: now},
	}
	if err := tx.Create(&preferences).Error; err != nil {
		t.Fatal(err)
	}
	store := &Store{DB: tx}
	if zone, err := store.TimeZone(context.Background(), users[0].ID); err != nil || zone != "America/New_York" {
		t.Fatalf("TimeZone() = %q, %v", zone, err)
	}
	for _, userID := range []string{users[1].ID, "time-zone-reader-missing"} {
		if _, err := store.TimeZone(context.Background(), userID); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("TimeZone(%q) error = %v", userID, err)
		}
	}
	if _, err := store.TimeZone(context.Background(), users[2].ID); err == nil {
		t.Fatal("invalid persisted time zone accepted")
	}
}
