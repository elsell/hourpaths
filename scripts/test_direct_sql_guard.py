#!/usr/bin/env python3
"""Behavioral tests for the fail-closed direct SQL structure guard."""

from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path


REPOSITORY = Path(__file__).resolve().parents[1]
GUARD = REPOSITORY / "scripts" / "check-direct-sql.py"

ACTIVATION = "apps/api/internal/adapters/gormstore/activation_repository.go"
POLICY = "apps/api/internal/adapters/gormstore/policy_authority.go"
SOCIAL_RELATIONSHIPS = "apps/api/internal/adapters/gormstore/social_relationship_repository.go"
SOCIAL_FEED = "apps/api/internal/adapters/gormstore/social_feed_repository.go"
SOCIAL_INTERACTION_SETTINGS = "apps/api/internal/adapters/gormstore/social_interaction_settings_repository.go"
SOCIAL_COMMENTS_POSTGRES = "apps/api/internal/adapters/gormstore/social_comment_repository_postgres_test.go"
ACTIVATION_LOCK = (
    '\t\tif err := tx.Exec("SELECT pg_advisory_xact_lock_shared(?)", '
    "policyPublisherAdvisoryLock).Error; err != nil { // hourpaths-direct-sql: allow "
    "PostgreSQL transaction advisory lock\n"
)
POLICY_LOCK = (
    '\t\tlocked := tx.Exec("SELECT pg_advisory_xact_lock(?)", '
    "policyPublisherAdvisoryLock) // hourpaths-direct-sql: allow PostgreSQL transaction "
    "advisory lock\n"
)
COMMENT_MIGRATION_ROLLBACK = (
    '\tif err := tx.Exec(downSQL).Error; err == nil || !strings.Contains(err.Error(), "cannot remove practice comments while comment data exists") { '
    "// hourpaths-direct-sql: allow rollback-only execution of embedded migration\n"
)
INTERACTION_SETTINGS_LOCK = (
    '\treturn tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", '
    'socialLockKey("social-interaction-owner", owner)).Error // hourpaths-direct-sql: allow '
    "deterministic PostgreSQL transaction advisory lock\n"
)


class DirectSQLGuardTests(unittest.TestCase):
    def run_guard(self, files: dict[str, str]) -> subprocess.CompletedProcess[str]:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for relative, content in files.items():
                target = root / relative
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text(content, encoding="utf-8")
            return subprocess.run(
                ["python3", str(GUARD), str(root)],
                check=False,
                capture_output=True,
                text=True,
            )

    def test_allows_only_the_two_reviewed_advisory_locks(self) -> None:
        result = self.run_guard(
            {ACTIVATION: ACTIVATION_LOCK, POLICY: POLICY_LOCK}
        )

        self.assertEqual(result.returncode, 0, result.stderr)

    def test_rejects_allow_marker_in_an_unreviewed_file(self) -> None:
        result = self.run_guard(
            {
                "apps/api/internal/adapters/gormstore/unsafe.go": (
                    '\ttx.Exec("DELETE FROM users") // hourpaths-direct-sql: allow '
                    "PostgreSQL transaction advisory lock\n"
                )
            }
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unsafe.go:1 uses unreviewed direct SQL execution", result.stderr)

    def test_rejects_changed_sql_in_a_reviewed_file(self) -> None:
        result = self.run_guard(
            {
                ACTIVATION: ACTIVATION_LOCK.replace(
                    "pg_advisory_xact_lock_shared", "pg_advisory_xact_lock"
                )
            }
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "activation_repository.go:1 uses unreviewed direct SQL execution",
            result.stderr,
        )

    def test_rejects_an_extra_statement_in_a_reviewed_file(self) -> None:
        result = self.run_guard(
            {ACTIVATION: ACTIVATION_LOCK + '\ttx.Raw("SELECT 1")\n'}
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "activation_repository.go:2 uses unreviewed direct SQL execution",
            result.stderr,
        )

    def test_allows_only_the_exact_reviewed_comment_migration_rollback(self) -> None:
        result = self.run_guard({SOCIAL_COMMENTS_POSTGRES: COMMENT_MIGRATION_ROLLBACK})
        self.assertEqual(result.returncode, 0, result.stderr)

        changed = COMMENT_MIGRATION_ROLLBACK.replace("downSQL", '"DROP TABLE users"')
        result = self.run_guard({SOCIAL_COMMENTS_POSTGRES: changed})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("social_comment_repository_postgres_test.go:1 uses unreviewed direct SQL execution", result.stderr)

    def test_allows_only_the_exact_interaction_settings_owner_lock(self) -> None:
        result = self.run_guard({SOCIAL_INTERACTION_SETTINGS: INTERACTION_SETTINGS_LOCK})
        self.assertEqual(result.returncode, 0, result.stderr)

        changed = INTERACTION_SETTINGS_LOCK.replace("social-interaction-owner", "social-owner")
        result = self.run_guard({SOCIAL_INTERACTION_SETTINGS: changed})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("social_interaction_settings_repository.go:1 uses unreviewed direct SQL execution", result.stderr)

    def test_allows_multiple_exact_reviewed_calls_in_one_file(self) -> None:
        result = self.run_guard(
            {
                "apps/api/internal/adapters/gormstore/path/notification_persistence.go": (
                    "\tif err := tx.Exec(`\nSELECT 1`) {}\n"
                    "\treturn tx.Exec(`\nSELECT 2`).Error\n"
                )
            }
        )

        self.assertEqual(result.returncode, 0, result.stderr)

    def test_rejects_a_third_call_in_a_multi_call_reviewed_file(self) -> None:
        result = self.run_guard(
            {
                "apps/api/internal/adapters/gormstore/path/notification_persistence.go": (
                    "\tif err := tx.Exec(`\nSELECT 1`) {}\n"
                    "\treturn tx.Exec(`\nSELECT 2`).Error\n"
                    '\ttx.Exec("DELETE FROM users")\n'
                )
            }
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "notification_persistence.go:5 uses unreviewed direct SQL execution",
            result.stderr,
        )

    def test_rejects_a_multiline_direct_sql_call(self) -> None:
        result = self.run_guard(
            {
                "apps/api/internal/adapters/gormstore/unsafe.go": (
                    "\ttx.Exec\n"
                    '\t\t("DELETE FROM users") // hourpaths-direct-sql: allow anything\n'
                )
            }
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unsafe.go:1 uses unreviewed direct SQL execution", result.stderr)

    def test_allows_only_the_four_reviewed_social_relationship_calls(self) -> None:
        reviewed = (
            '\treturn tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock\n'
            '\tif err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error; err != nil { // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock\n'
            "\tif err := tx.Exec(`\nINSERT INTO notification_push_delivery_models`) {}\n"
            "\treturn tx.Exec(`\nINSERT INTO notification_push_outbox_models`).Error\n"
        )
        result = self.run_guard({SOCIAL_RELATIONSHIPS: reviewed})
        self.assertEqual(result.returncode, 0, result.stderr)

        result = self.run_guard({SOCIAL_RELATIONSHIPS: reviewed + '\ttx.Exec("DELETE FROM follow_models")\n'})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("social_relationship_repository.go:7 uses unreviewed direct SQL execution", result.stderr)

    def test_allows_only_the_reviewed_social_feed_query(self) -> None:
        reviewed = "\tresult := repository.db.WithContext(ctx).Raw(`\nSELECT 1`).Scan(&rows)\n"
        result = self.run_guard({SOCIAL_FEED: reviewed})
        self.assertEqual(result.returncode, 0, result.stderr)

        result = self.run_guard({SOCIAL_FEED: reviewed + '\trepository.db.Raw("SELECT 2")\n'})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("social_feed_repository.go:3 uses unreviewed direct SQL execution", result.stderr)


if __name__ == "__main__":
    unittest.main()
