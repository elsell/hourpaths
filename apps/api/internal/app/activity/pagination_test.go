package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPathActivityHistoryReturnsStableOwnerBoundCursor(t *testing.T) {
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-2", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC",
	}, testNow.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	var pages []ActivityPageRequest
	service := testService(testRepository{activities: []ActivityListRecord{{Activity: entry, Version: 1}}, activityHasMore: true, activityPages: &pages})
	items, cursor, err := service.ListActivities(context.Background(), "Bearer valid", "path-1", "user-2", "", 0)
	if err != nil || len(items) != 1 || cursor == "" || len(pages) != 1 || pages[0].ParticipantID != "user-2" || pages[0].Limit != 25 || !pages[0].Snapshot.Equal(testNow) {
		t.Fatalf("first page = %+v, %q, %v; requests=%+v", items, cursor, err, pages)
	}
	payload, err := shared.DecodeCursor(service.CursorSigningKey, cursor)
	if err != nil || payload.Owner != "user-1" || payload.Domain != "path-activities:path-1:participant:user-2" || payload.AfterID != entry.ID || !payload.AfterCreated.Equal(entry.StartedAt) || !payload.Snapshot.Equal(testNow) {
		t.Fatalf("cursor payload = %+v, %v", payload, err)
	}
	service.Repository = testRepository{activityPages: &pages}
	if _, next, err := service.ListActivities(context.Background(), "Bearer valid", "path-1", "user-2", cursor, 100); err != nil || next != "" {
		t.Fatalf("next page cursor=%q error=%v", next, err)
	}
	if len(pages) != 2 || pages[1].ParticipantID != "user-2" || pages[1].AfterID != entry.ID || !pages[1].AfterStartedAt.Equal(entry.StartedAt) || !pages[1].Snapshot.Equal(testNow) || pages[1].Limit != 100 {
		t.Fatalf("decoded page request = %+v", pages)
	}
	if _, _, err := service.ListActivities(context.Background(), "Bearer valid", "path-1", "user-3", cursor, 100); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("cursor reused with a different participant returned %v", err)
	}
}

func TestActivityRevisionHistoryReturnsStableResourceBoundCursor(t *testing.T) {
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-1", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC",
	}, testNow.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	replacedAt := testNow.Add(-time.Second)
	revision := ActivityRevisionRecord{Version: 3, Revision: domain.ActivityRevision{Activity: entry, ReplacedAt: replacedAt}}
	var pages []ActivityRevisionPageRequest
	service := testService(testRepository{revisions: []ActivityRevisionRecord{revision}, revisionHasMore: true, revisionPages: &pages})
	items, cursor, err := service.ListActivityRevisions(context.Background(), "Bearer valid", "path-1", "activity-1", "", 1)
	if err != nil || len(items) != 1 || cursor == "" || len(pages) != 1 || pages[0].Limit != 1 || !pages[0].Snapshot.Equal(testNow) {
		t.Fatalf("first page = %+v, %q, %v; requests=%+v", items, cursor, err, pages)
	}
	payload, err := shared.DecodeCursor(service.CursorSigningKey, cursor)
	if err != nil || payload.Owner != "user-1" || payload.Domain != "activity-revisions:path-1:activity-1" || payload.AfterID != "3" || !payload.AfterCreated.Equal(replacedAt) || !payload.Snapshot.Equal(testNow) {
		t.Fatalf("cursor payload = %+v, %v", payload, err)
	}
	service.Repository = testRepository{revisionPages: &pages}
	if _, next, err := service.ListActivityRevisions(context.Background(), "Bearer valid", "path-1", "activity-1", cursor, 25); err != nil || next != "" {
		t.Fatalf("next page cursor=%q error=%v", next, err)
	}
	if len(pages) != 2 || pages[1].BeforeVersion != 3 || !pages[1].Snapshot.Equal(testNow) || pages[1].Limit != 25 {
		t.Fatalf("decoded page request = %+v", pages)
	}
}

func TestActivityHistoriesRejectInvalidPagination(t *testing.T) {
	service := testService(testRepository{})
	if _, _, err := service.ListActivities(context.Background(), "Bearer valid", "path-1", " user-2", "", 25); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("invalid participant filter returned %v", err)
	}
	wrongActivityOwner := activityCursor(t, service.CursorSigningKey, "other", "path-activities:path-1", "activity-1")
	wrongActivityDomain := activityCursor(t, service.CursorSigningKey, "user-1", "path-activities:path-2", "activity-1")
	wrongRevisionOwner := activityCursor(t, service.CursorSigningKey, "other", "activity-revisions:path-1:activity-1", "2")
	wrongRevisionDomain := activityCursor(t, service.CursorSigningKey, "user-1", "activity-revisions:path-1:activity-2", "2")
	for _, cursor := range []string{"malformed", wrongActivityOwner, wrongActivityDomain} {
		if _, _, err := service.ListActivities(context.Background(), "Bearer valid", "path-1", "", cursor, 25); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("activity cursor %q returned %v", cursor, err)
		}
	}
	for _, cursor := range []string{"malformed", wrongRevisionOwner, wrongRevisionDomain} {
		if _, _, err := service.ListActivityRevisions(context.Background(), "Bearer valid", "path-1", "activity-1", cursor, 25); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("revision cursor %q returned %v", cursor, err)
		}
	}
	for _, limit := range []int{-1, 101} {
		if _, _, err := service.ListActivities(context.Background(), "Bearer valid", "path-1", "", "", limit); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("activity limit %d returned %v", limit, err)
		}
		if _, _, err := service.ListActivityRevisions(context.Background(), "Bearer valid", "path-1", "activity-1", "", limit); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("revision limit %d returned %v", limit, err)
		}
	}
	service.CursorSigningKey = nil
	if _, _, err := service.ListActivities(context.Background(), "Bearer valid", "path-1", "", "", 25); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("missing activity signing key returned %v", err)
	}
	if _, _, err := service.ListActivityRevisions(context.Background(), "Bearer valid", "path-1", "activity-1", "", 25); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("missing revision signing key returned %v", err)
	}
}

func activityCursor(t *testing.T, key []byte, owner, domainName, afterID string) string {
	t.Helper()
	value, err := shared.EncodeCursor(key, shared.CursorPayload{Version: 1, Owner: owner, Domain: domainName, AfterID: afterID, AfterCreated: testNow.Add(-time.Minute), Snapshot: testNow})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
