package path

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestConfirmedGoalReconfigurationAuthorizesManagersAndExactlyReplacesOrRemovesGoals(t *testing.T) {
	createdAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	previousUpdate := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)
	existing := domain.Entity{
		ID:          "shared-path",
		OwnerUserID: "creator",
		Attributes: domain.Attributes{
			Name:       "Guitar practice",
			Visibility: "followers",
			IntervalGoal: domain.IntervalGoal{
				Present:       true,
				TargetSeconds: 3_600,
				Recurrence:    domain.RecurrenceWeekly,
				Alignment:     domain.GoalAlignment{ISOWeekday: 1},
			},
			OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 36_000},
		},
		CreatedAt: createdAt,
		UpdatedAt: previousUpdate,
	}
	replacementInterval := domain.IntervalGoal{
		Present:       true,
		TargetSeconds: 1_800,
		Recurrence:    domain.RecurrenceDaily,
		Alignment:     domain.GoalAlignment{Hour: 6},
	}
	replacementOverall := domain.OverallTarget{Present: true, TargetSeconds: 18_000}

	for _, actor := range []string{"creator", "administrator"} {
		for _, change := range []struct {
			name          string
			intervalGoal  domain.IntervalGoal
			overallTarget domain.OverallTarget
		}{
			{name: "replace both", intervalGoal: replacementInterval, overallTarget: replacementOverall},
			{name: "remove both"},
		} {
			t.Run(actor+"/"+change.name, func(t *testing.T) {
				var checks []authorizationCall
				var gets []repositoryGet
				var updates []UpdateGoalsCommand
				var timeZoneCalls []string
				dependencies := configuredDependencies()
				dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: actor, Scopes: []string{"api:user"}}}
				dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
				dependencies.Profiles = controlledProfiles{timeZone: "Etc/UTC", timeZoneCalls: &timeZoneCalls}
				wantPath := existing
				wantPath.IntervalGoal = change.intervalGoal
				wantPath.OverallTarget = change.overallTarget
				wantPath.UpdatedAt = dependencies.Clock.Now()
				wantProgress := &GoalIntervalProgress{
					TargetSeconds:      change.intervalGoal.TargetSeconds,
					AccumulatedSeconds: 45,
					StartedAt:          dependencies.Clock.Now().Add(-12 * time.Hour),
					EndedAt:            dependencies.Clock.Now().Add(12 * time.Hour),
				}
				if !change.intervalGoal.Present {
					wantProgress = nil
				}
				wantTimeZone := ""
				if change.intervalGoal.Present {
					wantTimeZone = "Etc/UTC"
				}
				dependencies.Repository = controlledRepository{
					entity: existing, gets: &gets, goalUpdates: &updates,
					goalResult: UpdateGoalsResult{Path: wantPath, AccumulatedSeconds: 45, IntervalProgress: wantProgress},
				}

				got, err := New(dependencies).UpdateGoals(
					context.Background(),
					"Bearer valid",
					"goal-update-key-0001",
					existing.ID,
					true,
					domain.GoalConfiguration{IntervalGoal: existing.IntervalGoal, OverallTarget: existing.OverallTarget},
					change.intervalGoal,
					change.overallTarget,
				)
				if err != nil {
					t.Fatalf("UpdateGoals() error = %v", err)
				}
				if checks != nil && (len(checks) != 1 || checks[0] != (authorizationCall{"path", string(existing.ID), "manage_goals", actor})) {
					t.Fatalf("authorization checks = %+v", checks)
				}
				if len(gets) != 1 || gets[0] != (repositoryGet{userID: actor, id: existing.ID}) {
					t.Fatalf("Path reads = %+v", gets)
				}
				if len(updates) != 1 {
					t.Fatalf("repository updates = %+v", updates)
				}
				if got != dependencies.Repository.(controlledRepository).goalResult {
					t.Fatalf("UpdateGoals() = %+v, want authoritative repository result %+v", got, dependencies.Repository.(controlledRepository).goalResult)
				}
				command := updates[0]
				if command.ActorUserID != actor || command.ParticipantTimeZone != wantTimeZone ||
					command.ExpectedGoals != (domain.GoalConfiguration{IntervalGoal: existing.IntervalGoal, OverallTarget: existing.OverallTarget}) ||
					command.Path != wantPath || !command.ProjectedAt.Equal(dependencies.Clock.Now()) {
					t.Fatalf("atomic goal command = %+v, want actor=%q zone=%q Path=%+v projectedAt=%v", command, actor, wantTimeZone, wantPath, dependencies.Clock.Now())
				}
				if change.intervalGoal.Present && (len(timeZoneCalls) != 1 || timeZoneCalls[0] != actor) {
					t.Fatalf("participant time-zone reads = %+v", timeZoneCalls)
				}
				if !change.intervalGoal.Present && len(timeZoneCalls) != 0 {
					t.Fatalf("goal removal unnecessarily read participant time zone: %+v", timeZoneCalls)
				}
				if command.Idempotency.PrincipalID != actor || command.Idempotency.Operation != UpdateGoalsOperation ||
					command.Idempotency.Key != "goal-update-key-0001" || len(command.Idempotency.RequestHash) != 32 {
					t.Fatalf("goal idempotency = %+v", command.Idempotency)
				}
				event := command.Audit
				if event.Action != audit.ResourceUpdated || event.Outcome != audit.Succeeded ||
					event.TargetType != "path" || event.TargetID != string(existing.ID) ||
					event.OwnerUserID != existing.OwnerUserID || event.ActorUserID != actor ||
					!event.OccurredAt.Equal(dependencies.Clock.Now()) {
					t.Fatalf("atomic update audit = %+v", event)
				}
				if got.Path.ID != existing.ID || got.Path.OwnerUserID != existing.OwnerUserID || got.Path.Name != existing.Name ||
					got.Path.Visibility != existing.Visibility || got.Path.CreatedAt != existing.CreatedAt {
					t.Fatalf("unrelated or immutable Path state changed: got=%+v existing=%+v", got.Path, existing)
				}
			})
		}
	}
}

func TestGoalReconfigurationRequiresExplicitConfirmationBeforeAuthorizationOrMutation(t *testing.T) {
	var checks []authorizationCall
	var gets []repositoryGet
	var updates []UpdateGoalsCommand
	var events []audit.Event
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	dependencies.Repository = controlledRepository{gets: &gets, goalUpdates: &updates}
	dependencies.Audits = controlledAudits{events: &events}

	_, err := New(dependencies).UpdateGoals(
		context.Background(),
		"Bearer valid",
		"goal-update-key-0001",
		"path-id",
		false,
		domain.GoalConfiguration{},
		domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 0}},
		domain.OverallTarget{},
	)
	if !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("unconfirmed UpdateGoals() error = %v, want invalid argument", err)
	}
	if len(checks) != 0 || len(gets) != 0 || len(updates) != 0 || len(events) != 0 {
		t.Fatalf("unconfirmed change reached dependencies: checks=%+v gets=%+v updates=%+v audits=%+v", checks, gets, updates, events)
	}
}

func TestGoalReconfigurationRejectsArchivedPathBeforeMutation(t *testing.T) {
	var updates []UpdateGoalsCommand
	dependencies := configuredDependencies()
	archivedAt := dependencies.Clock.Now().Add(-time.Minute)
	existing := domain.Entity{
		ID: "archived-path", OwnerUserID: "creator",
		Attributes: domain.Attributes{Name: "Retained practice", Visibility: "private"},
		CreatedAt:  archivedAt.Add(-time.Hour), UpdatedAt: archivedAt, ArchivedAt: archivedAt,
	}
	dependencies.Authorizer = controlledAuthorizer{allowed: true}
	dependencies.Repository = controlledRepository{entity: existing, goalUpdates: &updates}

	_, err := New(dependencies).UpdateGoals(
		context.Background(),
		"Bearer valid",
		"archived-goal-key-0001",
		existing.ID,
		true,
		existing.GoalConfiguration(),
		domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 0}},
		domain.OverallTarget{},
	)
	if !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("archived UpdateGoals() error = %v, want conflict", err)
	}
	if len(updates) != 0 {
		t.Fatalf("archived goal change reached persistence: %+v", updates)
	}
}

func TestGoalReconfigurationRejectsInvalidIdempotencyKeysBeforeAuthorizationOrMutation(t *testing.T) {
	for _, key := range []string{"", "too-short", "goal-update-key-\x1f", strings.Repeat("x", 129)} {
		t.Run(key, func(t *testing.T) {
			var checks []authorizationCall
			var gets []repositoryGet
			var updates []UpdateGoalsCommand
			var events []audit.Event
			dependencies := configuredDependencies()
			dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
			dependencies.Repository = controlledRepository{gets: &gets, goalUpdates: &updates}
			dependencies.Audits = controlledAudits{events: &events}

			_, err := New(dependencies).UpdateGoals(
				context.Background(), "Bearer valid", key, "path-id", true,
				domain.GoalConfiguration{},
				domain.IntervalGoal{}, domain.OverallTarget{},
			)
			if !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("UpdateGoals() error = %v, want invalid argument", err)
			}
			if len(checks) != 0 || len(gets) != 0 || len(updates) != 0 || len(events) != 0 {
				t.Fatalf("invalid key reached dependencies: checks=%+v gets=%+v updates=%+v audits=%+v", checks, gets, updates, events)
			}
		})
	}
}

func TestGoalReconfigurationRejectsMalformedReviewedConfigurationBeforeAuthorizationOrMutation(t *testing.T) {
	var checks []authorizationCall
	var gets []repositoryGet
	var updates []UpdateGoalsCommand
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	dependencies.Repository = controlledRepository{gets: &gets, goalUpdates: &updates}

	_, err := New(dependencies).UpdateGoals(
		context.Background(), "Bearer valid", "goal-update-key-0001", "path-id", true,
		domain.GoalConfiguration{IntervalGoal: domain.IntervalGoal{TargetSeconds: 60}},
		domain.IntervalGoal{}, domain.OverallTarget{},
	)
	if !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("UpdateGoals() error = %v, want invalid argument", err)
	}
	if len(checks) != 0 || len(gets) != 0 || len(updates) != 0 {
		t.Fatalf("malformed reviewed goals reached dependencies: checks=%+v gets=%+v updates=%+v", checks, gets, updates)
	}
}

func TestDeniedGoalReconfigurationAuditsDenialBeforePathReadOrMutation(t *testing.T) {
	var checks []authorizationCall
	var gets []repositoryGet
	var updates []UpdateGoalsCommand
	var events []audit.Event
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{allowed: false, calls: &checks}
	dependencies.Repository = controlledRepository{gets: &gets, goalUpdates: &updates}
	dependencies.Audits = controlledAudits{events: &events}

	_, err := New(dependencies).UpdateGoals(
		context.Background(),
		"Bearer valid",
		"goal-update-key-0001",
		"secret-path",
		true,
		domain.GoalConfiguration{},
		domain.IntervalGoal{},
		domain.OverallTarget{},
	)
	if !errors.Is(err, platformapp.ErrForbidden) {
		t.Fatalf("denied UpdateGoals() error = %v, want forbidden", err)
	}
	if len(checks) != 1 || checks[0] != (authorizationCall{"path", "secret-path", "manage_goals", "member"}) {
		t.Fatalf("authorization checks = %+v", checks)
	}
	if len(gets) != 0 || len(updates) != 0 {
		t.Fatalf("denied change reached Path persistence: gets=%+v updates=%+v", gets, updates)
	}
	if len(events) != 1 {
		t.Fatalf("denial audits = %+v", events)
	}
	event := events[0]
	if event.Action != audit.ResourceAccessDenied || event.Outcome != audit.Denied ||
		event.TargetType != "path" || event.TargetID != "secret-path" ||
		event.OwnerUserID != "member" || event.ActorUserID != "member" {
		t.Fatalf("denial audit = %+v", event)
	}
}

func TestGoalReconfigurationIdempotencyHashBindsPathAndExactGoalConfiguration(t *testing.T) {
	interval := domain.IntervalGoal{
		Present:       true,
		TargetSeconds: 1_800,
		Recurrence:    domain.RecurrenceDaily,
		Alignment:     domain.GoalAlignment{Hour: 6},
	}
	overall := domain.OverallTarget{Present: true, TargetSeconds: 18_000}
	expected := domain.GoalConfiguration{
		IntervalGoal: domain.IntervalGoal{
			Present: true, TargetSeconds: 3_600, Recurrence: domain.RecurrenceWeekly,
			Alignment: domain.GoalAlignment{ISOWeekday: 1},
		},
		OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 36_000},
	}
	baseline := canonicalUpdateGoalsRequestHash("path-1", expected, interval, overall)
	for _, test := range []struct {
		name          string
		pathID        domain.ID
		expected      domain.GoalConfiguration
		intervalGoal  domain.IntervalGoal
		overallTarget domain.OverallTarget
	}{
		{name: "different Path", pathID: "path-2", expected: expected, intervalGoal: interval, overallTarget: overall},
		{name: "different reviewed interval", pathID: "path-1", expected: domain.GoalConfiguration{OverallTarget: expected.OverallTarget}, intervalGoal: interval, overallTarget: overall},
		{name: "different reviewed overall", pathID: "path-1", expected: domain.GoalConfiguration{IntervalGoal: expected.IntervalGoal}, intervalGoal: interval, overallTarget: overall},
		{name: "removed interval", pathID: "path-1", expected: expected, overallTarget: overall},
		{name: "changed interval target", pathID: "path-1", expected: expected, intervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 1_801, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 6}}, overallTarget: overall},
		{name: "changed recurrence and alignment", pathID: "path-1", expected: expected, intervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 1_800, Recurrence: domain.RecurrenceWeekly, Alignment: domain.GoalAlignment{ISOWeekday: 6}}, overallTarget: overall},
		{name: "removed overall", pathID: "path-1", expected: expected, intervalGoal: interval},
		{name: "changed overall target", pathID: "path-1", expected: expected, intervalGoal: interval, overallTarget: domain.OverallTarget{Present: true, TargetSeconds: 18_001}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := canonicalUpdateGoalsRequestHash(test.pathID, test.expected, test.intervalGoal, test.overallTarget); got == baseline {
				t.Fatal("materially different goal request retained the same idempotency hash")
			}
		})
	}
}
