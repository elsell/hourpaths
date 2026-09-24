package dbmigrations

import (
	"strings"
	"testing"
)

func TestPracticeEventCommentMigrationPreservesHistoryReplayAndCancellableNotifications(t *testing.T) {
	if LatestVersion < 48 {
		t.Fatalf("latest version=%d want practice comment migration", LatestVersion)
	}
	up, err := Files.ReadFile("000048_practice_event_comments.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(up)
	for _, required := range []string{
		"CREATE TABLE public.social_practice_comment_models",
		"REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE",
		"CREATE TABLE public.social_practice_comment_revision_models",
		"PRIMARY KEY (comment_id, version)",
		"CREATE TABLE public.social_practice_comment_replay_models",
		"social.practice_comment.create",
		"social.practice_comment.edit",
		"social.practice_comment.delete",
		"ADD COLUMN comment_id text REFERENCES public.social_practice_comment_models(id) ON DELETE CASCADE",
		"notification_models_active_practice_comment_idx",
		"WHERE kind = 'practice_comment' AND deleted_at IS NULL",
		"REVOKE ALL ON public.social_practice_comment_revision_models FROM app;",
		"GRANT SELECT, INSERT ON public.social_practice_comment_revision_models TO app;",
		"REVOKE ALL ON public.social_practice_comment_replay_models FROM app;",
		"GRANT SELECT, INSERT ON public.social_practice_comment_replay_models TO app;",
		"REVOKE ALL ON public.social_feed_event_models FROM app;",
		"GRANT SELECT, INSERT ON public.social_feed_event_models TO app;",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"GRANT UPDATE ON public.social_feed_event_models",
		"GRANT DELETE ON public.social_feed_event_models",
		"GRANT UPDATE ON public.social_practice_comment_revision_models",
		"GRANT DELETE ON public.social_practice_comment_revision_models",
		"GRANT UPDATE ON public.social_practice_comment_replay_models",
		"GRANT DELETE ON public.social_practice_comment_replay_models",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration grants excessive privilege %q", forbidden)
		}
	}

	down, err := Files.ReadFile("000048_practice_event_comments.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	downSQL := string(down)
	for _, required := range []string{
		"EXISTS (SELECT 1 FROM public.social_practice_comment_replay_models)",
		"DROP TABLE public.social_practice_comment_replay_models",
		"DROP TABLE public.social_practice_comment_revision_models",
		"DROP TABLE public.social_practice_comment_models",
		"REVOKE ALL ON public.social_feed_event_models FROM app;",
		"GRANT SELECT, INSERT ON public.social_feed_event_models TO app;",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
	guard := downSQL[:strings.Index(downSQL, "DROP INDEX")]
	if !strings.Contains(guard, "social_practice_comment_replay_models") {
		t.Fatal("down migration can destroy comment replay tombstones without refusing rollback")
	}
}
