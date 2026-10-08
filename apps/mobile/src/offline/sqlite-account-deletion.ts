import { validDeletionIntent, type DeletionIntent } from '@hourpaths/client-core';
import type { TrackingSQLDatabase } from './sqlite-tracking-store';

export class SQLiteAccountDeletion {
  private readonly ready: Promise<void>;
  constructor(private readonly database: TrackingSQLDatabase) {
    this.ready = database.execAsync(`CREATE TABLE IF NOT EXISTS account_deletion_fences_v1 (owner TEXT PRIMARY KEY NOT NULL);
      CREATE TABLE IF NOT EXISTS account_deletion_surfaces_v1 (owner TEXT PRIMARY KEY NOT NULL, payload TEXT NOT NULL);
      CREATE TABLE IF NOT EXISTS account_deletion_intents_v1 (owner TEXT PRIMARY KEY NOT NULL, payload TEXT NOT NULL);`);
  }
  async isFenced(owner: string): Promise<boolean> {
    await this.ready;
    return !!await this.database.getFirstAsync<{ owner: string }>('SELECT owner FROM account_deletion_fences_v1 WHERE owner = ?', owner);
  }
  async read(owner: string): Promise<DeletionIntent | null> {
    await this.ready;
    const row = await this.database.getFirstAsync<{ payload: string }>('SELECT payload FROM account_deletion_intents_v1 WHERE owner = ?', owner);
    if (!row) return null;
    const value: unknown = JSON.parse(row.payload);
    if (!validDeletionIntent(value, owner)) throw new Error('deletion_intent_invalid');
    return value;
  }
  async save(intent: DeletionIntent): Promise<DeletionIntent> {
    if (!validDeletionIntent(intent, intent.owner)) throw new Error('deletion_intent_invalid');
    await this.ready;
    let saved = intent;
    await this.database.withExclusiveTransactionAsync(async transaction => {
      const row = await transaction.getFirstAsync<{ payload: string }>('SELECT payload FROM account_deletion_intents_v1 WHERE owner = ?', intent.owner);
      if (row) {
        const existing: unknown = JSON.parse(row.payload);
        if (!validDeletionIntent(existing, intent.owner)) throw new Error('deletion_intent_invalid');
        if (existing.receiptSecret !== intent.receiptSecret || existing.phase === 'confirmed') saved = existing;
      }
      await transaction.runAsync('INSERT OR IGNORE INTO account_deletion_fences_v1(owner) VALUES (?)', intent.owner);
      await transaction.runAsync(`INSERT INTO account_deletion_intents_v1(owner,payload) VALUES (?,?)
        ON CONFLICT(owner) DO UPDATE SET payload=excluded.payload`, intent.owner, JSON.stringify(saved));
    });
    return saved;
  }
  async fence(owner: string): Promise<void> {
    if (!owner || owner.trim() !== owner) throw new Error('tracking_owner_required');
    await this.ready;
    await this.database.runAsync('INSERT OR IGNORE INTO account_deletion_fences_v1(owner) VALUES (?)', owner);
  }
  async purge(owner: string): Promise<void> {
    await this.fence(owner);
    await this.database.withExclusiveTransactionAsync(async transaction => {
      for (const table of ['tracking_accounts_v1', 'tracking_home_v1', 'tracking_appearances_v1'] as const) {
        const exists = await transaction.getFirstAsync<{ name: string }>('SELECT name FROM sqlite_master WHERE type = ? AND name = ?', 'table', table);
        if (exists) await transaction.runAsync(`DELETE FROM ${table} WHERE owner = ?`, owner);
      }
    });
  }
  async forget(owner: string): Promise<void> {
    await this.ready;
    await this.database.withExclusiveTransactionAsync(async transaction => {
      await transaction.runAsync('DELETE FROM account_deletion_intents_v1 WHERE owner = ?', owner);
      await transaction.runAsync('DELETE FROM account_deletion_surfaces_v1 WHERE owner = ?', owner);
    });
  }
  async readSurfaces(owner: string): Promise<string[] | null> {
    await this.ready;
    const row = await this.database.getFirstAsync<{ payload: string }>('SELECT payload FROM account_deletion_surfaces_v1 WHERE owner = ?', owner);
    if (!row) return null;
    const value: unknown = JSON.parse(row.payload);
    if (!Array.isArray(value) || value.some(id => typeof id !== 'string')) throw new Error('deletion_surfaces_invalid');
    return value;
  }
  async saveSurfaces(owner: string, identifiers: string[]): Promise<void> {
    await this.ready;
    if (!owner || identifiers.some(id => typeof id !== 'string')) throw new Error('deletion_surfaces_invalid');
    await this.database.withExclusiveTransactionAsync(async transaction => {
      const row = await transaction.getFirstAsync<{ payload: string }>('SELECT payload FROM account_deletion_surfaces_v1 WHERE owner = ?', owner);
      const previous: unknown = row ? JSON.parse(row.payload) : [];
      if (!Array.isArray(previous) || previous.some(id => typeof id !== 'string')) throw new Error('deletion_surfaces_invalid');
      await transaction.runAsync(`INSERT INTO account_deletion_surfaces_v1(owner,payload) VALUES (?,?)
        ON CONFLICT(owner) DO UPDATE SET payload=excluded.payload`, owner, JSON.stringify([...new Set([...previous, ...identifiers])]));
    });
  }
  async pendingOwners(): Promise<string[]> {
    await this.ready;
    const row = await this.database.getFirstAsync<{ owners: string }>('SELECT json_group_array(owner) AS owners FROM account_deletion_intents_v1');
    const owners: unknown = JSON.parse(row?.owners ?? '[]');
    if (!Array.isArray(owners) || owners.some(owner => typeof owner !== 'string' || !owner || owner.trim() !== owner)) throw new Error('deletion_intent_invalid');
    return owners;
  }
}
