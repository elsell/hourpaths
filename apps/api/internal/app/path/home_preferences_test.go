package path

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type homeRepository struct {
	controlledRepository
	projection HomeOrganizationProjection
	command    *UpdateHomePreferencesCommand
	result     domain.HomePreferences
	homeErr    error
}

type advancingHomeClock struct {
	now   time.Time
	calls int
}

func (clock *advancingHomeClock) Now() time.Time {
	value := clock.now.Add(time.Duration(clock.calls) * time.Nanosecond)
	clock.calls++
	return value
}

func (f controlledRepository) ProjectHome(_ context.Context, _ string, pathIDs []domain.ID) (HomeOrganizationProjection, error) {
	if f.err != nil {
		return HomeOrganizationProjection{}, f.err
	}
	projection := HomeOrganizationProjection{Preferences: domain.HomePreferences{OrderMethod: domain.HomeOrderRecent}, Organization: map[domain.ID]HomeOrganization{}}
	for _, id := range pathIDs {
		projection.Organization[id] = HomeOrganization{PathID: id, Classification: HomeSolo}
	}
	return projection, nil
}
func (f controlledRepository) UpdateHomePreferences(_ context.Context, command UpdateHomePreferencesCommand) (domain.HomePreferences, error) {
	if f.err != nil {
		return domain.HomePreferences{}, f.err
	}
	return domain.HomePreferences{OrderMethod: command.OrderMethod, Revision: command.ExpectedRevision + 1, PinnedPathIDs: command.PinnedPathIDs, ManualPathIDs: command.ManualPathIDs, UpdatedAt: command.UpdatedAt}, nil
}

func (repository homeRepository) ProjectHome(context.Context, string, []domain.ID) (HomeOrganizationProjection, error) {
	return repository.projection, repository.homeErr
}
func (repository homeRepository) UpdateHomePreferences(_ context.Context, command UpdateHomePreferencesCommand) (domain.HomePreferences, error) {
	if repository.command != nil {
		*repository.command = command
	}
	return repository.result, repository.homeErr
}

func TestUpdateHomePreferencesBuildsSelfScopedAtomicCommand(t *testing.T) {
	var command UpdateHomePreferencesCommand
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	dependencies := configuredDependencies()
	dependencies.Clock = controlledClock{now: now}
	dependencies.Repository = homeRepository{controlledRepository: controlledRepository{}, command: &command, result: domain.HomePreferences{OrderMethod: domain.HomeOrderManual, Revision: 4, PinnedPathIDs: []domain.ID{"path-2"}, ManualPathIDs: []domain.ID{"path-2", "path-1"}, UpdatedAt: now}}
	result, err := New(dependencies).UpdateHomePreferences(context.Background(), "Bearer valid", "home-order-key-0001", 3, domain.HomeOrderManual, []domain.ID{"path-2"}, []domain.ID{"path-2", "path-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision != 4 || command.ActorUserID != "member" || command.Idempotency.PrincipalID != "member" || command.Idempotency.Operation != UpdateHomePreferencesOperation || len(command.Idempotency.RequestHash) != 32 || command.Audit.TargetType != "home_preferences" || command.Audit.TargetID != "member" {
		t.Fatalf("result=%+v command=%+v", result, command)
	}
}

func TestUpdateHomePreferencesUsesOneTimestampForMutationAndAudit(t *testing.T) {
	var command UpdateHomePreferencesCommand
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	clock := &advancingHomeClock{now: now}
	dependencies := configuredDependencies()
	dependencies.Clock = clock
	dependencies.Repository = homeRepository{
		controlledRepository: controlledRepository{},
		command:              &command,
		result:               domain.HomePreferences{OrderMethod: domain.HomeOrderRecent, Revision: 1, PinnedPathIDs: []domain.ID{"path-1"}, ManualPathIDs: []domain.ID{"path-1"}, UpdatedAt: now},
	}

	if _, err := New(dependencies).UpdateHomePreferences(context.Background(), "Bearer valid", "home-pin-key-0001", 0, domain.HomeOrderRecent, []domain.ID{"path-1"}, []domain.ID{"path-1"}); err != nil {
		t.Fatal(err)
	}
	if clock.calls < 2 {
		t.Fatalf("clock calls=%d, want at least 2 to exercise a real advancing clock", clock.calls)
	}
	if !command.Audit.OccurredAt.Equal(command.UpdatedAt) {
		t.Fatalf("audit occurred at %s, update occurred at %s", command.Audit.OccurredAt, command.UpdatedAt)
	}
}

func TestUpdateHomePreferencesRejectsMalformedAndPropagatesConflicts(t *testing.T) {
	for name, request := range map[string]struct {
		pinned, manual []domain.ID
		method         domain.HomeOrderMethod
		revision       int64
	}{
		"foreign pin":       {[]domain.ID{"other"}, []domain.ID{"path"}, domain.HomeOrderRecent, 0},
		"duplicate manual":  {nil, []domain.ID{"path", "path"}, domain.HomeOrderManual, 0},
		"unknown method":    {nil, []domain.ID{"path"}, "popular", 0},
		"negative revision": {nil, []domain.ID{"path"}, domain.HomeOrderRecent, -1},
	} {
		t.Run(name, func(t *testing.T) {
			dependencies := configuredDependencies()
			if _, err := New(dependencies).UpdateHomePreferences(context.Background(), "Bearer valid", "home-order-key-0001", request.revision, request.method, request.pinned, request.manual); !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	for _, failure := range []error{ports.ErrConflict, ports.ErrIdempotencyConflict} {
		dependencies := configuredDependencies()
		dependencies.Repository = homeRepository{controlledRepository: controlledRepository{}, homeErr: failure}
		if _, err := New(dependencies).UpdateHomePreferences(context.Background(), "Bearer valid", "home-order-key-0001", 0, domain.HomeOrderRecent, nil, nil); !errors.Is(err, failure) {
			t.Fatalf("error=%v want=%v", err, failure)
		}
	}
}

func TestReadHomePreferencesAdmitsDefaultEmptySnapshot(t *testing.T) {
	dependencies := configuredDependencies()
	dependencies.Repository = homeRepository{controlledRepository: controlledRepository{}, projection: HomeOrganizationProjection{Preferences: domain.HomePreferences{OrderMethod: domain.HomeOrderRecent}, Organization: map[domain.ID]HomeOrganization{}}}
	preferences, err := New(dependencies).ReadHomePreferences(context.Background(), "Bearer valid")
	if err != nil || preferences.OrderMethod != domain.HomeOrderRecent || preferences.Revision != 0 {
		t.Fatalf("preferences=%+v error=%v", preferences, err)
	}
}

func TestListProjectedCarriesAuthoritativePersonalHomeOrganization(t *testing.T) {
	created := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 8, 4, 11, 0, 0, 0, time.UTC)
	pinned, manual := int64(0), int64(1)
	dependencies := configuredDependencies()
	dependencies.Repository = homeRepository{
		controlledRepository: controlledRepository{page: Page{Items: []domain.Entity{
			{ID: "shared", OwnerUserID: "other", Attributes: domain.Attributes{Name: "Shared", Visibility: "private"}, CreatedAt: created, UpdatedAt: created},
		}}},
		projection: HomeOrganizationProjection{
			Preferences:  domain.HomePreferences{OrderMethod: domain.HomeOrderRecent},
			Organization: map[domain.ID]HomeOrganization{"shared": {PathID: "shared", Classification: HomeShared, PinnedPosition: &pinned, ManualPosition: &manual, RecentActivityAt: &recent}},
		},
	}
	items, _, err := New(dependencies).ListProjected(context.Background(), "Bearer valid", "", 25)
	if err != nil || len(items) != 1 || items[0].Home.Classification != HomeShared || items[0].Home.PinnedPosition == nil || items[0].Home.RecentActivityAt == nil || !items[0].Home.RecentActivityAt.Equal(recent) {
		t.Fatalf("items=%+v error=%v", items, err)
	}
}

func TestGetProjectedCarriesHomeOrganizationForMemberAndOmitsItForPublicNonMember(t *testing.T) {
	recent := time.Date(2026, 8, 4, 11, 0, 0, 0, time.UTC)
	dependencies := configuredDependencies()
	dependencies.Repository = homeRepository{
		controlledRepository: controlledRepository{entity: domain.Entity{ID: "shared", OwnerUserID: "other", Attributes: domain.Attributes{Name: "Shared", Visibility: "public"}}},
		projection: HomeOrganizationProjection{
			Preferences:  domain.HomePreferences{OrderMethod: domain.HomeOrderRecent},
			Organization: map[domain.ID]HomeOrganization{"shared": {PathID: "shared", Classification: HomeShared, RecentActivityAt: &recent}},
		},
	}
	member, err := New(dependencies).GetProjected(context.Background(), "Bearer valid", "shared")
	if err != nil || member.Home.Classification != HomeShared || member.Home.RecentActivityAt == nil || !member.Home.RecentActivityAt.Equal(recent) {
		t.Fatalf("member=%+v error=%v", member, err)
	}

	dependencies.Repository = homeRepository{
		controlledRepository: controlledRepository{entity: domain.Entity{ID: "public", OwnerUserID: "other", Attributes: domain.Attributes{Name: "Public", Visibility: "public"}}},
		projection:           HomeOrganizationProjection{Preferences: domain.HomePreferences{OrderMethod: domain.HomeOrderRecent}, Organization: map[domain.ID]HomeOrganization{}},
	}
	public, err := New(dependencies).GetProjected(context.Background(), "Bearer valid", "public")
	if err != nil || public.Home.Classification != "" {
		t.Fatalf("public=%+v error=%v", public, err)
	}
}
