package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestPathInvitationMigrationAddsDurableInvitationsNotificationsAndPushOutbox(t *testing.T) {
	if LatestVersion < 32 {
		t.Fatalf("latest migration version = %d, want Path invitation migration 32 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000032_path_invitations.up.sql")
	if err != nil {
		t.Fatalf("Path invitation migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"CREATE TABLE public.path_invitation_models",
		"offered_role text NOT NULL",
		"offered_role IN ('participant', 'supporter')",
		"authorization_change_id text UNIQUE REFERENCES public.authorization_outbox_models(id) ON DELETE RESTRICT",
		"num_nonnulls(accepted_at, rejected_at, canceled_at) <= 1",
		"(accepted_at IS NULL) = (authorization_change_id IS NULL)",
		"CREATE UNIQUE INDEX path_invitation_models_one_pending_idx",
		"WHERE accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
		"CREATE TABLE public.notification_models",
		"kind IN ('path_invitation_received', 'path_invitation_accepted')",
		"presentation_class IN ('actionable', 'informational')",
		"channel = 'path_access'",
		"CREATE UNIQUE INDEX notification_models_invitation_kind_recipient_idx",
		"CREATE TABLE public.notification_push_outbox_models",
		"notification_id text PRIMARY KEY",
		"GRANT SELECT, INSERT, UPDATE ON public.path_invitation_models TO app",
		"GRANT SELECT, INSERT, UPDATE ON public.notification_models TO app",
		"GRANT SELECT, INSERT, UPDATE ON public.notification_push_outbox_models TO app",
		"COMMIT;",
	})
	for _, table := range []string{
		"path_invitation_models",
		"notification_models",
		"notification_push_outbox_models",
	} {
		if strings.Contains(migration, "GRANT ALL ON public."+table) ||
			strings.Contains(migration, "GRANT DELETE ON public."+table) ||
			strings.Contains(migration, "GRANT TRUNCATE ON public."+table) {
			t.Fatalf("Path invitation migration overgrants mutable access to %s", table)
		}
	}
}

func TestPathInvitationDownMigrationRefusesToDestroyDeliveryEvidence(t *testing.T) {
	down, err := fs.ReadFile(Files, "000032_path_invitations.down.sql")
	if err != nil {
		t.Fatalf("Path invitation down migration is not embedded: %v", err)
	}
	migration := string(down)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"DO $$",
		"FROM public.path_invitation_models",
		"FROM public.notification_models",
		"FROM public.notification_push_outbox_models",
		"RAISE EXCEPTION 'cannot remove Path invitation and notification state while delivery evidence exists';",
		"DROP TABLE public.notification_push_outbox_models",
		"DROP TABLE public.notification_models",
		"DROP TABLE public.path_invitation_models",
		"COMMIT;",
	})
	if strings.Contains(strings.ToUpper(migration), "DELETE FROM") ||
		strings.Contains(strings.ToUpper(migration), "TRUNCATE") {
		t.Fatal("Path invitation rollback must not destroy delivery evidence")
	}
}
