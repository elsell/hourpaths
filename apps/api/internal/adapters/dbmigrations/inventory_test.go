package dbmigrations

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPriorReleaseV17InventoryMatchesFrozenMigrations(t *testing.T) {
	inventory, err := os.ReadFile("prior-release-v17.sha256")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyMigrationInventory(Files, inventory, 1, 17); err != nil {
		t.Fatal(err)
	}
}

func TestPriorReleaseV20InventoryMatchesFrozenMigrations(t *testing.T) {
	inventory, err := os.ReadFile("prior-release-v20.sha256")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyMigrationInventory(Files, inventory, 1, 20); err != nil {
		t.Fatal(err)
	}
}

func TestPriorReleaseV21InventoryMatchesFrozenMigrations(t *testing.T) {
	inventory, err := os.ReadFile("prior-release-v21.sha256")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyMigrationInventory(Files, inventory, 1, 21); err != nil {
		t.Fatal(err)
	}
}

func TestPriorReleaseV22InventoryMatchesFrozenMigrations(t *testing.T) {
	inventory, err := os.ReadFile("prior-release-v22.sha256")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyMigrationInventory(Files, inventory, 1, 22); err != nil {
		t.Fatal(err)
	}
}

func TestPriorReleaseV23InventoryMatchesFrozenMigrations(t *testing.T) {
	inventory, err := os.ReadFile("prior-release-v23.sha256")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyMigrationInventory(Files, inventory, 1, 23); err != nil {
		t.Fatal(err)
	}
}

func TestPriorReleaseV24InventoryMatchesFrozenMigrations(t *testing.T) {
	inventory, err := os.ReadFile("prior-release-v24.sha256")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyMigrationInventory(Files, inventory, 1, 24); err != nil {
		t.Fatal(err)
	}
}

func TestPathCreatorMembershipMigrationIsLeastPrivilege(t *testing.T) {
	if LatestVersion < 26 {
		t.Fatalf("latest migration version = %d, want path creator membership migration 26 or later", LatestVersion)
	}
	up, err := fs.ReadFile(Files, "000026_path_creator_membership.up.sql")
	if err != nil {
		t.Fatalf("path creator membership migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(up), []string{
		"BEGIN;",
		"REVOKE ALL ON public.path_membership_models FROM app;",
		"GRANT SELECT, INSERT ON public.path_membership_models TO app;",
		"COMMIT;",
	})
	if strings.Contains(string(up), "GRANT ALL") {
		t.Fatal("path membership migration granted unrestricted runtime access")
	}

	down, err := fs.ReadFile(Files, "000026_path_creator_membership.down.sql")
	if err != nil {
		t.Fatalf("path creator membership down migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(down), []string{
		"BEGIN;",
		"REVOKE INSERT ON public.path_membership_models FROM app;",
		"COMMIT;",
	})
}

func TestAuthorizationOutboxOrderingMigrationIsCurrent(t *testing.T) {
	contents, err := fs.ReadFile(Files, "000018_authorization_outbox_ordering.up.sql")
	if err != nil {
		t.Fatalf("authorization ordering migration is not embedded: %v", err)
	}
	orderedStatements := []string{
		"BEGIN;",
		"LOCK TABLE public.authorization_outbox_models IN SHARE ROW EXCLUSIVE MODE;",
		"DO $$",
		"CREATE FUNCTION public.order_authorization_outbox_change()",
		"CREATE TRIGGER authorization_outbox_ordering",
		"COMMIT;",
	}
	position := -1
	for _, statement := range orderedStatements {
		next := strings.Index(string(contents), statement)
		if next <= position {
			t.Fatalf("authorization ordering migration statement %q is missing or out of order", statement)
		}
		position = next
	}
}

func TestProvisionalUserStatusMigrationIsCurrent(t *testing.T) {
	contents, err := fs.ReadFile(Files, "000019_provisional_user_status.up.sql")
	if err != nil {
		t.Fatalf("provisional user status migration is not embedded: %v", err)
	}
	for _, statement := range []string{
		"BEGIN;",
		"DROP CONSTRAINT user_models_status_check",
		"CHECK (status IN ('provisional', 'active', 'disabled'))",
		"ALTER COLUMN status SET DEFAULT 'provisional'",
		"COMMIT;",
	} {
		if !strings.Contains(string(contents), statement) {
			t.Fatalf("provisional user status migration is missing %q", statement)
		}
	}
}

func TestDuplicateEmailRecoveryMigrationIsCurrent(t *testing.T) {
	up, err := fs.ReadFile(Files, "000020_duplicate_email_recovery.up.sql")
	if err != nil {
		t.Fatalf("duplicate email recovery migration is not embedded: %v", err)
	}
	upStatements := []string{
		"BEGIN;",
		"DROP INDEX public.user_models_normalized_email_unique_idx;",
		"CREATE INDEX user_models_normalized_email_lookup_idx",
		"ON public.user_models (lower(email))",
		"WHERE email <> '';",
		"COMMIT;",
	}
	assertMigrationStatementsInOrder(t, string(up), upStatements)
	if strings.Contains(string(up), "CREATE UNIQUE INDEX") {
		t.Fatal("duplicate email recovery migration retained email uniqueness")
	}

	down, err := fs.ReadFile(Files, "000020_duplicate_email_recovery.down.sql")
	if err != nil {
		t.Fatalf("duplicate email recovery down migration is not embedded: %v", err)
	}
	downStatements := []string{
		"BEGIN;",
		"IF EXISTS",
		"GROUP BY lower(email)",
		"HAVING count(*) > 1",
		"RAISE EXCEPTION",
		"DROP INDEX public.user_models_normalized_email_lookup_idx;",
		"CREATE UNIQUE INDEX user_models_normalized_email_unique_idx",
		"ON public.user_models (lower(email))",
		"WHERE email <> '';",
		"COMMIT;",
	}
	assertMigrationStatementsInOrder(t, string(down), downStatements)
}

func TestDuplicateEmailRecoveryDeclineMigrationIsCurrent(t *testing.T) {
	up, err := fs.ReadFile(Files, "000021_duplicate_email_recovery_declines.up.sql")
	if err != nil {
		t.Fatalf("duplicate email recovery decline migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(up), []string{
		"BEGIN;",
		"ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;",
		"ALTER TABLE audit_event_models ADD CONSTRAINT audit_event_models_action_check CHECK",
		"'duplicate_email_recovery.declined'",
		"CREATE TABLE duplicate_email_recovery_declines",
		"provisional_user_id text NOT NULL REFERENCES user_models(id) ON DELETE CASCADE",
		"normalized_email text NOT NULL",
		"CHECK (normalized_email <> '' AND normalized_email = lower(btrim(normalized_email)))",
		"PRIMARY KEY (provisional_user_id, normalized_email)",
		"REVOKE UPDATE, DELETE, TRUNCATE ON duplicate_email_recovery_declines FROM app;",
		"GRANT SELECT, INSERT ON duplicate_email_recovery_declines TO app;",
		"COMMIT;",
	})

	down, err := fs.ReadFile(Files, "000021_duplicate_email_recovery_declines.down.sql")
	if err != nil {
		t.Fatalf("duplicate email recovery decline down migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(down), []string{
		"BEGIN;",
		"DO $$",
		"WHERE action = 'duplicate_email_recovery.declined'",
		"RAISE EXCEPTION 'cannot remove duplicate email recovery declines while their audit events exist';",
		"ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;",
		"ALTER TABLE audit_event_models ADD CONSTRAINT audit_event_models_action_check CHECK",
		"REVOKE ALL ON duplicate_email_recovery_declines FROM app;",
		"DROP TABLE duplicate_email_recovery_declines;",
		"COMMIT;",
	})
	if strings.Count(string(up), "'duplicate_email_recovery.declined'") != 1 {
		t.Fatal("duplicate email recovery decline up migration must admit exactly one new audit action")
	}
	if strings.Count(string(down), "'duplicate_email_recovery.declined'") != 1 {
		t.Fatal("duplicate email recovery decline down migration must reference the removed audit action only in its rollback preflight")
	}
	wantRestoredConstraint, err := fs.ReadFile(Files, "000013_user_viewed_audit.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	wantAdd := strings.Split(strings.TrimSpace(string(wantRestoredConstraint)), "\n")[1]
	if !strings.Contains(string(down), wantAdd) {
		t.Fatal("duplicate email recovery decline down migration did not restore the exact prior audit action constraint")
	}
	wantExpandedAdd := strings.TrimSuffix(wantAdd, "));") + ", 'duplicate_email_recovery.declined'));"
	if !strings.Contains(string(up), wantExpandedAdd) {
		t.Fatal("duplicate email recovery decline up migration did not preserve the exact prior audit action constraint")
	}
}

func TestActiveUsernameMigrationIsCurrent(t *testing.T) {
	up, err := fs.ReadFile(Files, "000022_active_usernames.up.sql")
	if err != nil {
		t.Fatalf("active username migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(up), []string{
		"BEGIN;",
		"ALTER TABLE public.user_models ADD COLUMN username text NULL;",
		"ADD CONSTRAINT user_models_provisional_username_check",
		"status <> 'provisional' OR username IS NULL",
		"ADD CONSTRAINT user_models_username_format_check",
		"username IS NULL OR (",
		"char_length(username) BETWEEN 3 AND 64",
		"username ~ '^[A-Za-z0-9_.]+$'",
		"CREATE UNIQUE INDEX user_models_normalized_username_unique_idx",
		"ON public.user_models (translate(username, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'))",
		"WHERE username IS NOT NULL;",
		"COMMIT;",
	})

	down, err := fs.ReadFile(Files, "000022_active_usernames.down.sql")
	if err != nil {
		t.Fatalf("active username down migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(down), []string{
		"BEGIN;",
		"DO $$",
		"WHERE username IS NOT NULL",
		"RAISE EXCEPTION 'cannot remove active usernames while username data exists';",
		"DROP INDEX public.user_models_normalized_username_unique_idx;",
		"DROP CONSTRAINT user_models_provisional_username_check",
		"DROP CONSTRAINT user_models_username_format_check",
		"DROP COLUMN username;",
		"COMMIT;",
	})
}

func TestAtomicAccountActivationMigrationIsCurrent(t *testing.T) {
	up, err := fs.ReadFile(Files, "000023_atomic_account_activation.up.sql")
	if err != nil {
		t.Fatalf("atomic account activation migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(up), []string{
		"BEGIN;",
		"ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;",
		"'user.onboarding_completed'",
		"ALTER TABLE public.user_models ADD COLUMN profile_visibility text NULL",
		"CREATE TABLE public.user_account_activation_models",
		"minimum_age_attested smallint NOT NULL CHECK (minimum_age_attested = 16)",
		"CREATE TABLE public.user_policy_acceptance_models",
		"policy text NOT NULL CHECK (policy IN ('terms', 'privacy', 'community_guidelines'))",
		"acknowledgement text NOT NULL CHECK (acknowledgement IN ('accepted', 'acknowledged'))",
		"CREATE TABLE public.user_preference_models",
		"first_day_of_week smallint NOT NULL CHECK (first_day_of_week BETWEEN 1 AND 7)",
		"CREATE TABLE public.user_time_zone_history_models",
		"CREATE FUNCTION public.enforce_complete_account_activation()",
		"OLD.status = 'provisional' AND NEW.status = 'active'",
		"accepted_at <= activation.completed_at",
		"CREATE TRIGGER user_models_complete_account_activation",
		"CREATE FUNCTION public.enforce_active_onboarding_owner()",
		"CREATE CONSTRAINT TRIGGER user_account_activation_owner_active",
		"DEFERRABLE INITIALLY DEFERRED",
		"CREATE CONSTRAINT TRIGGER user_policy_acceptance_owner_active",
		"CREATE CONSTRAINT TRIGGER user_preference_owner_active",
		"CREATE CONSTRAINT TRIGGER user_time_zone_history_owner_active",
		"REVOKE ALL ON public.user_account_activation_models FROM app;",
		"GRANT SELECT, INSERT ON public.user_account_activation_models TO app;",
		"REVOKE ALL ON public.user_policy_acceptance_models FROM app;",
		"GRANT SELECT, INSERT ON public.user_policy_acceptance_models TO app;",
		"REVOKE ALL ON public.user_time_zone_history_models FROM app;",
		"GRANT SELECT, INSERT ON public.user_time_zone_history_models TO app;",
		"REVOKE ALL ON public.user_preference_models FROM app;",
		"GRANT SELECT, INSERT ON public.user_preference_models TO app;",
		"COMMIT;",
	})

	down, err := fs.ReadFile(Files, "000023_atomic_account_activation.down.sql")
	if err != nil {
		t.Fatalf("atomic account activation down migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(down), []string{
		"BEGIN;",
		"DO $$",
		"IF EXISTS (SELECT 1 FROM public.user_account_activation_models)",
		"RAISE EXCEPTION 'cannot remove account activation persistence while activation data exists';",
		"DROP TRIGGER user_account_activation_owner_active ON public.user_account_activation_models;",
		"DROP TRIGGER user_policy_acceptance_owner_active ON public.user_policy_acceptance_models;",
		"DROP TRIGGER user_preference_owner_active ON public.user_preference_models;",
		"DROP TRIGGER user_time_zone_history_owner_active ON public.user_time_zone_history_models;",
		"DROP FUNCTION public.enforce_active_onboarding_owner();",
		"DROP TRIGGER user_models_complete_account_activation ON public.user_models;",
		"DROP FUNCTION public.enforce_complete_account_activation();",
		"DROP TABLE public.user_time_zone_history_models;",
		"DROP TABLE public.user_preference_models;",
		"DROP TABLE public.user_policy_acceptance_models;",
		"DROP TABLE public.user_account_activation_models;",
		"ALTER TABLE public.user_models DROP COLUMN profile_visibility;",
		"ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;",
		"ALTER TABLE audit_event_models ADD CONSTRAINT audit_event_models_action_check CHECK",
		"COMMIT;",
	})
	if strings.Count(string(up), "'user.onboarding_completed'") != 1 {
		t.Fatal("atomic account activation up migration must admit exactly one audit action")
	}
	if strings.Count(string(down), "'user.onboarding_completed'") != 1 {
		t.Fatal("atomic account activation down migration must reference the removed audit action only in rollback preflight")
	}
	prior, err := fs.ReadFile(Files, "000021_duplicate_email_recovery_declines.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	priorConstraint := strings.Split(strings.TrimSpace(string(prior)), "\n")[3]
	if !strings.Contains(string(down), priorConstraint) {
		t.Fatal("atomic account activation down migration did not restore the exact version-22 audit action constraint")
	}
	expandedConstraint := strings.TrimSuffix(priorConstraint, "));") + ", 'user.onboarding_completed'));"
	if !strings.Contains(string(up), expandedConstraint) {
		t.Fatal("atomic account activation up migration did not preserve the exact version-22 audit action constraint")
	}
}

func TestCurrentPolicyAuthorityMigrationIsCurrent(t *testing.T) {
	up, err := fs.ReadFile(Files, "000024_current_policy_authority.up.sql")
	if err != nil {
		t.Fatalf("current policy authority migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(up), []string{
		"BEGIN;",
		"CREATE TABLE public.current_policy_set_models",
		"singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton)",
		"revision bigint NOT NULL CHECK (revision > 0)",
		"terms_version text NOT NULL CHECK (btrim(terms_version) <> '' AND char_length(terms_version) <= 128)",
		"privacy_policy_version text NOT NULL CHECK (btrim(privacy_policy_version) <> '' AND char_length(privacy_policy_version) <= 128)",
		"community_guidelines_version text NOT NULL CHECK (btrim(community_guidelines_version) <> '' AND char_length(community_guidelines_version) <= 128)",
		"terms_url text NOT NULL CHECK (btrim(terms_url) <> '' AND char_length(terms_url) <= 2048)",
		"privacy_policy_url text NOT NULL CHECK (btrim(privacy_policy_url) <> '' AND char_length(privacy_policy_url) <= 2048)",
		"community_guidelines_url text NOT NULL CHECK (btrim(community_guidelines_url) <> '' AND char_length(community_guidelines_url) <= 2048)",
		"support_url text NOT NULL CHECK (btrim(support_url) <> '' AND char_length(support_url) <= 2048)",
		"updated_at timestamptz NOT NULL",
		"ALTER TABLE public.user_account_activation_models",
		"ADD COLUMN policy_set_revision bigint NULL",
		"CREATE OR REPLACE FUNCTION public.enforce_complete_account_activation()",
		"activation.policy_set_revision = authority.revision",
		"acceptance.policy = 'terms'",
		"acceptance.version = authority.terms_version",
		"acceptance.policy = 'privacy'",
		"acceptance.version = authority.privacy_policy_version",
		"acceptance.policy = 'community_guidelines'",
		"acceptance.version = authority.community_guidelines_version",
		"REVOKE ALL ON public.current_policy_set_models FROM app;",
		"GRANT SELECT ON public.current_policy_set_models TO app;",
		"COMMIT;",
	})

	down, err := fs.ReadFile(Files, "000024_current_policy_authority.down.sql")
	if err != nil {
		t.Fatalf("current policy authority down migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(down), []string{
		"BEGIN;",
		"DO $$",
		"IF EXISTS (SELECT 1 FROM public.current_policy_set_models)",
		"OR EXISTS (SELECT 1 FROM public.user_account_activation_models WHERE policy_set_revision IS NOT NULL)",
		"RAISE EXCEPTION 'cannot remove current policy authority while policy authority or revision evidence exists';",
		"CREATE OR REPLACE FUNCTION public.enforce_complete_account_activation()",
		"ALTER TABLE public.user_account_activation_models DROP COLUMN policy_set_revision;",
		"REVOKE ALL ON public.current_policy_set_models FROM app;",
		"DROP TABLE public.current_policy_set_models;",
		"COMMIT;",
	})
}

func TestProviderEmailVerificationMigrationIsCurrent(t *testing.T) {
	up, err := fs.ReadFile(Files, "000025_provider_email_verification.up.sql")
	if err != nil {
		t.Fatalf("provider email verification migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(up), []string{
		"BEGIN;",
		"ALTER TABLE public.user_models",
		"ADD COLUMN provider_email_verified boolean NOT NULL DEFAULT false",
		"ADD CONSTRAINT user_models_verified_provider_email_check",
		"NOT provider_email_verified OR btrim(email) <> ''",
		"COMMIT;",
	})
	if strings.Contains(string(up), "UPDATE public.user_models") {
		t.Fatal("provider email verification migration must not trust legacy email rows")
	}

	down, err := fs.ReadFile(Files, "000025_provider_email_verification.down.sql")
	if err != nil {
		t.Fatalf("provider email verification down migration is not embedded: %v", err)
	}
	assertMigrationStatementsInOrder(t, string(down), []string{
		"BEGIN;",
		"IF EXISTS (SELECT 1 FROM public.user_models WHERE provider_email_verified)",
		"RAISE EXCEPTION 'cannot remove provider email verification provenance while verified provider emails exist';",
		"DROP CONSTRAINT user_models_verified_provider_email_check",
		"DROP COLUMN provider_email_verified;",
		"COMMIT;",
	})
}

func assertMigrationStatementsInOrder(t *testing.T, contents string, statements []string) {
	t.Helper()
	position := -1
	for _, statement := range statements {
		next := strings.Index(contents, statement)
		if next <= position {
			t.Fatalf("migration statement %q is missing or out of order", statement)
		}
		position = next
	}
}

func TestVerifyMigrationInventoryFailsClosed(t *testing.T) {
	files := inventoryFixtureFiles(1, 2)
	inventory := inventoryFixture(t, files)
	filesWithDuplicate := inventoryFixtureFiles(1, 2)
	filesWithDuplicate["000001_duplicate.up.sql"] = &fstest.MapFile{Data: []byte("duplicate\n")}
	filesWithCurrent := inventoryFixtureFiles(1, 2)
	currentName := migrationFixtureName(17, "up")
	currentContents := []byte(currentName + " contents\n")
	currentDigest := sha256.Sum256(currentContents)
	filesWithCurrent[currentName] = &fstest.MapFile{Data: currentContents}

	tests := map[string]struct {
		files     fs.FS
		inventory string
		wantError string
	}{
		"migration mutation": {
			files: fstest.MapFS{
				"000001_fixture.up.sql":   {Data: []byte("mutated")},
				"000001_fixture.down.sql": files["000001_fixture.down.sql"],
				"000002_fixture.up.sql":   files["000002_fixture.up.sql"],
				"000002_fixture.down.sql": files["000002_fixture.down.sql"],
			},
			inventory: inventory,
			wantError: "has changed",
		},
		"version gap": {
			files:     files,
			inventory: inventoryWithoutVersion(inventory, "000001"),
			wantError: "incomplete at version 1",
		},
		"duplicate entry": {
			files:     files,
			inventory: inventory + strings.Split(inventory, "\n")[0] + "\n",
			wantError: "duplicate migration inventory entry",
		},
		"uninventoried duplicate migration": {
			files:     filesWithDuplicate,
			inventory: inventory,
			wantError: "is absent from the inventory",
		},
		"malformed digest": {
			files:     files,
			inventory: "not-a-sha256  000001_fixture.up.sql\n" + inventory,
			wantError: "malformed migration inventory line",
		},
		"current migration included": {
			files:     filesWithCurrent,
			inventory: inventory + fmt.Sprintf("%x  %s\n", currentDigest, currentName),
			wantError: "out-of-range version 17",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := VerifyMigrationInventory(test.files, []byte(test.inventory), 1, 2)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("VerifyMigrationInventory() error = %v, want error containing %q", err, test.wantError)
			}
		})
	}
}

func inventoryFixtureFiles(firstVersion, lastVersion int) fstest.MapFS {
	files := fstest.MapFS{}
	for version := firstVersion; version <= lastVersion; version++ {
		for _, direction := range []string{"down", "up"} {
			name := migrationFixtureName(version, direction)
			files[name] = &fstest.MapFile{Data: []byte(name + " contents\n")}
		}
	}
	return files
}

func inventoryFixture(t *testing.T, files fstest.MapFS) string {
	t.Helper()
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	var inventory strings.Builder
	for _, name := range names {
		contents, err := fs.ReadFile(files, name)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(contents)
		inventory.WriteString(hex.EncodeToString(digest[:]))
		inventory.WriteString("  ")
		inventory.WriteString(name)
		inventory.WriteByte('\n')
	}
	return inventory.String()
}

func inventoryWithoutVersion(inventory, version string) string {
	var result strings.Builder
	for _, line := range strings.SplitAfter(inventory, "\n") {
		if !strings.Contains(line, "  "+version+"_") {
			result.WriteString(line)
		}
	}
	return result.String()
}

func migrationFixtureName(version int, direction string) string {
	return fmt.Sprintf("%06d_fixture.%s.sql", version, direction)
}
