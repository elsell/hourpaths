package dbmigrations

import (
	"strings"
	"testing"
)

func TestPracticeCommentHeartMigrationPreservesBooleanStateReplayAndCancellableNotifications(t *testing.T) {
	if LatestVersion < 49 {
		t.Fatalf("latest version=%d", LatestVersion)
	}
	up, err := Files.ReadFile("000049_practice_comment_hearts.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(up)
	for _, required := range []string{"PRIMARY KEY (comment_id, actor_user_id)", "ON DELETE CASCADE", "social.practice_comment_heart.set", "social.practice_comment_heart.remove", "comment_heart", "comment_hearts", "created_at, actor_user_id", "GRANT SELECT, INSERT, DELETE ON public.social_practice_comment_heart_models TO app", "GRANT SELECT, INSERT ON public.social_practice_comment_heart_replay_models TO app"} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"GRANT UPDATE ON public.social_practice_comment_heart_models", "GRANT UPDATE ON public.social_practice_comment_heart_replay_models", "GRANT DELETE ON public.social_practice_comment_heart_replay_models"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration grants excessive privilege %q", forbidden)
		}
	}
	down, err := Files.ReadFile("000049_practice_comment_hearts.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	downSQL := string(down)
	guard := downSQL[:strings.Index(downSQL, "DROP INDEX")]
	for _, required := range []string{"social_practice_comment_heart_models", "social_practice_comment_heart_replay_models", "kind = 'comment_heart'"} {
		if !strings.Contains(guard, required) {
			t.Fatalf("rollback guard missing %q", required)
		}
	}
}
