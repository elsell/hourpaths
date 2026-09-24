package mapper

import (
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/dto"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
)

func Timer(value domain.RunningTimer) dto.Timer {
	return dto.Timer{ID: value.ID, PathID: value.PathID, StartedAt: value.StartedAt, OccurrenceTimeZone: value.OccurrenceTimeZone}
}

func Current(value application.CurrentTimerResult) dto.TimerState {
	result := dto.TimerState{AccumulatedSeconds: value.AccumulatedSeconds, IntervalProgress: IntervalProgress(value.IntervalProgress)}
	if value.Timer != nil {
		mapped := Timer(*value.Timer)
		result.Running, result.Timer = true, &mapped
	}
	return result
}

func Started(value application.StartTimerResult) dto.TimerState {
	if value.Timer == (domain.RunningTimer{}) {
		return dto.TimerState{AccumulatedSeconds: value.AccumulatedSeconds, IntervalProgress: IntervalProgress(value.IntervalProgress)}
	}
	mapped := Timer(value.Timer)
	return dto.TimerState{Running: true, Timer: &mapped, AccumulatedSeconds: value.AccumulatedSeconds, IntervalProgress: IntervalProgress(value.IntervalProgress)}
}

func Stopped(value application.StopTimerResult) dto.StopResult {
	result := dto.StopResult{TimerState: dto.TimerState{Running: false, AccumulatedSeconds: value.AccumulatedSeconds, IntervalProgress: IntervalProgress(value.IntervalProgress)}, Saved: value.Saved, Subsecond: !value.Saved}
	if value.CurrentTimer != nil {
		mapped := Timer(*value.CurrentTimer)
		result.Running, result.Timer = true, &mapped
	}
	if value.Saved {
		entry := value.Activity
		mapped := Activity(entry)
		result.Activity = &mapped
	}
	return result
}

func IntervalProgress(value *application.IntervalProgress) *dto.IntervalProgress {
	if value == nil {
		return nil
	}
	return &dto.IntervalProgress{AccumulatedSeconds: value.AccumulatedSeconds, TargetSeconds: value.TargetSeconds}
}

func Activity(entry domain.RecordedActivity) dto.Activity {
	result := dto.Activity{ID: entry.ID, PathID: entry.PathID, ParticipantID: entry.ParticipantID, StartedAt: entry.StartedAt, EndedAt: entry.EndedAt, OccurrenceTimeZone: entry.OccurrenceTimeZone, DurationSeconds: entry.DurationSeconds(), CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt}
	if entry.Note != "" {
		note := entry.Note
		result.Note = &note
	}
	return result
}
