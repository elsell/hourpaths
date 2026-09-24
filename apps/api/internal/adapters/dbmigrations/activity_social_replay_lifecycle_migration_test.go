package dbmigrations

import (
	"strings"
	"testing"
)

func TestActivitySocialReplayLifecycleMigrationCleansOrphansAndCascadesWithFeedEvents(t *testing.T) {
	if LatestVersion < 61 {
		t.Fatalf("latest version=%d want activity social replay lifecycle migration", LatestVersion)
	}
	up, err := Files.ReadFile("000061_activity_social_replay_lifecycle.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(up)
	for _, required := range []string{
		"BEGIN;",
		"LOCK TABLE public.social_feed_event_models IN SHARE ROW EXCLUSIVE MODE;",
		"LOCK TABLE public.social_practice_comment_replay_models IN SHARE ROW EXCLUSIVE MODE;",
		"LOCK TABLE public.social_practice_comment_heart_replay_models IN SHARE ROW EXCLUSIVE MODE;",
		"DELETE FROM public.social_practice_comment_replay_models",
		"DELETE FROM public.social_practice_comment_heart_replay_models",
		"NOT EXISTS",
		"ADD COLUMN result_session_count bigint",
		"ADD COLUMN result_unread_notification_count bigint",
		"ADD COLUMN result_removed_feed_event_ids text[]",
		"UPDATE public.activity_mutation_models",
		"operation = 'activity.delete'",
		"result_removed_feed_event_ids = ARRAY['practice:' || mutation.result_activity_id]",
		"ADD CONSTRAINT activity_mutation_models_deletion_projection_receipt_check",
		"ADD CONSTRAINT social_practice_comment_replay_models_event_fkey",
		"ADD CONSTRAINT social_practice_comment_heart_replay_models_event_fkey",
		"REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE",
		"COMMIT;",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"GRANT DELETE", "TRUNCATE", "DROP TABLE"} {
		if strings.Contains(strings.ToUpper(sql), forbidden) {
			t.Fatalf("migration contains excessive operation %q", forbidden)
		}
	}
	receiptAlter := strings.Index(sql, "ALTER TABLE public.activity_mutation_models\n  ADD COLUMN")
	feedLock := strings.Index(sql, "LOCK TABLE public.social_feed_event_models")
	if receiptAlter < 0 || feedLock < 0 || receiptAlter > feedLock {
		t.Fatal("receipt ALTER/backfill must precede feed/replay locks to preserve runtime lock order")
	}

	down, err := Files.ReadFile("000061_activity_social_replay_lifecycle.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	downSQL := string(down)
	for _, required := range []string{
		"BEGIN;",
		"LOCK TABLE public.activity_mutation_models IN ACCESS EXCLUSIVE MODE;",
		"DROP CONSTRAINT social_practice_comment_replay_models_event_fkey",
		"DROP CONSTRAINT social_practice_comment_heart_replay_models_event_fkey",
		"cannot remove activity deletion feed-event receipts while deletion evidence exists",
		"DROP CONSTRAINT activity_mutation_models_deletion_projection_receipt_check",
		"DROP COLUMN result_session_count",
		"DROP COLUMN result_unread_notification_count",
		"DROP COLUMN result_removed_feed_event_ids",
		"COMMIT;",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"DELETE FROM", "TRUNCATE", "DROP TABLE"} {
		if strings.Contains(strings.ToUpper(downSQL), forbidden) {
			t.Fatalf("down migration destroys replay evidence with %q", forbidden)
		}
	}
	receiptLock := strings.Index(downSQL, "LOCK TABLE public.activity_mutation_models")
	childDDL := strings.Index(downSQL, "ALTER TABLE public.social_practice_comment_heart_replay_models")
	if receiptLock < 0 || childDDL < 0 || receiptLock > childDDL {
		t.Fatal("down migration must lock deletion receipts before child lifecycle DDL")
	}
}
