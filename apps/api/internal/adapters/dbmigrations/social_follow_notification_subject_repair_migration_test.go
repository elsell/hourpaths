package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestSocialFollowNotificationSubjectRepairMigrationRepairsAndConstrainsActorIdentity(t *testing.T) {
	if LatestVersion < 52 {
		t.Fatalf("latest migration version = %d, want follow-notification repair migration 52 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000052_social_follow_notification_subject_repair.up.sql")
	if err != nil {
		t.Fatalf("follow-notification repair migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"ADD CONSTRAINT notification_models_follow_subject_actor_check CHECK",
		"OR follow_subject_user_id = actor_user_id",
		") NOT VALID",
		"UPDATE public.notification_models",
		"SET follow_subject_user_id = actor_user_id",
		"kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted')",
		"follow_subject_user_id IS DISTINCT FROM actor_user_id",
		"VALIDATE CONSTRAINT notification_models_follow_subject_actor_check",
		"COMMIT;",
	})
}

func TestSocialFollowNotificationSubjectRepairRollbackPreservesRepairedEvidence(t *testing.T) {
	down, err := fs.ReadFile(Files, "000052_social_follow_notification_subject_repair.down.sql")
	if err != nil {
		t.Fatalf("follow-notification repair rollback is not embedded: %v", err)
	}
	migration := string(down)
	if !strings.Contains(migration, "DROP CONSTRAINT notification_models_follow_subject_actor_check") {
		t.Fatal("rollback must remove only the forward invariant constraint")
	}
	upper := strings.ToUpper(migration)
	if strings.Contains(upper, "UPDATE PUBLIC.NOTIFICATION_MODELS") || strings.Contains(upper, "DELETE FROM") || strings.Contains(upper, "TRUNCATE") {
		t.Fatal("rollback must preserve repaired notification evidence")
	}
}
