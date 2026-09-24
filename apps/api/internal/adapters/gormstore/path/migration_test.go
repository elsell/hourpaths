package pathstore

import (
	"io/fs"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestOptionalGoalMigrationEncodesIndependentTypedGoalState(t *testing.T) {
	if dbmigrations.LatestVersion < 28 {
		t.Fatalf("latest migration version = %d, want optional Path goals migration 28 or later", dbmigrations.LatestVersion)
	}
	contents, err := fs.ReadFile(dbmigrations.Files, "000028_path_creation_goals.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(contents)
	for _, required := range []string{
		"interval_goal_target_seconds bigint",
		"interval_goal_recurrence text",
		"interval_goal_start_minute smallint",
		"interval_goal_start_hour smallint",
		"interval_goal_start_weekday smallint",
		"interval_goal_start_day smallint",
		"interval_goal_start_month smallint",
		"overall_target_seconds bigint",
		"CONSTRAINT path_models_interval_goal_check CHECK",
		"interval_goal_target_seconds IS NULL",
		"interval_goal_recurrence IS NULL",
		"interval_goal_target_seconds > 0",
		"interval_goal_recurrence = 'hourly'",
		"interval_goal_start_minute BETWEEN 0 AND 59",
		"interval_goal_recurrence = 'daily'",
		"interval_goal_start_hour BETWEEN 0 AND 23",
		"interval_goal_recurrence = 'weekly'",
		"interval_goal_start_weekday BETWEEN 1 AND 7",
		"interval_goal_recurrence = 'monthly'",
		"interval_goal_start_day BETWEEN 1 AND 31",
		"interval_goal_recurrence = 'yearly'",
		"interval_goal_start_month BETWEEN 1 AND 12",
		"WHEN 2 THEN 29",
		"WHEN 4 THEN 30",
		"WHEN 6 THEN 30",
		"WHEN 9 THEN 30",
		"WHEN 11 THEN 30",
		"CONSTRAINT path_models_overall_target_check CHECK",
		"overall_target_seconds IS NULL OR overall_target_seconds > 0",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("optional Path goals migration is missing %q", required)
		}
	}
	if strings.Contains(strings.ToLower(migration), "json") {
		t.Fatal("optional Path goals must use typed relational columns, not opaque JSON")
	}
}

func TestOptionalGoalDownMigrationRejectsDurableGoalStateBeforeDestruction(t *testing.T) {
	contents, err := fs.ReadFile(dbmigrations.Files, "000028_path_creation_goals.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(contents)
	preflight := "IF EXISTS (\n    SELECT 1\n    FROM public.path_models\n    WHERE interval_goal_target_seconds IS NOT NULL"
	firstDestructive := "ALTER TABLE public.path_models DROP CONSTRAINT path_models_interval_goal_check;"
	preflightAt, destructiveAt := strings.Index(migration, preflight), strings.Index(migration, firstDestructive)
	if preflightAt < 0 || destructiveAt < 0 || preflightAt > destructiveAt {
		t.Fatal("optional Path goals down migration must preflight durable goal state before dropping constraints")
	}
	for _, column := range []string{
		"interval_goal_target_seconds",
		"interval_goal_recurrence",
		"interval_goal_start_minute",
		"interval_goal_start_hour",
		"interval_goal_start_weekday",
		"interval_goal_start_day",
		"interval_goal_start_month",
		"overall_target_seconds",
	} {
		if !strings.Contains(migration[:destructiveAt], column+" IS NOT NULL") {
			t.Fatalf("rollback preflight does not protect %s", column)
		}
		if !strings.Contains(migration[destructiveAt:], "DROP COLUMN "+column) {
			t.Fatalf("rollback does not remove %s after the preflight", column)
		}
	}
	for _, forbidden := range []string{"DELETE FROM public.path_models", "TRUNCATE"} {
		if strings.Contains(strings.ToUpper(migration), strings.ToUpper(forbidden)) {
			t.Fatalf("rollback must not destroy Path rows: found %q", forbidden)
		}
	}
}

func TestPostgresOptionalGoalConstraintsRejectPartialAndInvalidConfigurations(t *testing.T) {
	if *migrationDatabaseDSN == "" {
		t.Skip("-migration-database-dsn is required for PostgreSQL integration")
	}
	database, err := gorm.Open(postgres.Open(*migrationDatabaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tests := []struct {
		name  string
		goals map[string]any
		valid bool
	}{
		{name: "neither goal", valid: true},
		{name: "overall only", goals: map[string]any{"overall_target_seconds": int64(1)}, valid: true},
		{name: "hourly lower bound", goals: intervalGoal("hourly", 1, "interval_goal_start_minute", int16(0)), valid: true},
		{name: "hourly upper bound", goals: intervalGoal("hourly", 1, "interval_goal_start_minute", int16(59)), valid: true},
		{name: "daily upper bound", goals: intervalGoal("daily", 1, "interval_goal_start_hour", int16(23)), valid: true},
		{name: "weekly upper bound", goals: intervalGoal("weekly", 1, "interval_goal_start_weekday", int16(7)), valid: true},
		{name: "monthly upper bound", goals: intervalGoal("monthly", 1, "interval_goal_start_day", int16(31)), valid: true},
		{name: "yearly leap day", goals: yearlyGoal(2, 29), valid: true},
		{name: "yearly thirty day month", goals: yearlyGoal(4, 30), valid: true},
		{name: "both goals", goals: withOverall(intervalGoal("weekly", 60, "interval_goal_start_weekday", int16(1)), 3600), valid: true},
		{name: "dangling recurrence", goals: map[string]any{"interval_goal_recurrence": "daily"}},
		{name: "target without recurrence", goals: map[string]any{"interval_goal_target_seconds": int64(1)}},
		{name: "zero interval target", goals: intervalGoal("daily", 0, "interval_goal_start_hour", int16(0))},
		{name: "unknown recurrence", goals: intervalGoal("custom", 1, "interval_goal_start_hour", int16(0))},
		{name: "hourly minute missing", goals: map[string]any{"interval_goal_target_seconds": int64(1), "interval_goal_recurrence": "hourly"}},
		{name: "hourly minute negative", goals: intervalGoal("hourly", 1, "interval_goal_start_minute", int16(-1))},
		{name: "hourly minute too large", goals: intervalGoal("hourly", 1, "interval_goal_start_minute", int16(60))},
		{name: "hourly carries daily alignment", goals: withAlignment(intervalGoal("hourly", 1, "interval_goal_start_minute", int16(0)), "interval_goal_start_hour", int16(0))},
		{name: "daily hour too large", goals: intervalGoal("daily", 1, "interval_goal_start_hour", int16(24))},
		{name: "weekly weekday zero", goals: intervalGoal("weekly", 1, "interval_goal_start_weekday", int16(0))},
		{name: "monthly day too large", goals: intervalGoal("monthly", 1, "interval_goal_start_day", int16(32))},
		{name: "yearly month zero", goals: yearlyGoal(0, 1)},
		{name: "yearly month too large", goals: yearlyGoal(13, 1)},
		{name: "yearly february thirtieth", goals: yearlyGoal(2, 30)},
		{name: "yearly april thirty first", goals: yearlyGoal(4, 31)},
		{name: "zero overall target", goals: map[string]any{"overall_target_seconds": int64(0)}},
		{name: "negative overall target", goals: map[string]any{"overall_target_seconds": int64(-1)}},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction := database.Begin()
			if transaction.Error != nil {
				t.Fatal(transaction.Error)
			}
			t.Cleanup(func() { _ = transaction.Rollback().Error })
			owner := "path-goal-migration-owner-" + strconv.Itoa(index)
			user := map[string]any{
				"id": owner, "status": identity.StatusActive,
				"created_at": now, "updated_at": now,
			}
			if err := transaction.Table("user_models").Create(user).Error; err != nil {
				t.Fatal(err)
			}
			row := map[string]any{
				"id": "path-goal-migration-" + strconv.Itoa(index), "owner_user_id": owner,
				"name": "Goal migration", "visibility": "private", "created_at": now, "updated_at": now,
			}
			for column, value := range test.goals {
				row[column] = value
			}
			err := transaction.Table("path_models").Create(row).Error
			if test.valid && err != nil {
				t.Fatalf("valid goal configuration was rejected: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("invalid goal configuration was persisted")
			}
		})
	}
}

func intervalGoal(recurrence string, target int64, alignmentColumn string, alignment int16) map[string]any {
	return map[string]any{
		"interval_goal_target_seconds": target,
		"interval_goal_recurrence":     recurrence,
		alignmentColumn:                alignment,
	}
}

func yearlyGoal(month, day int16) map[string]any {
	goal := intervalGoal("yearly", 1, "interval_goal_start_month", month)
	goal["interval_goal_start_day"] = day
	return goal
}

func withAlignment(goal map[string]any, column string, value int16) map[string]any {
	goal[column] = value
	return goal
}

func withOverall(goal map[string]any, seconds int64) map[string]any {
	goal["overall_target_seconds"] = seconds
	return goal
}
