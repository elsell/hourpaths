package dbmigrations

import (
	"io/fs"
	"strings"
	"testing"
)

func TestPushPersistenceMigrationProtectsTokensAndSupportsPerInstallationClaims(t *testing.T) {
	if LatestVersion < 33 {
		t.Fatalf("latest migration version = %d, want push persistence migration 33 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000033_push_persistence.up.sql")
	if err != nil {
		t.Fatalf("push persistence migration is not embedded: %v", err)
	}
	migration := string(up)
	assertMigrationStatementsInOrder(t, migration, []string{
		"BEGIN;",
		"CREATE TABLE public.push_installation_models",
		"token_ciphertext bytea",
		"token_nonce bytea",
		"token_hash bytea",
		"CREATE UNIQUE INDEX push_installation_models_active_token_idx",
		"WHERE deleted_at IS NULL",
		"CREATE TABLE public.notification_push_delivery_models",
		"PRIMARY KEY (notification_id, installation_id)",
		"CHECK (delivered_at IS NULL OR delivered_at >= created_at)",
		"CHECK (suppressed_at IS NULL OR failure_code <> '')",
		"CREATE FUNCTION public.validate_push_delivery_installation()",
		"i.owner_user_id = NEW.recipient_user_id",
		"i.deleted_at IS NULL",
		"CREATE INDEX notification_push_delivery_models_claim_idx",
		"GRANT SELECT, INSERT, UPDATE ON public.push_installation_models TO app",
		"GRANT SELECT, INSERT, UPDATE ON public.notification_push_delivery_models TO app",
		"COMMIT;",
	})
	if strings.Contains(migration, "token text") || strings.Contains(migration, "token varchar") {
		t.Fatal("push tokens must never be stored as plaintext")
	}
}

func TestPushPersistenceDownMigrationRefusesToDestroyDeliveryEvidence(t *testing.T) {
	down, err := fs.ReadFile(Files, "000033_push_persistence.down.sql")
	if err != nil {
		t.Fatalf("push persistence down migration is not embedded: %v", err)
	}
	migration := string(down)
	if !strings.Contains(migration, "cannot remove push persistence while delivery evidence exists") {
		t.Fatal("rollback must refuse to destroy push delivery evidence")
	}
}
