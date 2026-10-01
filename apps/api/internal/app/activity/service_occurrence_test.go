package activity

import (
	"context"
	"testing"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
)

func TestUpdateActivityRetainsUnchangedRepeatedHourAndFractionalStart(t *testing.T) {
	start := time.Date(2025, time.November, 2, 6, 30, 0, 250_000_000, time.UTC)
	original, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-1",
		StartedAt: start, DurationSeconds: 60, OccurrenceTimeZone: "America/New_York", Note: "before",
	}, testNow.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	edited, revision, err := original.EditByOwner("user-1", domain.ActivityEdit{
		StartedAt: start, DurationSeconds: 120, OccurrenceTimeZone: original.OccurrenceTimeZone, Note: "after",
	}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	var updates []UpdateActivityCommand
	service := testService(testRepository{updates: &updates, updateResult: UpdateActivityResult{Activity: edited, Revision: revision, Version: 2, AccumulatedSeconds: 120}})
	_, err = service.UpdateActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "update-request-001", ManualActivityInput{
		LocalDate: "2025-11-02", LocalStartTime: "01:30:00", DurationSeconds: 120, Note: "after",
	})
	if err != nil || len(updates) != 1 || !updates[0].Edit.StartedAt.Equal(start) {
		t.Fatalf("unchanged occurrence moved: updates=%+v error=%v", updates, err)
	}
}
