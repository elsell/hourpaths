package dbmigrations

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestPublicPathAuthorizationMigrationFreezesPriorFeedRelease(t *testing.T) {
	for name, want := range map[string]string{
		"000045_social_practice_feed_events.down.sql": "169316774c54d86524b43aab38971d0c09b2f1ec9570aad137dd474f928e9715",
		"000045_social_practice_feed_events.up.sql":   "3ad76d7fc8425a548bfd1353c5b03c9ab893afcbbe2a323f4ae919ec029ab069",
	} {
		contents, err := Files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(contents)
		if got := hex.EncodeToString(digest[:]); got != want {
			t.Fatalf("frozen migration %s changed: got %s want %s", name, got, want)
		}
	}
}

func TestPublicPathViewAuthorizationMigrationBackfillsAndBridgesTheRuntimeCutover(t *testing.T) {
	if LatestVersion < 46 {
		t.Fatalf("latest version=%d want public Path authorization migration", LatestVersion)
	}
	up, err := Files.ReadFile("000046_public_path_view_authorization.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(up)
	for _, required := range []string{
		"LOCK TABLE public.path_models IN SHARE ROW EXCLUSIVE MODE",
		"CREATE FUNCTION public.publish_public_path_viewer_authorization()",
		"AFTER INSERT ON public.authorization_outbox_models",
		"NEW.id || '-public-viewer'",
		"path.visibility = 'public'",
		"relation, subject_type, subject_id",
		"'public_viewer', 'user', '*'",
		"migration 46 requires creator authorization evidence for every public Path",
		"JOIN LATERAL",
		"ON CONFLICT (id) DO NOTHING",
		"public Path authorization cutover identifier conflicts with another relationship",
		"migration 46 public Path authorization backfill did not converge",
	} {
		if !strings.Contains(contents, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Index(contents, "CREATE TRIGGER authorization_outbox_publish_public_path_viewer") > strings.LastIndex(contents, "INSERT INTO public.authorization_outbox_models") {
		t.Fatal("cutover bridge must be installed before the existing public Path backfill")
	}
}

func TestPublicPathViewAuthorizationRollbackRefusesAfterDeliveryMayHaveBegun(t *testing.T) {
	down, err := Files.ReadFile("000046_public_path_view_authorization.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(down)
	for _, required := range []string{
		"completed_at IS NOT NULL OR locked_until IS NOT NULL OR attempts > 0",
		"RAISE EXCEPTION 'cannot remove public Path view authorization after delivery may have begun'",
		"DELETE FROM public.authorization_outbox_models",
		"DROP TRIGGER authorization_outbox_publish_public_path_viewer",
	} {
		if !strings.Contains(contents, required) {
			t.Fatalf("rollback missing %q", required)
		}
	}
	if strings.Index(contents, "RAISE EXCEPTION") > strings.Index(contents, "DELETE FROM public.authorization_outbox_models") {
		t.Fatal("rollback destroys pending evidence before its fail-closed preflight")
	}
}
