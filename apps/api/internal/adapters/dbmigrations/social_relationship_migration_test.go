package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestSocialRelationshipMigrationAddsRaceSafeRelationshipsRequestsNotificationsAndReplay(t *testing.T) {
	if LatestVersion < 44 {
		t.Fatalf("latest migration version = %d, want social relationship migration 44 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000044_social_follow_relationships.up.sql")
	if err != nil {
		t.Fatalf("social relationship migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"ALTER TABLE public.follow_models",
		"activity_notifications_enabled boolean NOT NULL DEFAULT false",
		"authorization_change_id text UNIQUE REFERENCES public.authorization_outbox_models(id) ON DELETE RESTRICT",
		"CREATE TABLE public.follow_request_models",
		"requester_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE",
		"target_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE",
		"num_nonnulls(accepted_at, rejected_at, canceled_at) <= 1",
		"CREATE UNIQUE INDEX follow_request_models_one_pending_idx",
		"WHERE accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
		"CREATE INDEX follow_request_models_target_pending_idx",
		"created_at DESC, id DESC",
		"CREATE TABLE public.social_relationship_replay_models",
		"operation IN ('social.follow', 'social.follow_request.cancel', 'social.unfollow', 'social.follow_request.accept', 'social.follow_request.reject')",
		"request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32)",
		"authorization_change_id text REFERENCES public.authorization_outbox_models(id) ON DELETE RESTRICT",
		"changed boolean NOT NULL",
		"result_state text NOT NULL CHECK (result_state IN ('none', 'requested', 'following'))",
		"ADD COLUMN follow_request_id text REFERENCES public.follow_request_models(id) ON DELETE CASCADE",
		"ADD COLUMN follow_subject_user_id text REFERENCES public.user_models(id) ON DELETE CASCADE",
		"'new_follower', 'follow_request_received', 'follow_request_accepted'",
		"channel IN ('path_access', 'following')",
		"CREATE UNIQUE INDEX notification_models_follow_request_kind_recipient_idx",
		"GRANT SELECT, INSERT, DELETE ON public.follow_models TO app",
		"GRANT SELECT, INSERT, UPDATE ON public.follow_request_models TO app",
		"GRANT SELECT, INSERT ON public.social_relationship_replay_models TO app",
		"COMMIT;",
	})
	for _, table := range []string{"follow_request_models", "social_relationship_replay_models"} {
		if strings.Contains(migration, "GRANT ALL ON public."+table) ||
			strings.Contains(migration, "GRANT DELETE ON public."+table) ||
			strings.Contains(migration, "GRANT TRUNCATE ON public."+table) {
			t.Fatalf("social relationship migration overgrants mutable access to %s", table)
		}
	}
	if strings.Contains(migration, "GRANT UPDATE ON public.follow_models") ||
		strings.Contains(migration, "GRANT TRUNCATE ON public.follow_models") {
		t.Fatal("follow relationship persistence grants privileges beyond insert/delete lifecycle")
	}
}

func TestSocialRelationshipRollbackRefusesToDiscardRelationshipEvidence(t *testing.T) {
	down, err := fs.ReadFile(Files, "000044_social_follow_relationships.down.sql")
	if err != nil {
		t.Fatalf("social relationship down migration is not embedded: %v", err)
	}
	migration := string(down)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"DO $$",
		"FROM public.follow_request_models",
		"FROM public.social_relationship_replay_models",
		"FROM public.notification_models",
		"RAISE EXCEPTION 'cannot remove social relationship state while durable evidence exists';",
		"DROP INDEX public.notification_models_follow_request_kind_recipient_idx",
		"DROP TABLE public.social_relationship_replay_models",
		"DROP TABLE public.follow_request_models",
		"ALTER TABLE public.follow_models",
		"DROP COLUMN activity_notifications_enabled",
		"COMMIT;",
	})
	upper := strings.ToUpper(migration)
	if strings.Contains(upper, "DELETE FROM") || strings.Contains(upper, "TRUNCATE") {
		t.Fatal("social relationship rollback must not destroy durable evidence")
	}
}
