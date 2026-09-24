package activitystore

import (
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresIntervalSecondsClipsCompletedActivityToHalfOpenWindow(t *testing.T) {
	db := postgresDB(t, true)
	window := application.IntervalWindow{
		StartedAt: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 7, 22, 12, 10, 0, 0, time.UTC),
	}
	recordedAt := window.EndedAt.Add(time.Hour)
	participantID, pathID := "interval-participant", "interval-path"
	seedParticipantAndPath(t, db, participantID, pathID, recordedAt)
	seedParticipantAndPath(t, db, "interval-other-participant", "interval-other-owned-path", recordedAt)
	seedPath(t, db, participantID, "interval-other-path", recordedAt)

	record := func(id, activityParticipantID, activityPathID string, startedAt time.Time, duration time.Duration) *activityModel {
		t.Helper()
		entry, err := domain.RecordManualActivity(domain.ManualActivity{
			ID: id, PathID: activityPathID, ParticipantID: activityParticipantID,
			StartedAt: startedAt, DurationSeconds: int64(duration / time.Second), OccurrenceTimeZone: "Etc/UTC",
		}, recordedAt)
		if err != nil {
			t.Fatalf("RecordManualActivity(%q): %v", id, err)
		}
		return fromActivity(entry)
	}
	activities := []*activityModel{
		record("interval-left-edge", participantID, pathID, window.StartedAt.Add(-10*time.Minute), 10*time.Minute),
		record("interval-right-edge", participantID, pathID, window.EndedAt, 10*time.Minute),
		record("interval-left-overlap", participantID, pathID, window.StartedAt.Add(-time.Minute), 3*time.Minute),
		record("interval-right-overlap", participantID, pathID, window.EndedAt.Add(-2*time.Minute), 3*time.Minute),
		record("interval-encompassing", participantID, pathID, window.StartedAt.Add(-5*time.Minute), 20*time.Minute),
		record("interval-multiple-boundaries", participantID, pathID, window.StartedAt.Add(-20*time.Minute), 45*time.Minute),
		record("interval-independent-one", participantID, pathID, window.StartedAt.Add(3*time.Minute), 4*time.Minute),
		record("interval-independent-two", participantID, pathID, window.StartedAt.Add(5*time.Minute), 4*time.Minute),
		record("interval-other-participant", "interval-other-participant", pathID, window.StartedAt, 10*time.Minute),
		record("interval-other-path", participantID, "interval-other-path", window.StartedAt, 10*time.Minute),
	}
	if err := db.Create(&activities).Error; err != nil {
		t.Fatalf("persist interval activities: %v", err)
	}
	running, err := domain.StartTimer("interval-running", pathID, participantID, window.StartedAt.Add(2*time.Minute), "Etc/UTC", window.StartedAt.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(fromTimer(running)).Error; err != nil {
		t.Fatalf("persist overlapping running timer: %v", err)
	}

	repository := New(db)
	const wantAccumulated int64 = 600 + 600 + 180 + 180 + 1200 + 2700 + 240 + 240
	before, err := repository.AccumulatedSeconds(context.Background(), participantID, pathID)
	if err != nil || before != wantAccumulated {
		t.Fatalf("AccumulatedSeconds() before interval projection = %d, %v; want %d", before, err, wantAccumulated)
	}
	const wantInterval int64 = 120 + 120 + 600 + 600 + 240 + 240
	if got, err := repository.IntervalSeconds(context.Background(), participantID, pathID, window); err != nil || got != wantInterval {
		t.Fatalf("IntervalSeconds() = %d, %v; want %d", got, err, wantInterval)
	}
	if got, err := repository.AccumulatedSeconds(context.Background(), participantID, pathID); err != nil || got != wantAccumulated {
		t.Fatalf("AccumulatedSeconds() after interval projection = %d, %v; want unchanged full duration %d", got, err, wantAccumulated)
	}
}

func TestPostgresIntervalSecondsPartitionsCanonicalWholeSecondsWithoutLoss(t *testing.T) {
	db := postgresDB(t, true)
	boundary := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	participantID, pathID := "interval-subsecond-participant", "interval-subsecond-path"
	seedParticipantAndPath(t, db, participantID, pathID, boundary.Add(time.Hour))
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "interval-subsecond-activity", PathID: pathID, ParticipantID: participantID,
		StartedAt: boundary.Add(-500 * time.Millisecond), DurationSeconds: 1, OccurrenceTimeZone: "Etc/UTC",
	}, boundary.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(fromActivity(entry)).Error; err != nil {
		t.Fatalf("persist subsecond boundary activity: %v", err)
	}

	repository := New(db)
	windows := []struct {
		name   string
		window application.IntervalWindow
		want   int64
	}{
		{name: "ending interval", window: application.IntervalWindow{StartedAt: boundary.Add(-time.Hour), EndedAt: boundary}, want: 0},
		{name: "beginning interval", window: application.IntervalWindow{StartedAt: boundary, EndedAt: boundary.Add(time.Hour)}, want: 1},
	}
	var partitioned int64
	for _, test := range windows {
		t.Run(test.name, func(t *testing.T) {
			got, err := repository.IntervalSeconds(context.Background(), participantID, pathID, test.window)
			if err != nil || got != test.want {
				t.Fatalf("IntervalSeconds() = %d, %v; want %d", got, err, test.want)
			}
			partitioned += got
		})
	}
	if partitioned != entry.DurationSeconds() {
		t.Fatalf("adjacent interval total = %d; want canonical duration %d", partitioned, entry.DurationSeconds())
	}
}

func TestPostgresIntervalSecondsRejectsInvalidScopeAndWindow(t *testing.T) {
	db := postgresDB(t, true)
	start := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	valid := application.IntervalWindow{StartedAt: start, EndedAt: start.Add(time.Hour)}
	tests := []struct {
		name          string
		repository    *Repository
		participantID string
		pathID        string
		window        application.IntervalWindow
	}{
		{name: "nil repository", participantID: "participant", pathID: "path", window: valid},
		{name: "nil database", repository: &Repository{}, participantID: "participant", pathID: "path", window: valid},
		{name: "blank participant", repository: New(db), pathID: "path", window: valid},
		{name: "whitespace participant", repository: New(db), participantID: " participant", pathID: "path", window: valid},
		{name: "blank path", repository: New(db), participantID: "participant", window: valid},
		{name: "whitespace path", repository: New(db), participantID: "participant", pathID: "path ", window: valid},
		{name: "zero start", repository: New(db), participantID: "participant", pathID: "path", window: application.IntervalWindow{EndedAt: start.Add(time.Hour)}},
		{name: "zero end", repository: New(db), participantID: "participant", pathID: "path", window: application.IntervalWindow{StartedAt: start}},
		{name: "empty window", repository: New(db), participantID: "participant", pathID: "path", window: application.IntervalWindow{StartedAt: start, EndedAt: start}},
		{name: "reversed window", repository: New(db), participantID: "participant", pathID: "path", window: application.IntervalWindow{StartedAt: start, EndedAt: start.Add(-time.Second)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.repository.IntervalSeconds(context.Background(), test.participantID, test.pathID, test.window); !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("IntervalSeconds() error = %v, want invalid argument", err)
			}
		})
	}
}
