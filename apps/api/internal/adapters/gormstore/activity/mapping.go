package activitystore

import domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"

func fromRevision(revision domain.ActivityRevision, version int64, publicChanged bool) activityRevisionModel {
	return activityRevisionModel{ActivityID: revision.Activity.ID, Version: version, StartedAt: revision.Activity.StartedAt, EndedAt: revision.Activity.EndedAt, OccurrenceTimeZone: revision.Activity.OccurrenceTimeZone, Note: optionalNote(revision.Activity.Note), PublicChanged: publicChanged, UpdatedAt: revision.Activity.UpdatedAt, ReplacedAt: revision.ReplacedAt}
}

func fromTimer(timer domain.RunningTimer) *timerModel {
	return &timerModel{ID: timer.ID, PathID: timer.PathID, ParticipantID: timer.ParticipantID, StartedAt: timer.StartedAt, OccurrenceTimeZone: timer.OccurrenceTimeZone}
}

func toTimer(row timerModel) domain.RunningTimer {
	return domain.RunningTimer{ID: row.ID, PathID: row.PathID, ParticipantID: row.ParticipantID, StartedAt: row.StartedAt.UTC(), OccurrenceTimeZone: row.OccurrenceTimeZone}
}

func fromActivity(activity domain.RecordedActivity) *activityModel {
	return &activityModel{ID: activity.ID, PathID: activity.PathID, ParticipantID: activity.ParticipantID, StartedAt: activity.StartedAt, EndedAt: activity.EndedAt, OccurrenceTimeZone: activity.OccurrenceTimeZone, Note: optionalNote(activity.Note), CreatedAt: activity.CreatedAt, UpdatedAt: activity.UpdatedAt}
}

func toActivity(row activityModel) domain.RecordedActivity {
	return domain.RecordedActivity{ID: row.ID, PathID: row.PathID, ParticipantID: row.ParticipantID, StartedAt: row.StartedAt.UTC(), EndedAt: row.EndedAt.UTC(), OccurrenceTimeZone: row.OccurrenceTimeZone, Note: noteValue(row.Note), CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
}

func optionalNote(note string) *string {
	if note == "" {
		return nil
	}
	return &note
}

func noteValue(note *string) string {
	if note == nil {
		return ""
	}
	return *note
}
