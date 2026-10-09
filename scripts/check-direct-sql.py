#!/usr/bin/env python3
"""Reject direct GORM SQL except exact, reviewed call sites."""

from __future__ import annotations

import re
import sys
from collections import Counter
from pathlib import Path


DIRECT_SQL = re.compile(r"\.(?:Raw|Exec)\s*\(")
REVIEWED_LINES: dict[str, tuple[str, ...]] = {
    # Reviewed deletion receipt locking, offline-restore isolation, and controlled retention fixtures.
    'apps/api/cmd/account-deletion-records/main.go': (
        '\tif err = db.WithContext(ctx).Raw("SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND backend_type=\'client backend\'").Scan(&clients).Error; err != nil {',
    ),
    'apps/api/internal/adapters/gormstore/account_deletion.go': (
        '\t\tif err := tx.Raw("SELECT public.lock_account_deletion_receipt(?, ?, ?)", userID, hash, now).Scan(&locked).Error; err != nil {',
        '\terr = s.DB.WithContext(ctx).Raw("SELECT public.prepare_deletion_journal_retirement(?, ?, ?)", record.UserID, hash, record.DeletedAt).Scan(&ready).Error',
    ),
    'apps/api/internal/adapters/gormstore/account_deletion_postgres_test.go': (
        '\t\t\t\tif err := seed.DB.Exec("INSERT INTO authorization_outbox_models(id,resource_type,resource_id,relation,subject_type,subject_id,owner_user_id,actor_user_id,operation,created_at) VALUES (?,\'resource\',?,\'owner\',\'user\',?,?,?,\'delete\',?)", newTestID(), newTestID(), owner, owner, owner, now).Error; err != nil {',
        '\tif err := runtime.DB.Exec("UPDATE account_deletion_models SET journal_retired_at=clock_timestamp()").Error; err == nil {',
    ),
    'apps/api/internal/adapters/auditretention/account_deletion_postgres_test.go': (
        '\t\tif err := admin.Exec("INSERT INTO account_deletion_models(user_id,deleted_at,audit_event_id,receipt_hash) VALUES (?,?,?,?)", owner, now.Add(-age), uuid.NewString(), make([]byte, 32)).Error; err != nil {',
        '\t\tif err := admin.Exec("INSERT INTO audit_event_models(id,owner_user_id,actor_user_id,action,target_type,target_id,outcome,correlation_id,occurred_at) VALUES (?,?,?,?,?,?,?,?,?)", ids[i], owner, owner, "resource.viewed", "account", owner, "succeeded", uuid.NewString(), now).Error; err != nil {',
        '\tif err := retention.Exec("DELETE FROM audit_event_models WHERE id = ?", ids[2]).Error; err == nil {',
        '\tif err := retention.Exec("UPDATE account_deletion_retention_run_models SET deleted_count = 99").Error; err == nil {',
        '\tif err = admin.Exec("INSERT INTO account_deletion_models(user_id,deleted_at,audit_event_id,receipt_hash) VALUES (?,?,?,?)", expired, now.Add(-29*24*time.Hour-time.Hour), uuid.NewString(), make([]byte, 32)).Error; err != nil {',
        '\t\tif err = admin.Exec("INSERT INTO authorization_outbox_models(id,resource_type,resource_id,relation,subject_type,subject_id,owner_user_id,actor_user_id,operation,completed_at,created_at) VALUES (?,\'resource\',?,\'owner\',\'user\',?,?,?,\'delete\',?,?)", ids[i], uuid.NewString(), owner, owner, owner, completed, now).Error; err != nil {',
        '\tif err = retention.Exec("DELETE FROM authorization_outbox_models WHERE id = ?", ids[1]).Error; err == nil {',
        '\t\tif err := admin.Exec("INSERT INTO account_deletion_models(user_id,deleted_at,audit_event_id,receipt_hash,journal_retired_at) VALUES (?,?,?,?,?)", id, now.Add(-30*24*time.Hour), uuid.NewString(), make([]byte, 32), retired).Error; err != nil {',
        '\t\t\tif err := admin.Exec("INSERT INTO authorization_outbox_models(id,resource_type,resource_id,relation,subject_type,subject_id,owner_user_id,actor_user_id,operation,created_at) VALUES (?,\'resource\',?,\'owner\',\'user\',?,?,?,\'delete\',?)", uuid.NewString(), uuid.NewString(), id, id, id, now).Error; err != nil {',
        '\tif err := retention.Exec("DELETE FROM account_deletion_models WHERE user_id=?", ids[1]).Error; err == nil {',
    ),

    # Controlled PostgreSQL tests: runtime-role enforcement and transactional migration setup.
    'apps/api/internal/adapters/gormstore/activity/offline_activity_postgres_test.go': (
        '\tif err := db.Raw("SELECT to_regclass(\'public.activity_edit_order_models\') IS NOT NULL").Scan(&present).Error; err != nil {',
        '\t\tif err := db.Exec(statement).Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
    ),
    'apps/api/internal/adapters/gormstore/activity/offline_postgres_test.go': (
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\tif err := db.Raw("SELECT to_regclass(\'public.offline_timer_state_models\') IS NOT NULL").Scan(&present).Error; err != nil {',
        '\t\tif err := db.Exec(statement).Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\tif err := db.Exec("RESET ROLE").Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\t\t\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\t\t\tif err := db.Exec("RESET ROLE").Error; err != nil {',
        '\t\t\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\t\t\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
        '\t\t\tif err := db.Exec("SET LOCAL ROLE app").Error; err != nil {',
    ),
    "apps/api/internal/adapters/gormstore/activation_repository.go": (
        '\t\tif err := tx.Exec("SELECT pg_advisory_xact_lock_shared(?)", '
        "policyPublisherAdvisoryLock).Error; err != nil { // hourpaths-direct-sql: allow "
        "PostgreSQL transaction advisory lock",
    ),
    "apps/api/internal/adapters/gormstore/progresslock/progress_lock.go": (
        '\treturn tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock',
    ),
    "apps/api/internal/adapters/gormstore/activity/membership_mutation_guard_postgres_test.go": (
        "\tbackfilled := migrationDB.Exec(`UPDATE public.path_membership_models AS membership SET joined_at = evidence.accepted_at FROM (SELECT invitation.path_id, invitation.recipient_user_id AS user_id, max(invitation.accepted_at) AS accepted_at FROM public.path_invitation_models AS invitation WHERE invitation.accepted_at IS NOT NULL GROUP BY invitation.path_id, invitation.recipient_user_id) AS evidence WHERE membership.path_id = evidence.path_id AND membership.user_id = evidence.user_id`)",
        "\tbackfilled := migrationDB.Exec(`UPDATE public.path_membership_models AS membership SET joined_at = COALESCE((SELECT max(invitation.accepted_at) FROM public.path_invitation_models AS invitation WHERE invitation.path_id = membership.path_id AND invitation.recipient_user_id = membership.user_id AND invitation.accepted_at IS NOT NULL), CASE WHEN membership.user_id = COALESCE((SELECT transfer.initiator_user_id FROM public.path_ownership_transfer_models AS transfer WHERE transfer.path_id = membership.path_id AND transfer.accepted_at IS NOT NULL ORDER BY transfer.accepted_at, transfer.created_at, transfer.id LIMIT 1), path.owner_user_id) THEN path.created_at END, CURRENT_TIMESTAMP) FROM public.path_models AS path WHERE path.id = membership.path_id AND path.id = ?`, pathID)",
    ),
    "apps/api/internal/adapters/gormstore/policy_renewal.go": (
        '\t\tif err = tx.Exec("SELECT pg_advisory_xact_lock(?)", policyPublisherAdvisoryLock).Error; err != nil { // hourpaths-direct-sql: allow PostgreSQL transaction advisory lock',
    ),
    "apps/api/internal/adapters/gormstore/policy_authority.go": (
        '\t\tlocked := tx.Exec("SELECT pg_advisory_xact_lock(?)", '
        "policyPublisherAdvisoryLock) // hourpaths-direct-sql: allow PostgreSQL transaction "
        "advisory lock",
    ),
    "apps/api/internal/adapters/gormstore/path/notification_persistence.go": (
        "\tif err := tx.Exec(`",
        "\treturn tx.Exec(`",
    ),
    "apps/api/internal/adapters/gormstore/push_repository.go": (
        "\t\tif err := tx.Raw(`",
        "\t\tif err := tx.Exec(`",
        "\treturn tx.Exec(`",
        "\treturn tx.Exec(`",
    ),
    "apps/api/internal/adapters/gormstore/social_relationship_repository.go": (
        '\treturn tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock',
        '\tif err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error; err != nil { // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock',
        "\tif err := tx.Exec(`",
        "\treturn tx.Exec(`",
    ),
    "apps/api/internal/adapters/gormstore/social_feed_repository.go": (
        "\tresult := repository.db.WithContext(ctx).Raw(`",
    ),
    "apps/api/internal/adapters/gormstore/social_interaction_settings_repository.go": (
        '\treturn tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", socialLockKey("social-interaction-owner", owner)).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock',
    ),
    "apps/api/internal/adapters/gormstore/social_reaction_repository.go": (
        '\treturn tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock',
        "\tif err := tx.Exec(`INSERT INTO notification_push_delivery_models (",
        "\treturn tx.Exec(`UPDATE notification_push_outbox_models",
    ),
    "apps/api/internal/adapters/gormstore/social_comment_repository.go": (
        '\treturn tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock',
        '\treturn tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock',
        "\tif err := tx.Exec(`INSERT INTO notification_push_delivery_models (",
        "\treturn tx.Exec(`UPDATE notification_push_outbox_models",
    ),
    "apps/api/internal/adapters/gormstore/social_comment_repository_postgres_test.go": (
        "\t\tif err := store.DB.Raw(`SELECT",
        '\tif err := tx.Exec(downSQL).Error; err == nil || !strings.Contains(err.Error(), "cannot remove practice comments while comment data exists") { // hourpaths-direct-sql: allow rollback-only execution of embedded migration',
    ),
    "apps/api/internal/adapters/gormstore/social_reaction_repository_postgres_test.go": (
        "\tif err := store.DB.Raw(`SELECT",
    ),
    "apps/api/internal/adapters/gormstore/push_repository_postgres_test.go": (
        "\tif err := tx.Exec(`INSERT INTO path_models",
        "\tif err := tx.Exec(`INSERT INTO path_invitation_models",
        "\tif err := tx.Exec(`INSERT INTO notification_models",
        "\tif err := tx.Exec(`INSERT INTO notification_models",
        "\tif err := tx.Exec(`INSERT INTO notification_push_delivery_models",
        "\tif err := tx.Exec(`INSERT INTO notification_push_delivery_models",
        "\tif err := tx.Exec(`UPDATE notification_push_delivery_models",
        "\tif err := tx.Exec(`INSERT INTO notification_push_outbox_models (notification_id, created_at) VALUES (?, ?)`,",
        "\tif err := tx.Raw(`SELECT suppressed_at, token_ciphertext",
        "\tif err := tx.Raw(`SELECT permanently_failed_at",
    ),
}


def violations(root: Path) -> list[str]:
    findings: list[str] = []
    apps = root / "apps"
    if not apps.is_dir():
        return findings

    for source in sorted(apps.rglob("*.go")):
        relative = source.relative_to(root).as_posix()
        remaining = Counter(REVIEWED_LINES.get(relative, ()))
        content = source.read_text(encoding="utf-8")
        lines = content.splitlines()
        for match in DIRECT_SQL.finditer(content):
            number = content.count("\n", 0, match.start()) + 1
            line = lines[number - 1]
            if remaining[line] < 1:
                findings.append(
                    f"{relative}:{number} uses unreviewed direct SQL execution"
                )
            else:
                remaining[line] -= 1
    return findings


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    findings = violations(root)
    for finding in findings:
        print(f"direct SQL guard: {finding}", file=sys.stderr)
    return 1 if findings else 0


if __name__ == "__main__":
    raise SystemExit(main())
