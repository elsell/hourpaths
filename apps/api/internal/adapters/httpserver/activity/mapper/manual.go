package mapper

import (
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/dto"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
)

func ManualInput(input dto.ManualActivityInputBody) application.ManualActivityInput {
	return application.ManualActivityInput{LocalDate: input.LocalDate, LocalStartTime: input.LocalStartTime, DurationSeconds: input.DurationSeconds, Note: input.Note}
}

func Created(value application.CreateManualActivityResult) dto.ActivityMutationResult {
	return dto.ActivityMutationResult{Activity: Activity(value.Activity), Version: value.Version, AccumulatedSeconds: value.AccumulatedSeconds, IntervalProgress: IntervalProgress(value.IntervalProgress)}
}

func Updated(value application.UpdateActivityResult) dto.ActivityMutationResult {
	return dto.ActivityMutationResult{Activity: Activity(value.Activity), Version: value.Version, AccumulatedSeconds: value.AccumulatedSeconds, IntervalProgress: IntervalProgress(value.IntervalProgress)}
}

func Deleted(value application.DeleteActivityResult) dto.ActivityDeletionResult {
	return dto.ActivityDeletionResult{AccumulatedSeconds: value.AccumulatedSeconds, SessionCount: value.SessionCount, UnreadNotificationCount: value.UnreadNotificationCount, RemovedFeedEventIDs: append([]string(nil), value.RemovedFeedEventIDs...), IntervalProgress: IntervalProgress(value.IntervalProgress)}
}

func Detail(entry domain.RecordedActivity, version int64) dto.ActivityDetail {
	return dto.ActivityDetail{Activity: Activity(entry), Version: version}
}

func Revisions(values []application.ActivityRevisionRecord) []dto.ActivityRevision {
	items := make([]dto.ActivityRevision, 0, len(values))
	for _, value := range values {
		items = append(items, dto.ActivityRevision{Activity: Activity(value.Revision.Activity), Version: value.Version, ReplacedAt: value.Revision.ReplacedAt})
	}
	return items
}

func List(values []application.ActivityListRecord) []dto.ActivityDetail {
	items := make([]dto.ActivityDetail, 0, len(values))
	for _, value := range values {
		items = append(items, Detail(value.Activity, value.Version))
	}
	return items
}
