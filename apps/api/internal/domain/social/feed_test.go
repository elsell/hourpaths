package social

import (
	"testing"
	"time"

	activitydomain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
)

func TestPracticeSessionEventUsesStableSourceIdentityAndServerPublicationTime(t *testing.T) {
	acceptedAt := time.Date(2026, 7, 27, 15, 4, 5, 6000, time.UTC)
	entry, err := activitydomain.RecordManualActivity(activitydomain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-1",
		StartedAt: acceptedAt.Add(-24 * time.Hour), DurationSeconds: 90,
		OccurrenceTimeZone: "America/New_York", Note: "private note",
	}, acceptedAt)
	if err != nil {
		t.Fatal(err)
	}

	event, err := NewPracticeSessionEvent(entry)
	if err != nil {
		t.Fatal(err)
	}
	want := PracticeSessionEvent{
		ID: "practice:activity-1", SourceActivityID: "activity-1",
		ParticipantID: "user-1", PathID: "path-1", PublishedAt: acceptedAt,
	}
	if event != want {
		t.Fatalf("NewPracticeSessionEvent() = %+v, want %+v", event, want)
	}

	edited, _, err := entry.EditByOwner("user-1", activitydomain.ActivityEdit{
		StartedAt: entry.StartedAt.Add(-time.Hour), DurationSeconds: 120,
		OccurrenceTimeZone: entry.OccurrenceTimeZone, Note: "different private note",
	}, acceptedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	editedEvent, err := NewPracticeSessionEvent(edited)
	if err != nil || editedEvent != event {
		t.Fatalf("event after source edit = %+v, %v; want stable %+v", editedEvent, err, event)
	}
}

func TestPracticeSessionEventRejectsInvalidSourceActivity(t *testing.T) {
	if event, err := NewPracticeSessionEvent(activitydomain.RecordedActivity{}); err == nil || event != (PracticeSessionEvent{}) {
		t.Fatalf("NewPracticeSessionEvent(invalid) = %+v, %v", event, err)
	}
}
