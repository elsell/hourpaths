package mapper

import (
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/dto"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestGoalConfigurationsMapsReviewedAndProposedGoalsWithoutPathMetadata(t *testing.T) {
	expected, proposed, err := GoalConfigurations(dto.PathGoalsUpdate{
		Confirmed: true,
		ExpectedGoals: dto.PathGoalConfiguration{
			IntervalGoal: &dto.IntervalGoalItem{
				TargetSeconds: 60,
				Recurrence:    "weekly",
				Alignment:     dto.IntervalAlignment{ISOWeekday: integer(1)},
			},
			OverallTarget: &dto.OverallTarget{TargetSeconds: 7_200},
		},
		IntervalGoal: &dto.IntervalGoalItem{
			TargetSeconds: 30,
			Recurrence:    "yearly",
			Alignment:     dto.IntervalAlignment{Month: integer(2), Day: integer(29)},
		},
		OverallTarget: &dto.OverallTarget{TargetSeconds: 3_600},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantInterval := domain.IntervalGoal{
		Present:       true,
		TargetSeconds: 30,
		Recurrence:    domain.RecurrenceYearly,
		Alignment:     domain.GoalAlignment{Month: 2, Day: 29},
	}
	wantOverall := domain.OverallTarget{Present: true, TargetSeconds: 3_600}
	wantExpected := domain.GoalConfiguration{
		IntervalGoal: domain.IntervalGoal{
			Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceWeekly,
			Alignment: domain.GoalAlignment{ISOWeekday: 1},
		},
		OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 7_200},
	}
	if expected != wantExpected || proposed != (domain.GoalConfiguration{IntervalGoal: wantInterval, OverallTarget: wantOverall}) {
		t.Fatalf("GoalConfigurations() = %+v, %+v; want %+v, proposed interval=%+v overall=%+v", expected, proposed, wantExpected, wantInterval, wantOverall)
	}
}

func TestGoalConfigurationsTreatsOmissionAsExplicitAbsence(t *testing.T) {
	expected, proposed, err := GoalConfigurations(dto.PathGoalsUpdate{Confirmed: true, ExpectedGoals: dto.PathGoalConfiguration{}})
	if err != nil || expected != (domain.GoalConfiguration{}) || proposed != (domain.GoalConfiguration{}) {
		t.Fatalf("GoalConfigurations() = %+v, %+v, %v; want absent goals", expected, proposed, err)
	}
}

func TestGoalConfigurationsRejectsNonCanonicalExpectedOrProposedAlignment(t *testing.T) {
	for _, goal := range []*dto.IntervalGoalItem{
		{TargetSeconds: 30, Recurrence: "daily"},
		{TargetSeconds: 30, Recurrence: "daily", Alignment: dto.IntervalAlignment{Minute: integer(0)}},
		{TargetSeconds: 30, Recurrence: "weekly", Alignment: dto.IntervalAlignment{ISOWeekday: integer(1), Day: integer(1)}},
	} {
		if _, _, err := GoalConfigurations(dto.PathGoalsUpdate{Confirmed: true, ExpectedGoals: dto.PathGoalConfiguration{}, IntervalGoal: goal}); err == nil {
			t.Fatalf("GoalConfigurations() accepted noncanonical proposed goal %+v", goal)
		}
		if _, _, err := GoalConfigurations(dto.PathGoalsUpdate{Confirmed: true, ExpectedGoals: dto.PathGoalConfiguration{IntervalGoal: goal}}); err == nil {
			t.Fatalf("GoalConfigurations() accepted noncanonical expected goal %+v", goal)
		}
	}
}
