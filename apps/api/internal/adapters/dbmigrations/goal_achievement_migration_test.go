package dbmigrations

import (
	"strings"
	"testing"
)

func TestGoalAchievementMigrationPreservesTypedTargetsAndCascadingFeedLifecycle(t *testing.T) {
	if LatestVersion < 50 {
		t.Fatalf("latest version=%d", LatestVersion)
	}
	up, err := Files.ReadFile("000050_goal_achievements.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(up)
	for _, required := range []string{
		"CREATE TABLE public.social_goal_achievement_models",
		"kind IN ('interval', 'overall')", "target_seconds > 0",
		"interval_started_at", "interval_ended_at", "ON DELETE CASCADE",
		"social_goal_achievement_interval_supported_idx",
		"social_goal_achievement_overall_supported_idx",
		"achievement_id text UNIQUE", "num_nonnulls(source_activity_id, achievement_id) = 1",
		"GRANT SELECT, INSERT, DELETE ON public.social_goal_achievement_models TO app",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(sql, "GRANT UPDATE ON public.social_goal_achievement_models") {
		t.Fatal("achievement identities must not be mutable")
	}

	down, err := Files.ReadFile("000050_goal_achievements.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	downSQL := string(down)
	if guard, destructive := strings.Index(downSQL, "SELECT 1 FROM public.social_goal_achievement_models"), strings.Index(downSQL, "DROP CONSTRAINT social_feed_event_models_achievement_id_fkey"); guard < 0 || destructive < 0 || guard > destructive {
		t.Fatal("rollback must reject durable achievement state before destructive statements")
	}
}
