package activitystore

import domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"

func mutationResultUpdates(timer timerModel, entry domain.RecordedActivity, saved bool) map[string]any {
	updates := map[string]any{
		"result_started_at":     timer.StartedAt,
		"result_time_zone":      timer.OccurrenceTimeZone,
		"result_activity_saved": saved,
	}
	if saved {
		updates["result_activity_id"] = entry.ID
		updates["result_ended_at"] = entry.EndedAt
		updates["result_created_at"] = entry.CreatedAt
		updates["result_updated_at"] = entry.UpdatedAt
	}
	return updates
}
