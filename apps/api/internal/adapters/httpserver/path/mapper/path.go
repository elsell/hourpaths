package mapper

import (
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/dto"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func Item(entity domain.Entity) dto.PathItem {
	item := dto.PathItem{ID: string(entity.ID), Name: entity.Name, Visibility: entity.Visibility}
	if entity.Archived() {
		archivedAt := entity.ArchivedAt
		item.ArchivedAt = &archivedAt
	}
	if entity.IntervalGoal.Present {
		item.IntervalGoal = intervalGoalItem(entity.IntervalGoal)
	}
	if entity.OverallTarget.Present {
		item.OverallTarget = &dto.OverallTarget{TargetSeconds: entity.OverallTarget.TargetSeconds}
	}
	return item
}

func ProjectedItem(projection pathapp.Projection) dto.PathProjection {
	item := Item(projection.Path)
	result := dto.PathProjection{
		ID:            item.ID,
		Name:          item.Name,
		Visibility:    item.Visibility,
		IntervalGoal:  item.IntervalGoal,
		OverallTarget: item.OverallTarget,
		ArchivedAt:    item.ArchivedAt,
		Capabilities: dto.PathCapabilities{
			TrackTime:         projection.Capabilities.TrackTime,
			RenamePath:        projection.Capabilities.RenamePath,
			InviteMembers:     projection.Capabilities.InviteMembers,
			ManageMembers:     projection.Capabilities.ManageMembers,
			ManageGoals:       projection.Capabilities.ManageGoals,
			ManageLifecycle:   projection.Capabilities.ManageLifecycle,
			ManageVisibility:  projection.Capabilities.ManageVisibility,
			TransferOwnership: projection.Capabilities.TransferOwnership,
			LeavePath:         projection.Capabilities.LeavePath,
		},
	}
	if projection.Home.Classification != "" {
		result.Home = &dto.HomeOrganization{Classification: string(projection.Home.Classification), Pinned: projection.Home.PinnedPosition != nil, PinnedPosition: projection.Home.PinnedPosition, ManualPosition: projection.Home.ManualPosition, RecentActivityAt: projection.Home.RecentActivityAt}
	}
	return result
}

func HomePreferences(preferences domain.HomePreferences) dto.HomePreferences {
	result := dto.HomePreferences{OrderMethod: string(preferences.OrderMethod), Revision: preferences.Revision, PinnedPathIDs: make([]string, len(preferences.PinnedPathIDs)), ManualPathIDs: make([]string, len(preferences.ManualPathIDs))}
	for index, id := range preferences.PinnedPathIDs {
		result.PinnedPathIDs[index] = string(id)
	}
	for index, id := range preferences.ManualPathIDs {
		result.ManualPathIDs[index] = string(id)
	}
	if !preferences.UpdatedAt.IsZero() {
		updatedAt := preferences.UpdatedAt
		result.UpdatedAt = &updatedAt
	}
	return result
}

func Attributes(input dto.PathCreate) (domain.Attributes, error) {
	attributes := domain.Attributes{Name: input.Name, Visibility: input.Visibility}
	if input.OverallTarget != nil {
		if input.OverallTarget.TargetSeconds <= 0 {
			return domain.Attributes{}, ports.ErrInvalidArgument
		}
		attributes.OverallTarget = domain.OverallTarget{Present: true, TargetSeconds: input.OverallTarget.TargetSeconds}
	}
	if input.IntervalGoal == nil {
		return attributes, nil
	}
	recurrence := domain.Recurrence(input.IntervalGoal.Recurrence)
	if input.IntervalGoal.TargetSeconds <= 0 || !supportedRecurrence(recurrence) {
		return domain.Attributes{}, ports.ErrInvalidArgument
	}
	alignment, err := intervalAlignment(recurrence, input.IntervalGoal.Alignment)
	if err != nil {
		return domain.Attributes{}, err
	}
	attributes.IntervalGoal = domain.IntervalGoal{Present: true, TargetSeconds: input.IntervalGoal.TargetSeconds, Recurrence: recurrence, Alignment: alignment}
	return attributes, nil
}

func UpdateAttributes(input dto.PathUpdate) domain.Attributes {
	return domain.Attributes{Name: input.Name, Visibility: input.Visibility}
}

func GoalConfigurations(input dto.PathGoalsUpdate) (domain.GoalConfiguration, domain.GoalConfiguration, error) {
	expected, err := goalConfiguration(input.ExpectedGoals.IntervalGoal, input.ExpectedGoals.OverallTarget)
	if err != nil {
		return domain.GoalConfiguration{}, domain.GoalConfiguration{}, err
	}
	proposed, err := goalConfiguration(input.IntervalGoal, input.OverallTarget)
	if err != nil {
		return domain.GoalConfiguration{}, domain.GoalConfiguration{}, err
	}
	return expected, proposed, nil
}

func goalConfiguration(interval *dto.IntervalGoalItem, overallInput *dto.OverallTarget) (domain.GoalConfiguration, error) {
	var overall domain.OverallTarget
	if overallInput != nil {
		if overallInput.TargetSeconds <= 0 {
			return domain.GoalConfiguration{}, ports.ErrInvalidArgument
		}
		overall = domain.OverallTarget{Present: true, TargetSeconds: overallInput.TargetSeconds}
	}
	if interval == nil {
		return domain.GoalConfiguration{OverallTarget: overall}, nil
	}
	recurrence := domain.Recurrence(interval.Recurrence)
	if interval.TargetSeconds <= 0 || !supportedRecurrence(recurrence) {
		return domain.GoalConfiguration{}, ports.ErrInvalidArgument
	}
	alignment, err := intervalAlignment(recurrence, &interval.Alignment)
	if err != nil {
		return domain.GoalConfiguration{}, err
	}
	return domain.GoalConfiguration{
		IntervalGoal: domain.IntervalGoal{
			Present:       true,
			TargetSeconds: interval.TargetSeconds,
			Recurrence:    recurrence,
			Alignment:     alignment,
		},
		OverallTarget: overall,
	}, nil
}

func supportedRecurrence(recurrence domain.Recurrence) bool {
	switch recurrence {
	case domain.RecurrenceHourly, domain.RecurrenceDaily, domain.RecurrenceWeekly, domain.RecurrenceMonthly, domain.RecurrenceYearly:
		return true
	default:
		return false
	}
}

func intervalAlignment(recurrence domain.Recurrence, input *dto.IntervalAlignment) (domain.GoalAlignment, error) {
	if input == nil {
		return domain.GoalAlignment{}, nil
	}
	invalid := func() (domain.GoalAlignment, error) { return domain.GoalAlignment{}, ports.ErrInvalidArgument }
	switch recurrence {
	case domain.RecurrenceHourly:
		if input.Minute == nil || *input.Minute < 0 || *input.Minute > 59 || input.Hour != nil || input.ISOWeekday != nil || input.Month != nil || input.Day != nil {
			return invalid()
		}
		return domain.GoalAlignment{Minute: *input.Minute}, nil
	case domain.RecurrenceDaily:
		if input.Hour == nil || *input.Hour < 0 || *input.Hour > 23 || input.Minute != nil || input.ISOWeekday != nil || input.Month != nil || input.Day != nil {
			return invalid()
		}
		return domain.GoalAlignment{Hour: *input.Hour}, nil
	case domain.RecurrenceWeekly:
		if input.ISOWeekday == nil || *input.ISOWeekday < 1 || *input.ISOWeekday > 7 || input.Minute != nil || input.Hour != nil || input.Month != nil || input.Day != nil {
			return invalid()
		}
		return domain.GoalAlignment{ISOWeekday: *input.ISOWeekday}, nil
	case domain.RecurrenceMonthly:
		if input.Day == nil || *input.Day < 1 || *input.Day > 31 || input.Minute != nil || input.Hour != nil || input.ISOWeekday != nil || input.Month != nil {
			return invalid()
		}
		return domain.GoalAlignment{Day: *input.Day}, nil
	case domain.RecurrenceYearly:
		if input.Month == nil || input.Day == nil || input.Minute != nil || input.Hour != nil || input.ISOWeekday != nil || !validYearlyDate(*input.Month, *input.Day) {
			return invalid()
		}
		return domain.GoalAlignment{Month: *input.Month, Day: *input.Day}, nil
	default:
		return invalid()
	}
}

func validYearlyDate(month, day int) bool {
	if month < 1 || month > 12 || day < 1 {
		return false
	}
	date := time.Date(2000, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return int(date.Month()) == month && date.Day() == day
}

func intervalGoalItem(goal domain.IntervalGoal) *dto.IntervalGoalItem {
	item := &dto.IntervalGoalItem{TargetSeconds: goal.TargetSeconds, Recurrence: string(goal.Recurrence)}
	switch goal.Recurrence {
	case domain.RecurrenceHourly:
		item.Alignment.Minute = intPointer(goal.Alignment.Minute)
	case domain.RecurrenceDaily:
		item.Alignment.Hour = intPointer(goal.Alignment.Hour)
	case domain.RecurrenceWeekly:
		item.Alignment.ISOWeekday = intPointer(goal.Alignment.ISOWeekday)
	case domain.RecurrenceMonthly:
		item.Alignment.Day = intPointer(goal.Alignment.Day)
	case domain.RecurrenceYearly:
		item.Alignment.Month = intPointer(goal.Alignment.Month)
		item.Alignment.Day = intPointer(goal.Alignment.Day)
	}
	return item
}

func intPointer(value int) *int { return &value }
