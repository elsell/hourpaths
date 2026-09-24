package dbmigrations

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestPriorReleaseV44InventoryMatchesFrozenMigrations(t *testing.T) {
	inventory, err := os.ReadFile("prior-release-v44.sha256")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyMigrationInventory(Files, inventory, 1, 44); err != nil {
		t.Fatal(err)
	}
}

func TestSocialFeedMigrationMaterializesPracticeEventsSafely(t *testing.T) {
	if LatestVersion < 45 {
		t.Fatalf("latest migration version = %d, want social feed migration 45 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000045_social_practice_feed_events.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"LOCK TABLE public.recorded_activity_models IN SHARE ROW EXCLUSIVE MODE;",
		"CREATE TABLE public.social_feed_event_models",
		"source_activity_id text NOT NULL UNIQUE",
		"participant_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE",
		"path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE",
		"published_at timestamptz NOT NULL",
		"FOREIGN KEY (source_activity_id, participant_user_id, path_id)",
		"REFERENCES public.recorded_activity_models(id, participant_id, path_id)",
		"CREATE FUNCTION public.publish_practice_feed_event()",
		"CREATE TRIGGER recorded_activity_publish_feed_event",
		"SELECT 'practice:' || activity.id, activity.id, activity.participant_id, activity.path_id, activity.created_at",
		"REVOKE ALL ON public.social_feed_event_models FROM app;",
		"GRANT SELECT, INSERT ON public.social_feed_event_models TO app;",
		"COMMIT;",
	})
	if strings.Count(migration, "ON DELETE CASCADE") != 3 {
		t.Fatal("social feed event must cascade with its source activity, participant, and Path")
	}
	if strings.Count(migration, "INSERT INTO public.social_feed_event_models") != 2 {
		t.Fatal("social feed migration must publish both future inserts and the historical backfill")
	}
	for _, forbidden := range []string{"note", "started_at", "ended_at", "GRANT UPDATE", "GRANT DELETE", "GRANT ALL"} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("social feed migration contains forbidden copied or mutable state %q", forbidden)
		}
	}
}

func TestSocialFeedDownMigrationRefusesDestructiveRollback(t *testing.T) {
	down, err := fs.ReadFile(Files, "000045_social_practice_feed_events.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(down)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"IF EXISTS (SELECT 1 FROM public.social_feed_event_models)",
		"RAISE EXCEPTION 'cannot remove social feed persistence while feed events exist';",
		"DROP TRIGGER recorded_activity_publish_feed_event",
		"DROP FUNCTION public.publish_practice_feed_event();",
		"DROP TABLE public.social_feed_event_models;",
		"COMMIT;",
	})
}
