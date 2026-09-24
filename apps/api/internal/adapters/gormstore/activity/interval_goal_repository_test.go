package activitystore

import (
	"context"
	"errors"
	"testing"
	"time"

	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type intervalGoalPathModelFixture struct{ ID string }

func (intervalGoalPathModelFixture) TableName() string { return "path_models" }

func TestPostgresIntervalGoalDecodesPresentAndAbsentGoalsExactly(t *testing.T) {
	db := postgresDB(t, true)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	participantID := "interval-goal-participant"
	seedParticipantAndPath(t, db, participantID, "interval-goal-path", now)
	seedPath(t, db, participantID, "interval-goal-absent-path", now)
	repository := New(db)

	want := pathdomain.IntervalGoal{
		Present:       true,
		TargetSeconds: 7200,
		Recurrence:    pathdomain.RecurrenceYearly,
		Alignment:     pathdomain.GoalAlignment{Month: 2, Day: 29},
	}
	if err := db.Table("path_models").Where("id = ?", "interval-goal-path").Updates(map[string]any{
		"interval_goal_target_seconds": want.TargetSeconds,
		"interval_goal_recurrence":     string(want.Recurrence),
		"interval_goal_start_minute":   nil,
		"interval_goal_start_hour":     nil,
		"interval_goal_start_weekday":  nil,
		"interval_goal_start_day":      want.Alignment.Day,
		"interval_goal_start_month":    want.Alignment.Month,
	}).Error; err != nil {
		t.Fatalf("persist interval goal: %v", err)
	}

	got, err := repository.IntervalGoal(context.Background(), participantID, "interval-goal-path")
	if err != nil || got != want {
		t.Fatalf("IntervalGoal() = %+v, %v; want %+v", got, err, want)
	}
	absent, err := repository.IntervalGoal(context.Background(), participantID, "interval-goal-absent-path")
	if err != nil || absent != (pathdomain.IntervalGoal{}) {
		t.Fatalf("IntervalGoal() for absent goal = %+v, %v; want canonical absence", absent, err)
	}
}

func TestPostgresIntervalGoalIsScopedToCurrentPathParticipants(t *testing.T) {
	db := postgresDB(t, true)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	ownerID, memberID, outsiderID, pathID := "interval-goal-owner", "interval-goal-member", "interval-goal-outsider", "interval-goal-shared-path"
	seedParticipantAndPath(t, db, ownerID, pathID, now)
	seedParticipantAndPath(t, db, memberID, "interval-goal-member-path", now)
	seedParticipantAndPath(t, db, outsiderID, "interval-goal-outsider-path", now)
	type membershipRow struct {
		PathID string `gorm:"primaryKey"`
		UserID string `gorm:"primaryKey"`
		Role   string
	}
	if err := db.Table("path_membership_models").Create(&membershipRow{PathID: pathID, UserID: memberID, Role: "participant"}).Error; err != nil {
		t.Fatalf("persist Path participant: %v", err)
	}
	want := pathdomain.IntervalGoal{
		Present:       true,
		TargetSeconds: 1800,
		Recurrence:    pathdomain.RecurrenceWeekly,
		Alignment:     pathdomain.GoalAlignment{ISOWeekday: 7},
	}
	if err := db.Table("path_models").Where("id = ?", pathID).Updates(map[string]any{
		"interval_goal_target_seconds": want.TargetSeconds,
		"interval_goal_recurrence":     string(want.Recurrence),
		"interval_goal_start_weekday":  want.Alignment.ISOWeekday,
	}).Error; err != nil {
		t.Fatalf("persist shared interval goal: %v", err)
	}

	repository := New(db)
	for _, participantID := range []string{ownerID, memberID} {
		got, err := repository.IntervalGoal(context.Background(), participantID, pathID)
		if err != nil || got != want {
			t.Fatalf("IntervalGoal(%q) = %+v, %v; want %+v", participantID, got, err, want)
		}
	}
	for name, input := range map[string]struct{ participantID, pathID string }{
		"nonparticipant": {participantID: outsiderID, pathID: pathID},
		"missing path":   {participantID: ownerID, pathID: "interval-goal-missing-path"},
	} {
		t.Run(name, func(t *testing.T) {
			goal, err := repository.IntervalGoal(context.Background(), input.participantID, input.pathID)
			if !errors.Is(err, ports.ErrNotFound) || goal != (pathdomain.IntervalGoal{}) {
				t.Fatalf("IntervalGoal() = %+v, %v; want concealed not found", goal, err)
			}
		})
	}
}

func TestPostgresIntervalGoalFailsClosedForMalformedPersistence(t *testing.T) {
	db := postgresDB(t, true)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	participantID, pathID := "interval-goal-malformed-participant", "interval-goal-malformed-path"
	seedParticipantAndPath(t, db, participantID, pathID, now)
	if err := db.Migrator().DropConstraint(&intervalGoalPathModelFixture{}, "path_models_interval_goal_check"); err != nil {
		t.Fatalf("drop interval-goal constraint inside rollback fixture: %v", err)
	}
	if err := db.Table("path_models").Where("id = ?", pathID).Updates(map[string]any{
		"interval_goal_target_seconds": int64(60),
		"interval_goal_recurrence":     nil,
	}).Error; err != nil {
		t.Fatalf("persist malformed interval goal: %v", err)
	}

	goal, err := New(db).IntervalGoal(context.Background(), participantID, pathID)
	if err == nil || goal != (pathdomain.IntervalGoal{}) {
		t.Fatalf("IntervalGoal() = %+v, %v; want fail-closed malformed persistence error", goal, err)
	}
}

func TestPostgresIntervalGoalRejectsInvalidArguments(t *testing.T) {
	db := postgresDB(t, true)
	tests := []struct {
		name          string
		repository    *Repository
		participantID string
		pathID        string
	}{
		{name: "nil repository", participantID: "participant", pathID: "path"},
		{name: "nil database", repository: &Repository{}, participantID: "participant", pathID: "path"},
		{name: "blank participant", repository: New(db), pathID: "path"},
		{name: "whitespace participant", repository: New(db), participantID: " participant", pathID: "path"},
		{name: "blank path", repository: New(db), participantID: "participant"},
		{name: "whitespace path", repository: New(db), participantID: "participant", pathID: "path "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			goal, err := test.repository.IntervalGoal(context.Background(), test.participantID, test.pathID)
			if !errors.Is(err, ports.ErrInvalidArgument) || goal != (pathdomain.IntervalGoal{}) {
				t.Fatalf("IntervalGoal() = %+v, %v; want invalid argument", goal, err)
			}
		})
	}
}
