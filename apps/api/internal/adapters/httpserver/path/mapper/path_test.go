package mapper

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/dto"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func integer(value int) *int { return &value }

func TestAttributesMapsOptionalGoalsAndAlignmentPresence(t *testing.T) {
	tests := []struct {
		name  string
		input dto.PathCreate
		want  domain.Attributes
	}{
		{name: "neither", input: dto.PathCreate{Name: "Read", Visibility: "private"}, want: domain.Attributes{Name: "Read", Visibility: "private"}},
		{name: "overall only", input: dto.PathCreate{Name: "Read", OverallTarget: &dto.OverallTarget{TargetSeconds: 3_600}}, want: domain.Attributes{Name: "Read", OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 3_600}}},
		{name: "hourly default alignment omitted", input: dto.PathCreate{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 60, Recurrence: "hourly"}}, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceHourly}}},
		{name: "hourly", input: dto.PathCreate{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 60, Recurrence: "hourly", Alignment: &dto.IntervalAlignment{Minute: integer(0)}}}, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceHourly, Alignment: domain.GoalAlignment{Minute: 0}}}},
		{name: "daily", input: dto.PathCreate{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 600, Recurrence: "daily", Alignment: &dto.IntervalAlignment{Hour: integer(5)}}}, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 600, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 5}}}},
		{name: "weekly", input: dto.PathCreate{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 3_600, Recurrence: "weekly", Alignment: &dto.IntervalAlignment{ISOWeekday: integer(1)}}}, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 3_600, Recurrence: domain.RecurrenceWeekly, Alignment: domain.GoalAlignment{ISOWeekday: 1}}}},
		{name: "monthly", input: dto.PathCreate{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 7_200, Recurrence: "monthly", Alignment: &dto.IntervalAlignment{Day: integer(31)}}}, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 7_200, Recurrence: domain.RecurrenceMonthly, Alignment: domain.GoalAlignment{Day: 31}}}},
		{name: "yearly and overall", input: dto.PathCreate{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 36_000, Recurrence: "yearly", Alignment: &dto.IntervalAlignment{Month: integer(2), Day: integer(29)}}, OverallTarget: &dto.OverallTarget{TargetSeconds: 360_000}}, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 36_000, Recurrence: domain.RecurrenceYearly, Alignment: domain.GoalAlignment{Month: 2, Day: 29}}, OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 360_000}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Attributes(test.input)
			if err != nil || got != test.want {
				t.Fatalf("Attributes() = %+v, %v; want %+v", got, err, test.want)
			}
		})
	}
}

func TestAttributesRejectsMalformedGoalPresence(t *testing.T) {
	tests := []dto.PathCreate{
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 0, Recurrence: "daily"}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "fortnightly"}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "hourly", Alignment: &dto.IntervalAlignment{}}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "hourly", Alignment: &dto.IntervalAlignment{Hour: integer(1)}}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "daily", Alignment: &dto.IntervalAlignment{Hour: integer(24)}}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "weekly", Alignment: &dto.IntervalAlignment{ISOWeekday: integer(1), Day: integer(1)}}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "monthly", Alignment: &dto.IntervalAlignment{Day: integer(0)}}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "yearly", Alignment: &dto.IntervalAlignment{}}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "yearly", Alignment: &dto.IntervalAlignment{Month: integer(2)}}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "yearly", Alignment: &dto.IntervalAlignment{Day: integer(29)}}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "yearly", Alignment: &dto.IntervalAlignment{Month: integer(4), Day: integer(31)}}},
		{Name: "Read", IntervalGoal: &dto.IntervalGoalCreate{TargetSeconds: 1, Recurrence: "yearly", Alignment: &dto.IntervalAlignment{Month: integer(2), Day: integer(29), Hour: integer(1)}}},
		{Name: "Read", OverallTarget: &dto.OverallTarget{TargetSeconds: 0}},
	}
	for index, input := range tests {
		if _, err := Attributes(input); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("case %d error = %v, want invalid argument", index, err)
		}
	}
}

func TestItemMapsCanonicalGoalsAndOmitsAbsentGoals(t *testing.T) {
	withoutGoals := Item(domain.Entity{ID: "none", Attributes: domain.Attributes{Name: "Read", Visibility: "private"}})
	encoded, err := json.Marshal(withoutGoals)
	if err != nil || string(encoded) != `{"id":"none","name":"Read","visibility":"private"}` {
		t.Fatalf("absent goals JSON = %s, %v", encoded, err)
	}

	entity := domain.Entity{ID: "both", Attributes: domain.Attributes{
		Name: "Read", Visibility: "followers",
		IntervalGoal:  domain.IntervalGoal{Present: true, TargetSeconds: 36_000, Recurrence: domain.RecurrenceYearly, Alignment: domain.GoalAlignment{Month: 2, Day: 29}},
		OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 360_000},
	}}
	got := Item(entity)
	want := dto.PathItem{ID: "both", Name: "Read", Visibility: "followers", IntervalGoal: &dto.IntervalGoalItem{TargetSeconds: 36_000, Recurrence: "yearly", Alignment: dto.IntervalAlignment{Month: integer(2), Day: integer(29)}}, OverallTarget: &dto.OverallTarget{TargetSeconds: 360_000}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Item() = %+v, want %+v", got, want)
	}

	if UpdateAttributes(dto.PathUpdate{Name: "value", Visibility: "private"}) != (domain.Attributes{Name: "value", Visibility: "private"}) {
		t.Fatal("PathUpdate goal behavior changed")
	}
}
