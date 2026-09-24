package path

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type idempotencyRepository struct {
	controlledRepository
	requestHash []byte
	stored      domain.Entity
}

func (r *idempotencyRepository) Create(_ context.Context, entity domain.Entity, _ ports.AuthorizationChange, idempotency ports.Idempotency, _ audit.Event) (domain.Entity, bool, error) {
	if r.requestHash == nil {
		r.requestHash = append([]byte(nil), idempotency.RequestHash...)
		r.stored = entity
		return entity, false, nil
	}
	if !bytes.Equal(r.requestHash, idempotency.RequestHash) {
		return domain.Entity{}, false, ports.ErrIdempotencyConflict
	}
	return r.stored, true, nil
}

func TestCreatePathPreservesIndependentlyOptionalGoals(t *testing.T) {
	interval := domain.IntervalGoal{
		Present:       true,
		TargetSeconds: 600,
		Recurrence:    domain.RecurrenceWeekly,
		Alignment:     domain.GoalAlignment{ISOWeekday: 3},
	}
	overall := domain.OverallTarget{Present: true, TargetSeconds: 36_000_000}
	for _, test := range []struct {
		name          string
		intervalGoal  domain.IntervalGoal
		overallTarget domain.OverallTarget
	}{
		{name: "neither"},
		{name: "interval only", intervalGoal: interval},
		{name: "overall only", overallTarget: overall},
		{name: "both", intervalGoal: interval, overallTarget: overall},
	} {
		t.Run(test.name, func(t *testing.T) {
			var creates []repositoryCreate
			dependencies := configuredDependencies()
			dependencies.Repository = controlledRepository{creates: &creates}
			created, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{
				Name:          "Practice",
				IntervalGoal:  test.intervalGoal,
				OverallTarget: test.overallTarget,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(creates) != 1 || created.IntervalGoal != test.intervalGoal || created.OverallTarget != test.overallTarget || creates[0].entity != created {
				t.Fatalf("created goals = %+v, repository creates = %+v", created.Attributes, creates)
			}
		})
	}
}

func TestCreatePathAppliesOnlyCanonicalMissingAlignmentDefaults(t *testing.T) {
	for _, test := range []struct {
		name       string
		recurrence domain.Recurrence
		requested  domain.GoalAlignment
		want       domain.GoalAlignment
	}{
		{name: "hourly default", recurrence: domain.RecurrenceHourly, want: domain.GoalAlignment{Minute: 0}},
		{name: "hourly override", recurrence: domain.RecurrenceHourly, requested: domain.GoalAlignment{Minute: 45}, want: domain.GoalAlignment{Minute: 45}},
		{name: "daily default", recurrence: domain.RecurrenceDaily, want: domain.GoalAlignment{Hour: 0}},
		{name: "daily override", recurrence: domain.RecurrenceDaily, requested: domain.GoalAlignment{Hour: 18}, want: domain.GoalAlignment{Hour: 18}},
		{name: "weekly profile default", recurrence: domain.RecurrenceWeekly, want: domain.GoalAlignment{ISOWeekday: 6}},
		{name: "weekly override", recurrence: domain.RecurrenceWeekly, requested: domain.GoalAlignment{ISOWeekday: 7}, want: domain.GoalAlignment{ISOWeekday: 7}},
		{name: "monthly default", recurrence: domain.RecurrenceMonthly, want: domain.GoalAlignment{Day: 1}},
		{name: "monthly override", recurrence: domain.RecurrenceMonthly, requested: domain.GoalAlignment{Day: 31}, want: domain.GoalAlignment{Day: 31}},
		{name: "yearly default", recurrence: domain.RecurrenceYearly, want: domain.GoalAlignment{Month: 1, Day: 1}},
		{name: "yearly leap-day override", recurrence: domain.RecurrenceYearly, requested: domain.GoalAlignment{Month: 2, Day: 29}, want: domain.GoalAlignment{Month: 2, Day: 29}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var creates []repositoryCreate
			dependencies := configuredDependencies()
			dependencies.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPublic, firstDayOfWeek: identity.FirstDaySaturday}
			dependencies.Repository = controlledRepository{creates: &creates}
			created, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{
				Name: "Practice",
				IntervalGoal: domain.IntervalGoal{
					Present:       true,
					TargetSeconds: 60,
					Recurrence:    test.recurrence,
					Alignment:     test.requested,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(creates) != 1 || created.IntervalGoal.Alignment != test.want {
				t.Fatalf("created alignment = %+v, want %+v", created.IntervalGoal.Alignment, test.want)
			}
		})
	}
}

func TestCreatePathRejectsPartialMismatchedAndInvalidGoalsBeforePersistence(t *testing.T) {
	for _, test := range []struct {
		name       string
		attributes domain.Attributes
	}{
		{name: "interval fields without presence", attributes: domain.Attributes{IntervalGoal: domain.IntervalGoal{TargetSeconds: 1}}},
		{name: "present interval without target", attributes: domain.Attributes{IntervalGoal: domain.IntervalGoal{Present: true, Recurrence: domain.RecurrenceDaily}}},
		{name: "present interval without recurrence", attributes: domain.Attributes{IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 1}}},
		{name: "mismatched alignment", attributes: domain.Attributes{IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Minute: 1}}}},
		{name: "invalid alignment", attributes: domain.Attributes{IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: domain.RecurrenceMonthly, Alignment: domain.GoalAlignment{Day: 32}}}},
		{name: "overall fields without presence", attributes: domain.Attributes{OverallTarget: domain.OverallTarget{TargetSeconds: 1}}},
		{name: "present overall without target", attributes: domain.Attributes{OverallTarget: domain.OverallTarget{Present: true}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var creates []repositoryCreate
			dependencies := configuredDependencies()
			dependencies.Repository = controlledRepository{creates: &creates}
			test.attributes.Name = "Practice"
			_, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", test.attributes)
			if !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("Create() error = %v, want invalid argument", err)
			}
			if len(creates) != 0 {
				t.Fatalf("invalid goals reached repository: %+v", creates)
			}
		})
	}
}

func TestCreatePathReplayReturnsStoredGoalsAcrossProfilePreferenceChanges(t *testing.T) {
	repository := &idempotencyRepository{}
	var stored domain.Entity
	for index, weekday := range []identity.FirstDayOfWeek{identity.FirstDayMonday, identity.FirstDaySunday} {
		dependencies := configuredDependencies()
		dependencies.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPrivate, firstDayOfWeek: weekday}
		dependencies.Repository = repository
		dependencies.AuthorizationOutbox = controlledOutbox{claimErr: ports.ErrNotFound}
		created, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{
			Name:          " Practice ",
			IntervalGoal:  domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceWeekly},
			OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 3600},
		})
		if err != nil {
			t.Fatalf("create/replay %d error = %v", index, err)
		}
		if index == 0 {
			stored = created
			if created.IntervalGoal.Alignment.ISOWeekday != int(identity.FirstDayMonday) {
				t.Fatalf("stored weekly default = %+v", created.IntervalGoal.Alignment)
			}
		} else if created != stored {
			t.Fatalf("preference-changing replay = %+v, want stored %+v", created, stored)
		}
	}

	dependencies := configuredDependencies()
	dependencies.Repository = repository
	dependencies.AuthorizationOutbox = controlledOutbox{claimErr: ports.ErrNotFound}
	_, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{
		Name: "Practice",
		IntervalGoal: domain.IntervalGoal{
			Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceWeekly,
			Alignment: domain.GoalAlignment{ISOWeekday: 7},
		},
		OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 3600},
	})
	if !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("explicit goal change with reused key returned %v", err)
	}
}

func TestCreatePathRequestHashUsesCanonicalRawGoalRequest(t *testing.T) {
	baseline := domain.Attributes{
		Name: " Practice ", Visibility: " followers ",
		IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceDaily},
	}
	if canonicalCreateRequestHash(baseline) != canonicalCreateRequestHash(domain.Attributes{
		Name: "Practice", Visibility: "followers",
		IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceDaily},
	}) {
		t.Fatal("trim-equivalent request changed canonical hash")
	}
	for _, test := range []struct {
		name       string
		attributes domain.Attributes
	}{
		{name: "name", attributes: domain.Attributes{Name: "Read", Visibility: "followers", IntervalGoal: baseline.IntervalGoal}},
		{name: "requested visibility", attributes: domain.Attributes{Name: "Practice", Visibility: "private", IntervalGoal: baseline.IntervalGoal}},
		{name: "interval presence", attributes: domain.Attributes{Name: "Practice", Visibility: "followers"}},
		{name: "interval target", attributes: domain.Attributes{Name: "Practice", Visibility: "followers", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 61, Recurrence: domain.RecurrenceDaily}}},
		{name: "interval recurrence", attributes: domain.Attributes{Name: "Practice", Visibility: "followers", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceHourly}}},
		{name: "alignment minute", attributes: domain.Attributes{Name: "Practice", Visibility: "followers", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceHourly, Alignment: domain.GoalAlignment{Minute: 1}}}},
		{name: "alignment hour", attributes: domain.Attributes{Name: "Practice", Visibility: "followers", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 1}}}},
		{name: "alignment weekday", attributes: domain.Attributes{Name: "Practice", Visibility: "followers", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceWeekly, Alignment: domain.GoalAlignment{ISOWeekday: 1}}}},
		{name: "alignment month and day", attributes: domain.Attributes{Name: "Practice", Visibility: "followers", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceYearly, Alignment: domain.GoalAlignment{Month: 2, Day: 29}}}},
		{name: "overall presence and target", attributes: domain.Attributes{Name: "Practice", Visibility: "followers", IntervalGoal: baseline.IntervalGoal, OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 3600}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if canonicalCreateRequestHash(baseline) == canonicalCreateRequestHash(test.attributes) {
				t.Fatal("explicit request change retained the same hash")
			}
		})
	}
}

func TestCreatePathRejectsMalformedStoredReplayGoals(t *testing.T) {
	stored := domain.Entity{
		ID: "original-path", OwnerUserID: "member",
		Attributes: domain.Attributes{
			Name: "Practice", Visibility: "private",
			IntervalGoal: domain.IntervalGoal{Present: true, Recurrence: domain.RecurrenceDaily},
		},
		CreatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
	}
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{entity: stored, replayed: true}
	if _, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Practice"}); !errors.Is(err, errInvalidPathDependencies) {
		t.Fatalf("malformed stored goals returned %v", err)
	}
}
