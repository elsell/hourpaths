package dbmigrations

import (
	"strings"
	"testing"
)

func TestOwnershipTransferReviewMigrationPersistsAuthoritativeReviewTimeAndProtectsEvidence(t *testing.T) {
	if LatestVersion < 40 {
		t.Fatalf("latest migration version = %d, want 40 or later", LatestVersion)
	}
	up, err := Files.ReadFile("000040_ownership_transfer_reviews.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := Files.ReadFile("000040_ownership_transfer_reviews.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"ADD COLUMN reviewed_at timestamptz", "SET reviewed_at = created_at", "ALTER COLUMN reviewed_at SET NOT NULL", "CHECK (reviewed_at <= created_at)"} {
		if !strings.Contains(string(up), required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	if !strings.Contains(string(down), "IF EXISTS (SELECT 1 FROM public.path_ownership_transfer_models)") || !strings.Contains(string(down), "RAISE EXCEPTION") {
		t.Fatal("down migration must refuse to discard ownership-transfer evidence")
	}
}
