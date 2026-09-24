package dbmigrations

import (
	"strings"
	"testing"
)

func TestPathDeletionMigrationSupportsStandaloneSnapshotsAndCascadesSupersededBatches(t *testing.T) {
	up, err := Files.ReadFile("000042_path_deletion.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(up)
	for _, required := range []string{"'path_deleted'", "ALTER COLUMN path_id DROP NOT NULL", "path_name_snapshot", "actor_username_snapshot", "actor_display_name_snapshot", "ON DELETE CASCADE"} {
		if !strings.Contains(contents, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	down, err := Files.ReadFile("000042_path_deletion.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "cannot remove Path deletion persistence while deletion notices exist") {
		t.Fatal("down migration lacks destructive preflight")
	}
}

func TestPathDeletionMigrationUsesStableAuthorizationConstraintNames(t *testing.T) {
	up, err := Files.ReadFile("000042_path_deletion.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := Files.ReadFile("000042_path_deletion.down.sql")
	if err != nil {
		t.Fatal(err)
	}

	const (
		legacyTruncatedTransferConstraint = "authorization_batch_outbox_models_path_ownership_transfer_id_fk"
		transferConstraint                = "authorization_batch_transfer_id_fkey"
		resourceConstraint                = "authorization_batch_resource_id_fkey"
	)
	upMigration := string(up)
	downMigration := string(down)
	if !strings.Contains(upMigration, "DROP CONSTRAINT IF EXISTS "+legacyTruncatedTransferConstraint) {
		t.Fatalf("up migration must drop PostgreSQL's truncated legacy constraint %q", legacyTruncatedTransferConstraint)
	}
	for _, constraint := range []string{transferConstraint, resourceConstraint} {
		if !strings.Contains(upMigration, "ADD CONSTRAINT "+constraint) {
			t.Errorf("up migration must add stable constraint %q", constraint)
		}
		if !strings.Contains(downMigration, "DROP CONSTRAINT "+constraint) {
			t.Errorf("down migration must drop stable constraint %q", constraint)
		}
		if !strings.Contains(downMigration, "ADD CONSTRAINT "+constraint) {
			t.Errorf("down migration must restore stable constraint %q", constraint)
		}
	}
	if strings.Contains(upMigration, "authorization_batch_outbox_models_path_ownership_transfer_id_fkey") ||
		strings.Contains(downMigration, "authorization_batch_outbox_models_path_ownership_transfer_id_fkey") {
		t.Fatal("migration must not rely on a PostgreSQL identifier longer than 63 bytes")
	}
}
