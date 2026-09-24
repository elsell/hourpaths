package path

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewTrimsAndPreservesAValidUnicodeName(t *testing.T) {
	entity, err := New("path-1", "user-1", Attributes{
		Name:       "  Guitarra 🎸 練習  ",
		Visibility: "private",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if entity.Name != "Guitarra 🎸 練習" {
		t.Fatalf("New() name = %q, want trimmed printable Unicode name", entity.Name)
	}
	if entity.Visibility != "private" {
		t.Fatalf("New() visibility = %q, want supplied visibility preserved", entity.Visibility)
	}
}

func TestSetVisibilityPreservesPathStateAndRejectsArchivedOrUnknownVisibility(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	path := Entity{ID: "path-1", OwnerUserID: "creator", Attributes: Attributes{Name: "Read", Visibility: "private"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	updated, err := path.SetVisibility("followers", now)
	if err != nil || updated.Visibility != "followers" || updated.Name != path.Name || updated.CreatedAt != path.CreatedAt || updated.UpdatedAt != now {
		t.Fatalf("SetVisibility()=%+v err=%v", updated, err)
	}
	if _, err := path.SetVisibility("friends", now); err == nil {
		t.Fatal("unknown visibility accepted")
	}
	path.ArchivedAt = now.Add(-time.Second)
	if _, err := path.SetVisibility("followers", now); err == nil {
		t.Fatal("archived Path visibility changed")
	}
}

func TestNewAcceptsPathNameCharacterBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		pathName string
	}{
		{name: "one character", pathName: "🎸"},
		{name: "one hundred characters", pathName: strings.Repeat("練", 100)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New("path-1", "user-1", Attributes{Name: test.pathName, Visibility: "private"}); err != nil {
				t.Fatalf("New() error = %v", err)
			}
		})
	}
}

func TestNewRejectsInvalidPathNames(t *testing.T) {
	tests := []struct {
		name     string
		pathName string
	}{
		{name: "empty", pathName: ""},
		{name: "whitespace only", pathName: " \t\u2003 "},
		{name: "one hundred and one characters", pathName: strings.Repeat("🎸", 101)},
		{name: "line feed", pathName: "Morning\npractice"},
		{name: "carriage return", pathName: "Morning\rpractice"},
		{name: "tab", pathName: "Morning\tpractice"},
		{name: "null control", pathName: "Morning\x00practice"},
		{name: "Unicode line separator", pathName: "Morning\u2028practice"},
		{name: "Unicode formatting control", pathName: "Morning\u200bpractice"},
		{name: "invalid UTF-8", pathName: string([]byte{'M', 0xff})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New("path-1", "user-1", Attributes{Name: test.pathName, Visibility: "private"}); err != ErrInvalidFields {
				t.Fatalf("New() error = %v, want %v", err, ErrInvalidFields)
			}
		})
	}
}

func TestNewDoesNotRequirePathNamesToBeUnique(t *testing.T) {
	first, firstErr := New("path-1", "user-1", Attributes{Name: "Guitar", Visibility: "private"})
	second, secondErr := New("path-2", "user-1", Attributes{Name: "Guitar", Visibility: "private"})

	if firstErr != nil || secondErr != nil {
		t.Fatalf("New() errors = (%v, %v), want duplicate names accepted", firstErr, secondErr)
	}
	if first.ID == second.ID || first.Name != second.Name {
		t.Fatalf("duplicate-name paths = (%+v, %+v), want distinct paths with the same name", first, second)
	}
}

func TestArchivedPathRejectsGoalReconfiguration(t *testing.T) {
	archivedAt := time.Date(2026, 7, 23, 16, 0, 0, 0, time.UTC)
	entity := Entity{
		ID: "path-1", OwnerUserID: "user-1",
		Attributes: Attributes{Name: "Practice", Visibility: "private"},
		CreatedAt:  archivedAt.Add(-time.Hour), UpdatedAt: archivedAt, ArchivedAt: archivedAt,
	}

	if _, err := entity.ReconfigureGoals(
		IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: RecurrenceDaily, Alignment: GoalAlignment{Hour: 0}},
		OverallTarget{},
		archivedAt.Add(time.Minute),
	); err != ErrInvalidState {
		t.Fatalf("ReconfigureGoals() error = %v, want %v", err, ErrInvalidState)
	}
}

func TestNewAcceptsIndependentlyOptionalGoals(t *testing.T) {
	interval := IntervalGoal{
		Present:       true,
		TargetSeconds: 600,
		Recurrence:    RecurrenceDaily,
		Alignment:     GoalAlignment{Hour: 6},
	}
	overall := OverallTarget{Present: true, TargetSeconds: 36_000_000}
	tests := []struct {
		name          string
		intervalGoal  IntervalGoal
		overallTarget OverallTarget
	}{
		{name: "neither"},
		{name: "interval only", intervalGoal: interval},
		{name: "overall only", overallTarget: overall},
		{name: "both", intervalGoal: interval, overallTarget: overall},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attributes := Attributes{
				Name:          "Practice",
				Visibility:    "private",
				IntervalGoal:  test.intervalGoal,
				OverallTarget: test.overallTarget,
			}
			entity, err := New("path-1", "user-1", attributes)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if entity.Attributes != attributes {
				t.Fatalf("New() attributes = %+v, want value-equal %+v", entity.Attributes, attributes)
			}
		})
	}
}

func TestNewAcceptsCanonicalIntervalGoalRecurrencesAndAlignments(t *testing.T) {
	tests := []struct {
		name       string
		recurrence Recurrence
		alignment  GoalAlignment
	}{
		{name: "hourly minute zero", recurrence: RecurrenceHourly, alignment: GoalAlignment{Minute: 0}},
		{name: "hourly minute fifty nine", recurrence: RecurrenceHourly, alignment: GoalAlignment{Minute: 59}},
		{name: "daily hour zero", recurrence: RecurrenceDaily, alignment: GoalAlignment{Hour: 0}},
		{name: "daily hour twenty three", recurrence: RecurrenceDaily, alignment: GoalAlignment{Hour: 23}},
		{name: "weekly ISO Monday", recurrence: RecurrenceWeekly, alignment: GoalAlignment{ISOWeekday: 1}},
		{name: "weekly ISO Sunday", recurrence: RecurrenceWeekly, alignment: GoalAlignment{ISOWeekday: 7}},
		{name: "monthly first", recurrence: RecurrenceMonthly, alignment: GoalAlignment{Day: 1}},
		{name: "monthly thirty first", recurrence: RecurrenceMonthly, alignment: GoalAlignment{Day: 31}},
		{name: "yearly January first", recurrence: RecurrenceYearly, alignment: GoalAlignment{Month: 1, Day: 1}},
		{name: "yearly February twenty ninth", recurrence: RecurrenceYearly, alignment: GoalAlignment{Month: 2, Day: 29}},
		{name: "yearly December thirty first", recurrence: RecurrenceYearly, alignment: GoalAlignment{Month: 12, Day: 31}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New("path-1", "user-1", Attributes{
				Name:       "Practice",
				Visibility: "private",
				IntervalGoal: IntervalGoal{
					Present:       true,
					TargetSeconds: 1,
					Recurrence:    test.recurrence,
					Alignment:     test.alignment,
				},
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
		})
	}
}

func TestNewRejectsInvalidOrPartialGoals(t *testing.T) {
	validInterval := IntervalGoal{
		Present:       true,
		TargetSeconds: 1,
		Recurrence:    RecurrenceDaily,
		Alignment:     GoalAlignment{Hour: 0},
	}
	tests := []struct {
		name          string
		intervalGoal  IntervalGoal
		overallTarget OverallTarget
	}{
		{name: "interval fields without presence", intervalGoal: IntervalGoal{TargetSeconds: 1}},
		{name: "interval missing target", intervalGoal: IntervalGoal{Present: true, Recurrence: RecurrenceDaily}},
		{name: "interval negative target", intervalGoal: IntervalGoal{Present: true, TargetSeconds: -1, Recurrence: RecurrenceDaily}},
		{name: "interval missing recurrence", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1}},
		{name: "interval custom recurrence", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: "fortnightly"}},
		{name: "hourly minute below range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceHourly, Alignment: GoalAlignment{Minute: -1}}},
		{name: "hourly minute above range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceHourly, Alignment: GoalAlignment{Minute: 60}}},
		{name: "daily hour below range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceDaily, Alignment: GoalAlignment{Hour: -1}}},
		{name: "daily hour above range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceDaily, Alignment: GoalAlignment{Hour: 24}}},
		{name: "weekly weekday below range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceWeekly, Alignment: GoalAlignment{ISOWeekday: 0}}},
		{name: "weekly weekday above range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceWeekly, Alignment: GoalAlignment{ISOWeekday: 8}}},
		{name: "monthly day below range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceMonthly, Alignment: GoalAlignment{Day: 0}}},
		{name: "monthly day above range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceMonthly, Alignment: GoalAlignment{Day: 32}}},
		{name: "yearly month below range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceYearly, Alignment: GoalAlignment{Month: 0, Day: 1}}},
		{name: "yearly month above range", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceYearly, Alignment: GoalAlignment{Month: 13, Day: 1}}},
		{name: "yearly invalid calendar day", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceYearly, Alignment: GoalAlignment{Month: 4, Day: 31}}},
		{name: "noncanonical hourly alignment", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceHourly, Alignment: GoalAlignment{Minute: 5, Hour: 1}}},
		{name: "noncanonical daily alignment", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceDaily, Alignment: GoalAlignment{Hour: 5, Day: 1}}},
		{name: "noncanonical weekly alignment", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceWeekly, Alignment: GoalAlignment{ISOWeekday: 1, Hour: 1}}},
		{name: "noncanonical monthly alignment", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceMonthly, Alignment: GoalAlignment{Day: 1, Month: 1}}},
		{name: "noncanonical yearly alignment", intervalGoal: IntervalGoal{Present: true, TargetSeconds: 1, Recurrence: RecurrenceYearly, Alignment: GoalAlignment{Month: 1, Day: 1, Minute: 1}}},
		{name: "overall fields without presence", overallTarget: OverallTarget{TargetSeconds: 1}},
		{name: "overall missing target", overallTarget: OverallTarget{Present: true}},
		{name: "overall negative target", overallTarget: OverallTarget{Present: true, TargetSeconds: -1}},
		{name: "invalid overall does not invalidate interval absence semantics", intervalGoal: validInterval, overallTarget: OverallTarget{TargetSeconds: 1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New("path-1", "user-1", Attributes{
				Name:          "Practice",
				Visibility:    "private",
				IntervalGoal:  test.intervalGoal,
				OverallTarget: test.overallTarget,
			})
			if err != ErrInvalidFields {
				t.Fatalf("New() error = %v, want %v", err, ErrInvalidFields)
			}
		})
	}
}

func TestReconfigureGoalsExactlyReplacesOrRemovesGoalsWithoutChangingUnrelatedPathState(t *testing.T) {
	createdAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	previousUpdate := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)
	reconfiguredAt := time.Date(2026, 7, 22, 15, 30, 0, 0, time.UTC)
	original := Entity{
		ID:          "path-1",
		OwnerUserID: "creator-1",
		Attributes: Attributes{
			Name:       "Guitar practice",
			Visibility: "followers",
			IntervalGoal: IntervalGoal{
				Present:       true,
				TargetSeconds: 3_600,
				Recurrence:    RecurrenceWeekly,
				Alignment:     GoalAlignment{ISOWeekday: 1},
			},
			OverallTarget: OverallTarget{Present: true, TargetSeconds: 36_000},
		},
		CreatedAt: createdAt,
		UpdatedAt: previousUpdate,
	}
	replacementInterval := IntervalGoal{
		Present:       true,
		TargetSeconds: 1_800,
		Recurrence:    RecurrenceDaily,
		Alignment:     GoalAlignment{Hour: 6},
	}
	replacementOverall := OverallTarget{Present: true, TargetSeconds: 18_000}

	for _, test := range []struct {
		name          string
		intervalGoal  IntervalGoal
		overallTarget OverallTarget
	}{
		{name: "replace both goals", intervalGoal: replacementInterval, overallTarget: replacementOverall},
		{name: "remove both goals"},
		{name: "remove interval and replace overall", overallTarget: replacementOverall},
		{name: "replace interval and remove overall", intervalGoal: replacementInterval},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := original.ReconfigureGoals(test.intervalGoal, test.overallTarget, reconfiguredAt)
			if err != nil {
				t.Fatalf("ReconfigureGoals() error = %v", err)
			}
			if got.ID != original.ID || got.OwnerUserID != original.OwnerUserID || got.Name != original.Name ||
				got.Visibility != original.Visibility || got.CreatedAt != createdAt {
				t.Fatalf("unrelated or immutable Path state changed: got=%+v original=%+v", got, original)
			}
			if got.IntervalGoal != test.intervalGoal || got.OverallTarget != test.overallTarget {
				t.Fatalf("goals = (%+v, %+v), want exact (%+v, %+v)", got.IntervalGoal, got.OverallTarget, test.intervalGoal, test.overallTarget)
			}
			if got.UpdatedAt != reconfiguredAt {
				t.Fatalf("UpdatedAt = %v, want %v", got.UpdatedAt, reconfiguredAt)
			}
			if original.UpdatedAt != previousUpdate || original.IntervalGoal.TargetSeconds != 3_600 || original.OverallTarget.TargetSeconds != 36_000 {
				t.Fatalf("original Path was mutated: %+v", original)
			}
		})
	}
}

func TestArchiveAndUnarchivePreservePathConfigurationAndRecordExactLifecycleInstants(t *testing.T) {
	createdAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	archivedAt := time.Date(2026, 7, 23, 15, 0, 0, 123_456_000, time.UTC)
	unarchivedAt := archivedAt.Add(2 * time.Hour)
	original := Entity{
		ID:          "path-archive",
		OwnerUserID: "creator-1",
		Attributes: Attributes{
			Name:          "Guitar practice",
			Visibility:    "followers",
			IntervalGoal:  IntervalGoal{Present: true, TargetSeconds: 3_600, Recurrence: RecurrenceDaily, Alignment: GoalAlignment{Hour: 6}},
			OverallTarget: OverallTarget{Present: true, TargetSeconds: 36_000},
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	archived, err := original.Archive(archivedAt)
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if !archived.Archived() || archived.ArchivedAt != archivedAt || archived.UpdatedAt != archivedAt {
		t.Fatalf("archived lifecycle = %+v, want exact archive instant", archived)
	}
	if archived.ID != original.ID || archived.OwnerUserID != original.OwnerUserID || archived.Attributes != original.Attributes || archived.CreatedAt != original.CreatedAt {
		t.Fatalf("Archive() changed retained configuration: got=%+v original=%+v", archived, original)
	}
	if original.Archived() || !original.ArchivedAt.IsZero() || original.UpdatedAt != updatedAt {
		t.Fatalf("Archive() mutated original = %+v", original)
	}

	active, err := archived.Unarchive(unarchivedAt)
	if err != nil {
		t.Fatalf("Unarchive() error = %v", err)
	}
	if active.Archived() || !active.ArchivedAt.IsZero() || active.UpdatedAt != unarchivedAt {
		t.Fatalf("unarchived lifecycle = %+v, want active at exact instant", active)
	}
	if active.ID != original.ID || active.OwnerUserID != original.OwnerUserID || active.Attributes != original.Attributes || active.CreatedAt != original.CreatedAt {
		t.Fatalf("Unarchive() changed retained configuration: got=%+v original=%+v", active, original)
	}
}

func TestArchiveAndUnarchiveRejectInvalidLifecycleTransitions(t *testing.T) {
	createdAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	original := Entity{
		ID: "path-archive", OwnerUserID: "creator-1",
		Attributes: Attributes{Name: "Practice", Visibility: "private"},
		CreatedAt:  createdAt, UpdatedAt: updatedAt,
	}
	archived, err := original.Archive(updatedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("Archive() setup error = %v", err)
	}

	for _, test := range []struct {
		name string
		call func() (Entity, error)
	}{
		{name: "zero archive instant", call: func() (Entity, error) { return original.Archive(time.Time{}) }},
		{name: "archive before current update", call: func() (Entity, error) { return original.Archive(updatedAt.Add(-time.Nanosecond)) }},
		{name: "archive already archived", call: func() (Entity, error) { return archived.Archive(updatedAt.Add(2 * time.Hour)) }},
		{name: "unarchive active", call: func() (Entity, error) { return original.Unarchive(updatedAt.Add(time.Hour)) }},
		{name: "unarchive before archive", call: func() (Entity, error) { return archived.Unarchive(updatedAt) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, transitionErr := test.call()
			if transitionErr != ErrInvalidState || got != (Entity{}) {
				t.Fatalf("transition = (%+v, %v), want zero Entity and %v", got, transitionErr, ErrInvalidState)
			}
		})
	}
}

func TestRenameActivePathAppliesCreationNameRulesAndPreservesAllOtherState(t *testing.T) {
	now := time.Date(2026, 7, 24, 20, 0, 0, 0, time.UTC)
	entity := Entity{
		ID: "path-1", OwnerUserID: "creator",
		Attributes: Attributes{
			Name: "Before", Visibility: "followers",
			OverallTarget: OverallTarget{Present: true, TargetSeconds: 36_000},
		},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute),
	}

	renamed, err := entity.Rename("  音楽 🎸  ", now)
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if renamed.Name != "音楽 🎸" || renamed.UpdatedAt != now {
		t.Fatalf("Rename() = %+v", renamed)
	}
	if renamed.ID != entity.ID || renamed.OwnerUserID != entity.OwnerUserID ||
		renamed.Visibility != entity.Visibility || renamed.OverallTarget != entity.OverallTarget ||
		renamed.CreatedAt != entity.CreatedAt || !renamed.ArchivedAt.IsZero() {
		t.Fatalf("Rename() changed unrelated state: before=%+v after=%+v", entity, renamed)
	}

	for _, name := range []string{"", " \t ", "line\nbreak", strings.Repeat("x", 101)} {
		if _, err := entity.Rename(name, now); !errors.Is(err, ErrInvalidFields) {
			t.Fatalf("Rename(%q) error = %v, want invalid fields", name, err)
		}
	}
	archived := entity
	archived.ArchivedAt = now.Add(-time.Second)
	if _, err := archived.Rename("Later", now); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("archived Rename() error = %v, want invalid state", err)
	}
}
