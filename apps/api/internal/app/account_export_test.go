package app

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type exportProfileRepository struct {
	profile AccountExportProfile
	owners  []string
}

func (r *exportProfileRepository) ReadExportProfile(_ context.Context, owner string) (AccountExportProfile, error) {
	r.owners = append(r.owners, owner)
	return r.profile, nil
}
func TestAccountExportProfileBindsOwnerAndFailsClosedBeforeDisclosingData(t *testing.T) {
	now := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	repo := &exportProfileRepository{profile: AccountExportProfile{UserID: "owner", Email: "own@example.test", TimeZone: "Etc/UTC", FirstDayOfWeek: 7, CreatedAt: now, UpdatedAt: now}}
	app := timeZoneTestApp(nil)
	app.AccountExport = repo
	var events []audit.Event
	app.Audits = fakeAudits{events: &events}
	result, err := app.ExportOwnAccountProfile(context.Background(), "Bearer session")
	if err != nil || result.UserID != "owner" || len(repo.owners) != 1 || repo.owners[0] != "owner" || len(events) != 1 || events[0].TargetType != "account_export" {
		t.Fatalf("export=%+v err=%v owners=%v audits=%v", result, err, repo.owners, events)
	}
	repo.profile.UserID = "other"
	if result, err = app.ExportOwnAccountProfile(context.Background(), "Bearer session"); !errors.Is(err, ports.ErrUnavailable) || result.UserID != "" {
		t.Fatalf("foreign row leaked=%+v err=%v", result, err)
	}
	repo.profile.UserID = "owner"
	app.Audits = fakeAudits{err: ports.ErrUnavailable}
	if result, err = app.ExportOwnAccountProfile(context.Background(), "Bearer session"); err == nil || result.UserID != "" {
		t.Fatal("unaudited export disclosed")
	}
	app.Auth = fakeAuth{err: ports.ErrInvalidCredential}
	before := len(repo.owners)
	if _, err = app.ExportOwnAccountProfile(context.Background(), "Bearer expired"); err == nil || len(repo.owners) != before {
		t.Fatal("invalid session reached export store")
	}
}
