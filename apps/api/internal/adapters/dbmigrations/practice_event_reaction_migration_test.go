package dbmigrations

import (
	"strings"
	"testing"
)

func TestPracticeEventReactionMigrationEnforcesOneCuratedReactionAndCancellableNotification(t *testing.T) {
	if LatestVersion < 47 {
		t.Fatalf("latest version=%d", LatestVersion)
	}
	up, err := Files.ReadFile("000047_practice_event_reactions.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(up)
	for _, required := range []string{
		"PRIMARY KEY (social_feed_event_id, actor_user_id)",
		"'heart', 'applause', 'fire', 'strong', 'celebrate'",
		"social.practice_reaction.set", "social.practice_reaction.remove",
		"notification_models_active_practice_reaction_idx",
		"WHERE kind = 'practice_reaction' AND deleted_at IS NULL",
		"REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE",
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
		"GRANT ALL ON public.social_feed_event_models",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration grants excessive feed-event privilege %q", forbidden)
		}
	}

	down, err := Files.ReadFile("000047_practice_event_reactions.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	downSQL := string(down)
	for _, required := range []string{
		"REVOKE ALL ON public.social_feed_event_models FROM app;",
		"GRANT SELECT, INSERT ON public.social_feed_event_models TO app;",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("down migration does not restore feed-event privilege %q", required)
		}
	}
}
