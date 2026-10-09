package gormstore

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

func TestPostgresExportProfileScopesAccountAndExcludesInactiveOwners(t *testing.T) {
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
	closeSocialProfileTestStore(t, runtime)
	closeSocialProfileTestStore(t, admin)
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, peer := "export-owner-"+newTestID(), "export-peer-"+newTestID()
	for _, id := range []string{owner, peer} {
		seedTimeZonePreferenceUser(t, admin, id, "Etc/UTC", now)
		t.Cleanup(func() { admin.DB.Table("user_models").Where("id = ?", id).Delete(&struct{}{}) })
	}
	for _, id := range []string{owner, peer} {
		value, err := runtime.ReadExportProfile(context.Background(), id)
		if err != nil || value.UserID != id || value.Email != id+"@example.com" || value.TimeZone != "Etc/UTC" {
			t.Fatalf("export=%+v err=%v", value, err)
		}
	}
	if err := admin.DB.Table("user_models").Where("id = ?", owner).Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ReadExportProfile(context.Background(), owner); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("inactive export=%v", err)
	}
	if _, err := runtime.ReadExportProfile(context.Background(), ""); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("unscoped export=%v", err)
	}
}
