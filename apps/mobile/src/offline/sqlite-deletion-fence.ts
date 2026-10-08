export function visibleAccountSQL(): string {
  return 'owner NOT IN (SELECT owner FROM account_deletion_fences_v1)';
}

export function deletionFenceSchema(table: 'tracking_accounts_v1' | 'tracking_home_v1' | 'tracking_appearances_v1'): string {
  return `CREATE TABLE IF NOT EXISTS account_deletion_fences_v1 (owner TEXT PRIMARY KEY NOT NULL);
    CREATE TRIGGER IF NOT EXISTS ${table}_deletion_insert BEFORE INSERT ON ${table}
    WHEN EXISTS (SELECT 1 FROM account_deletion_fences_v1 WHERE owner = NEW.owner)
    BEGIN SELECT RAISE(ABORT, 'account_deletion_pending'); END;
    CREATE TRIGGER IF NOT EXISTS ${table}_deletion_update BEFORE UPDATE ON ${table}
    WHEN EXISTS (SELECT 1 FROM account_deletion_fences_v1 WHERE owner = NEW.owner)
    BEGIN SELECT RAISE(ABORT, 'account_deletion_pending'); END;`;
}
