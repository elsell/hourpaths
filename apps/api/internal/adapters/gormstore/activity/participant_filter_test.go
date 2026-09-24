package activitystore

import (
	"context"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

func TestParticipantFilteredActivityListRedactsAnotherMembersNote(t *testing.T) {
	db := postgresDB(t, false)
	now := time.Now().UTC().Truncate(time.Microsecond)
	viewerID, participantID, pathID := "filtered-viewer", "filtered-participant", "filtered-path"
	seedParticipantAndPath(t, db, viewerID, pathID, now)
	type participantRow struct {
		ID                   string `gorm:"primaryKey"`
		Status               identity.Status
		CreatedAt, UpdatedAt time.Time
	}
	type membershipRow struct {
		PathID, UserID, Role string
		JoinedAt             time.Time
	}
	if err := db.Table("user_models").Create(&participantRow{ID: participantID, Status: identity.StatusActive, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: participantID, Role: "participant", JoinedAt: now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	activity, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "filtered-activity", PathID: pathID, ParticipantID: participantID,
		StartedAt: now.Add(-25 * time.Second), DurationSeconds: 5, OccurrenceTimeZone: "Etc/UTC", Note: "private note",
	}, now.Add(-10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(fromActivity(activity)).Error; err != nil {
		t.Fatal(err)
	}
	page, err := New(db).ListActivities(context.Background(), viewerID, pathID, application.ActivityPageRequest{ParticipantID: participantID, Limit: 25, Snapshot: now.Add(time.Minute)})
	if err != nil || len(page.Items) != 1 || page.Items[0].Activity.ID != activity.ID || page.Items[0].Activity.Note != "" {
		t.Fatalf("participant-filtered activity list = %+v, %v", page, err)
	}
}
